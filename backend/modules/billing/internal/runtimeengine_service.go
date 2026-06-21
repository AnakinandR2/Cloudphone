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
		newMinutes int
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
		metas[ref] = meta{ref: ref, instanceID: cp, powerOn: iv.Start, newMinutes: newMin}
	}

	if len(states) == 0 {
		return res, nil
	}

	plans := planRuntimeCharges(states, bootSlots, remaining, dailyCap)
	res.WindowStart = end
	for _, p := range plans {
		m := metas[p.RunSessionRef]
		windowStart := end.Add(-time.Duration(m.newMinutes) * time.Minute)
		// 扣临时时长（落账 + 余量 + 当天累计）。
		if p.TempMin > 0 {
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
				WindowStart: windowStart, WindowEnd: end, QuotaType: QuotaTemp, ChargedMinutes: p.TempMin,
			})
			res.ChargedTempMinutes += int64(p.TempMin)
		}
		if p.BootSlotMin > 0 {
			_ = s.charges.insertCharge(&RuntimeCharge{
				UserID: uint(userID), InstanceID: m.instanceID, RunSessionRef: p.RunSessionRef,
				WindowStart: windowStart, WindowEnd: end, QuotaType: QuotaBootSlot, ChargedMinutes: 0,
			})
			res.BootSlotMinutes += int64(p.BootSlotMin)
		}
		if p.CappedFreeMin > 0 {
			_ = s.charges.insertCharge(&RuntimeCharge{
				UserID: uint(userID), InstanceID: m.instanceID, RunSessionRef: p.RunSessionRef,
				WindowStart: windowStart, WindowEnd: end, QuotaType: QuotaCappedFree, ChargedMinutes: 0,
			})
			res.CappedFreeMinutes += int64(p.CappedFreeMin)
		}
		// 推进会话结算进度（即便部分免费/名额，整体已结算分钟仍前移，保证幂等不重扣）。
		settled, _ := s.charges.sessionSettled(p.RunSessionRef)
		if err := s.charges.bumpSessionSettled(userID, m.instanceID, p.RunSessionRef, settled+m.newMinutes); err != nil {
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
