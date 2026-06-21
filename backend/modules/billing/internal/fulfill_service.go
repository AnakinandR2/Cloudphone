package billing

import (
	"time"

	"manager-backend/framework/apperr"
)

// 统一履约（fulfillment）：订单购买、试用、admin 赠送三条入口都收口到这里。
//   - seat / boot_slot 新发：创建 N 个 LicenseUnit（到期 = now + duration，单位按 kind）
//   - seat / boot_slot 续费：对选中 unit_ids 延长到期
//   - runtime_pack 新发：加临时时长余量

// fulfillServiceImpl 履约服务。
type fulfillServiceImpl struct {
	license  licenseRepository
	rtWallet runtimeWalletRepository
	wallet   repository
}

// FulfillService 模块内单例。
var FulfillService *fulfillServiceImpl

func newFulfillService(license licenseRepository, rtWallet runtimeWalletRepository, wallet repository) *fulfillServiceImpl {
	return &fulfillServiceImpl{license: license, rtWallet: rtWallet, wallet: wallet}
}

// expireFrom 计算到期：seat 用 AddDate 加月，boot_slot 加天。
func expireFrom(base time.Time, kind string, value int) time.Time {
	switch kind {
	case KindSeat:
		return base.AddDate(0, value, 0)
	case KindBootSlot:
		return base.AddDate(0, 0, value)
	}
	return base
}

// FulfillNew 新发 N 个授权单元（seat/boot_slot）。durationValue 单位按 kind。
func (s *fulfillServiceImpl) FulfillNew(userID int, kind string, quantity, durationValue int, source, ref string) error {
	if kind != KindSeat && kind != KindBootSlot {
		return apperr.Validation("非法的授权单元类型")
	}
	if quantity < 1 {
		return apperr.Validation("数量必须≥1")
	}
	if durationValue < 1 {
		return apperr.Validation("时长必须≥1")
	}
	now := time.Now()
	expire := expireFrom(now, kind, durationValue)
	units := make([]LicenseUnit, 0, quantity)
	for i := 0; i < quantity; i++ {
		units = append(units, LicenseUnit{
			UserID: uint(userID), Kind: kind, Status: LicenseActive,
			Source: source, SourceRef: ref, ExpireAt: expire,
		})
	}
	if err := s.license.create(units); err != nil {
		return err
	}
	// 落资源流水（subject=kind）。
	cap, _ := s.activeCount(userID, kind, now)
	ledgerType := LedgerPurchase
	switch source {
	case SourceTrial:
		ledgerType = LedgerTrial
	case SourceGrant, SourceAdjust:
		ledgerType = LedgerAdjustGrant
	}
	_ = s.writeResourceLedger(userID, kind, int64(quantity), cap, ledgerType, ref, sourceOperator(source))
	return nil
}

// FulfillRenew 续费：对选中单元延长到期（按 kind 单位叠加 durationValue）。
func (s *fulfillServiceImpl) FulfillRenew(userID int, kind string, unitIDs []uint, durationValue int) error {
	if len(unitIDs) == 0 {
		return apperr.Validation("请选择要续费的授权单元")
	}
	if durationValue < 1 {
		return apperr.Validation("时长必须≥1")
	}
	now := time.Now()
	// 校验单元归属。
	owned, err := s.license.getByIDs(userID, unitIDs, kind)
	if err != nil {
		return err
	}
	if len(owned) != len(unitIDs) {
		return apperr.Validation("存在不属于当前用户的授权单元")
	}
	err = s.license.extendExpiry(unitIDs, func(cur time.Time) time.Time {
		base := cur
		if base.Before(now) {
			base = now // 已过期单元从现在续起
		}
		return expireFrom(base, kind, durationValue)
	})
	if err != nil {
		return err
	}
	cap, _ := s.activeCount(userID, kind, now)
	_ = s.writeResourceLedger(userID, kind, int64(len(unitIDs)), cap, LedgerRenew, "renew", "user:")
	return nil
}

// FulfillRuntimePack 加临时开机时长余量。
func (s *fulfillServiceImpl) FulfillRuntimePack(userID int, minutes int, source, ref string) error {
	if minutes < 1 {
		return apperr.Validation("时长必须≥1")
	}
	after, err := s.rtWallet.addMinutes(userID, int64(minutes))
	if err != nil {
		return err
	}
	ledgerType := LedgerPurchase
	switch source {
	case SourceTrial:
		ledgerType = LedgerTrial
	case SourceGrant, SourceAdjust:
		ledgerType = LedgerAdjustGrant
	}
	_ = s.writeResourceLedger(userID, SubjectRuntimeMinute, int64(minutes), after, ledgerType, ref, sourceOperator(source))
	return nil
}

// GiftRuntime 赠送临时开机时长（席位购买/续费触发），落 gift 科目流水。返回赠送后余量。
func (s *fulfillServiceImpl) GiftRuntime(userID, minutes int, ref string) (int64, error) {
	if minutes < 1 {
		return 0, apperr.Validation("赠送时长必须≥1")
	}
	after, err := s.rtWallet.addMinutes(userID, int64(minutes))
	if err != nil {
		return 0, err
	}
	_ = s.writeResourceLedger(userID, SubjectRuntimeMinute, int64(minutes), after, LedgerGift, ref, "system:gift")
	return after, nil
}

func (s *fulfillServiceImpl) activeCount(userID int, kind string, now time.Time) (int64, error) {
	units, err := s.license.activeUnits(userID, kind, now)
	if err != nil {
		return 0, err
	}
	return int64(len(units)), nil
}

// writeResourceLedger 写一条资源科目流水（复用 LedgerEntry，subject=kind/runtime_minute）。
func (s *fulfillServiceImpl) writeResourceLedger(userID int, subject string, delta, after int64, ledgerType, reason, operator string) error {
	return s.wallet.insertLedgerEntry(&LedgerEntry{
		UserID: uint(userID), Subject: subject, Type: ledgerType,
		Delta: delta, BalanceAfter: after, Reason: reason, Operator: operator,
	})
}

func sourceOperator(source string) string {
	switch source {
	case SourceTrial:
		return "system:trial"
	case SourceGrant, SourceAdjust:
		return "staff"
	}
	return "system"
}
