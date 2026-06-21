package billing

import (
	"time"
)

// runtimeEngineServiceImpl 新运行计费引擎服务：结算（满1分钟/包月优先/200封顶）、开机校验、费用日志聚合。
type runtimeEngineServiceImpl struct {
	charges  runtimeChargeRepository
	rtWallet runtimeWalletRepository
	license  *licenseServiceImpl
	pricing  *pricingConfigServiceImpl
}

// RuntimeEngineService 模块内单例。
var RuntimeEngineService *runtimeEngineServiceImpl

func newRuntimeEngineService(charges runtimeChargeRepository, rtWallet runtimeWalletRepository, license *licenseServiceImpl, pricing *pricingConfigServiceImpl) *runtimeEngineServiceImpl {
	return &runtimeEngineServiceImpl{charges: charges, rtWallet: rtWallet, license: license, pricing: pricing}
}

// Settle 结算某用户在 windowEnd 时刻的运行扣费（每分钟 tick，幂等）。
// intervals 为该用户当前/近期运行区间（含 CpID/RunSessionRef；End=nil 表示运行中）。
func (s *runtimeEngineServiceImpl) Settle(userID int, windowEnd time.Time, intervals []RuntimeInterval) (SettleResult, error) {
	end := windowEnd.Truncate(time.Minute)
	res := SettleResult{WindowStart: end, WindowEnd: end}

	bootSlots, err := s.license.Capacity(userID, KindBootSlot)
	if err != nil {
		return res, err
	}
	remaining, err := s.rtWallet.remaining(userID)
	if err != nil {
		return res, err
	}
	dailyCap, err := s.pricing.DailyCapMinutes()
	if err != nil {
		return res, err
	}

	// 计算每台本 tick 待结算的新整分钟数（满 1 分钟规则）。
	states := make([]runInstanceState, 0, len(intervals))
	type meta struct {
		ref        string
		instanceID string
		powerOn    time.Time
		// base 为开机时刻按分钟取整：所有 charge 窗口从 base+settledBefore 起按分钟平铺，
		// 保证同一会话各窗口首尾相接、永不重叠（不再用 end-newMinutes 反推，避免错位）。
		base          time.Time
		settledBefore int
		newMinutes    int
	}
	metas := map[string]meta{}
	for _, iv := range intervals {
		cp := iv.CpID
		ref := iv.RunSessionRef
		if ref == "" {
			ref = sessionRef(cp, iv.Start)
		}
		if cp == "" {
			cp = ref
		}
		// 同一 ref 在本次结算里只处理一次，避免重复区间被重复计费。
		if _, dup := metas[ref]; dup {
			continue
		}
		ivEnd := end
		if iv.End != nil && iv.End.Before(ivEnd) {
			ivEnd = *iv.End
		}
		totalMin := int(ivEnd.Sub(iv.Start) / time.Minute)
		if totalMin < 0 {
			totalMin = 0
		}
		settled, err := s.charges.sessionSettled(ref)
		if err != nil {
			return res, err
		}
		newMin := totalMin - settled
		if newMin <= 0 {
			continue
		}
		day := utc8Day(iv.Start)
		dailyBefore, err := s.rtWallet.dailyCharged(userID, cp, day)
		if err != nil {
			return res, err
		}
		states = append(states, runInstanceState{
			CpID: cp, RunSessionRef: ref, PowerOn: iv.Start,
			NewMinutes: newMin, DailyChargedBefore: dailyBefore,
		})
		metas[ref] = meta{
			ref: ref, instanceID: cp, powerOn: iv.Start,
			base: iv.Start.Truncate(time.Minute), settledBefore: settled, newMinutes: newMin,
		}
	}

	if len(states) == 0 {
		return res, nil
	}

	plans := planRuntimeCharges(states, bootSlots, remaining, dailyCap)
	res.WindowStart = end
	for _, p := range plans {
		m := metas[p.RunSessionRef]
		// 游标从「开机时刻 + 已结算分钟」起，按各段分钟数依次平铺，段间首尾相接。
		cursor := m.base.Add(time.Duration(m.settledBefore) * time.Minute)
		advance := func(mins int) (time.Time, time.Time) {
			ws := cursor
			cursor = cursor.Add(time.Duration(mins) * time.Minute)
			return ws, cursor
		}
		// 扣临时时长（落账 + 余量 + 当天累计）。
		if p.TempMin > 0 {
			ws, we := advance(p.TempMin)
			if _, err := s.rtWallet.consume(userID, int64(p.TempMin)); err != nil {
				return res, err
			}
			if err := s.rtWallet.addDaily(userID, m.instanceID, utc8Day(m.powerOn), p.TempMin); err != nil {
				return res, err
			}
			// 写余额流水（subject=runtime_minute，type=runtime_consume）。
			afterRem, _ := s.rtWallet.remaining(userID)
			_ = WalletService.insertResourceLedger(userID, SubjectRuntimeMinute, -int64(p.TempMin), afterRem, LedgerRuntimeConsume, "运行扣临时时长", "system")
			_ = s.charges.insertCharge(&RuntimeCharge{
				UserID: uint(userID), InstanceID: m.instanceID, RunSessionRef: p.RunSessionRef,
				WindowStart: ws, WindowEnd: we, QuotaType: QuotaTemp, ChargedMinutes: p.TempMin,
				Reason: tempReason(p.Rank, p.Capacity),
			})
			res.ChargedTempMinutes += int64(p.TempMin)
		}
		if p.BootSlotMin > 0 {
			ws, we := advance(p.BootSlotMin)
			_ = s.charges.insertCharge(&RuntimeCharge{
				UserID: uint(userID), InstanceID: m.instanceID, RunSessionRef: p.RunSessionRef,
				WindowStart: ws, WindowEnd: we, QuotaType: QuotaBootSlot, ChargedMinutes: 0,
				Reason: bootReason(p.Rank, p.Capacity),
			})
			res.BootSlotMinutes += int64(p.BootSlotMin)
		}
		if p.CappedFreeMin > 0 {
			ws, we := advance(p.CappedFreeMin)
			_ = s.charges.insertCharge(&RuntimeCharge{
				UserID: uint(userID), InstanceID: m.instanceID, RunSessionRef: p.RunSessionRef,
				WindowStart: ws, WindowEnd: we, QuotaType: QuotaCappedFree, ChargedMinutes: 0,
				Reason: cappedReason(dailyCap, utc8Day(m.powerOn)),
			})
			res.CappedFreeMinutes += int64(p.CappedFreeMin)
		}
		// 推进会话结算进度到绝对值（开机以来累计已结算分钟），保证幂等不重扣。
		if err := s.charges.bumpSessionSettled(userID, m.instanceID, p.RunSessionRef, m.settledBefore+m.newMinutes); err != nil {
			return res, err
		}
	}
	return res, nil
}

// CanBoot 开机前置校验：有空闲包月名额或临时时长>0 才允许开机。
func (s *runtimeEngineServiceImpl) CanBoot(userID int) (bool, error) {
	remaining, err := s.rtWallet.remaining(userID)
	if err != nil {
		return false, err
	}
	if remaining > 0 {
		return true, nil
	}
	bootSlots, err := s.license.Capacity(userID, KindBootSlot)
	if err != nil {
		return false, err
	}
	return bootSlots > 0, nil
}

// ReleaseInstanceOccupancy 实例被删除/回收时释放其在 seat 单元上的占用。
func (s *runtimeEngineServiceImpl) ReleaseInstanceOccupancy(userID int, cpIDs []string) error {
	return s.license.repo.clearInstanceFor(userID, KindSeat, cpIDs)
}
