package billing

import "time"

// RuntimeService 时长费计量与结算服务（模块内单例）。
var RuntimeService *runtimeServiceImpl

// runtimeServiceImpl 时长费计量与结算（Metering 子域）。
type runtimeServiceImpl struct {
	repo   runtimeRepository
	ent    *entitlementServiceImpl
	wallet repository // 复用钱包 applyBalance + 账户读取
}

func newRuntimeService(repo runtimeRepository, ent *entitlementServiceImpl, wallet repository) *runtimeServiceImpl {
	return &runtimeServiceImpl{repo: repo, ent: ent, wallet: wallet}
}

// meterMinutes 逐分钟时间线：对窗口 [start,end) 每个整分钟，统计并发运行台 R 与开机席位 S，
// 累计应计台·分钟 = Σ max(0, R−S)、被席位覆盖台·分钟 = Σ min(R,S)。纯函数，便于单测。
// 约定 start/end 均按分钟对齐；End=nil 的区间视为运行中（覆盖窗口内任意分钟）。
func meterMinutes(intervals []RuntimeInterval, seatCap int64, start, end time.Time) (billable, covered int64) {
	if !end.After(start) {
		return 0, 0
	}
	for m := start; m.Before(end); m = m.Add(time.Minute) {
		var r int64
		for _, iv := range intervals {
			ivEnd := end
			if iv.End != nil {
				ivEnd = *iv.End
			}
			// 区间覆盖分钟 m ⟺ iv.Start ≤ m < ivEnd。
			if !iv.Start.After(m) && m.Before(ivEnd) {
				r++
			}
		}
		if r <= 0 {
			continue
		}
		if r > seatCap {
			billable += r - seatCap
			covered += seatCap
		} else {
			covered += r
		}
	}
	return billable, covered
}

// SettleRuntime 结算某用户在 [水位, windowEnd) 内已发生的运行分钟，按覆盖优先级扣费（幂等）。
// 首次见到该用户只落水位、不回溯历史。windowEnd 由调用方传 now；内部按分钟对齐。
func (s *runtimeServiceImpl) SettleRuntime(userID int, windowEnd time.Time, intervals []RuntimeInterval) (SettleResult, error) {
	end := windowEnd.Truncate(time.Minute)

	wm, err := s.repo.getWatermark(userID)
	if err != nil {
		return SettleResult{}, err
	}
	if wm == nil {
		// 首次：从当前分钟起算，不对历史计费。
		return SettleResult{WindowStart: end, WindowEnd: end}, s.repo.setWatermark(userID, end)
	}
	start := wm.SettlededAt.Truncate(time.Minute)
	if !end.After(start) {
		return SettleResult{WindowStart: start, WindowEnd: end}, nil
	}

	seatCap, err := s.ent.Capacity(userID, SubjectBootSeat)
	if err != nil {
		return SettleResult{}, err
	}
	billable, covered := meterMinutes(intervals, seatCap, start, end)

	res := SettleResult{WindowStart: start, WindowEnd: end, BillableUnitMinutes: billable}
	cfg, err := s.repo.getConfig()
	if err != nil {
		return SettleResult{}, err
	}
	unit := cfg.UnitPriceCentsPerMinute

	if billable > 0 {
		// 1) 先扣时长包（FIFO 临到期优先）。
		availMin, err := s.ent.Capacity(userID, SubjectRuntimeMinute)
		if err != nil {
			return SettleResult{}, err
		}
		packMin := min64(billable, availMin)
		if packMin > 0 {
			if err := s.ent.Deduct(userID, SubjectRuntimeMinute, packMin, "时长费扣减", LedgerRuntimeMinute, "system"); err != nil {
				return SettleResult{}, err
			}
			res.ChargedPackMinutes = packMin
		}
		// 2) 余下按单价从钱包余额扣（只扣余额能覆盖的部分，不越扣为负）。
		remainMin := billable - packMin
		if remainMin > 0 && unit > 0 {
			acc, err := s.wallet.getAccountOrNil(userID)
			if err != nil {
				return SettleResult{}, err
			}
			bal := int64(0)
			if acc != nil {
				bal = acc.BalanceCents
			}
			coverMin := min64(remainMin, bal/unit)
			if coverMin > 0 {
				cents := coverMin * unit
				if _, err := s.wallet.applyBalance(userID, -cents, LedgerRuntimeBalance, "时长费扣减", 0, "system"); err != nil {
					return SettleResult{}, err
				}
				res.ChargedBalanceCents = cents
				remainMin -= coverMin
			}
		}
		res.UnfundedMinutes = remainMin

		_ = s.repo.insertSlice(&RuntimeUsageSlice{
			UserID: uint(userID), WindowStart: start, WindowEnd: end,
			BillableUnitMinutes: billable, CoveredSeatMinutes: covered,
			ChargedPackMinutes: res.ChargedPackMinutes, ChargedBalanceCents: res.ChargedBalanceCents,
			UnfundedMinutes: res.UnfundedMinutes, UnitPriceCents: unit,
		})
	}

	// 推进水位（即便本窗口无应计，也要前移，避免下次重算空窗口）。
	if err := s.repo.setWatermark(userID, end); err != nil {
		return SettleResult{}, err
	}
	return res, nil
}

// RuntimeCoverage 给护栏读：可用开机席位 / 剩余时长包 / 余额 / 单价。
func (s *runtimeServiceImpl) RuntimeCoverage(userID int) (RuntimeCoverage, error) {
	seats, err := s.ent.Capacity(userID, SubjectBootSeat)
	if err != nil {
		return RuntimeCoverage{}, err
	}
	mins, err := s.ent.Capacity(userID, SubjectRuntimeMinute)
	if err != nil {
		return RuntimeCoverage{}, err
	}
	acc, err := s.wallet.getAccountOrNil(userID)
	if err != nil {
		return RuntimeCoverage{}, err
	}
	bal := int64(0)
	if acc != nil {
		bal = acc.BalanceCents
	}
	cfg, err := s.repo.getConfig()
	if err != nil {
		return RuntimeCoverage{}, err
	}
	return RuntimeCoverage{AvailableBootSeats: seats, RemainingPackMinutes: mins, BalanceCents: bal, UnitPriceCents: cfg.UnitPriceCentsPerMinute}, nil
}

func (s *runtimeServiceImpl) GetConfig() (*BillingRuntimeConfig, error) { return s.repo.getConfig() }

func (s *runtimeServiceImpl) SaveConfig(unitPrice, lowBalance int64) (*BillingRuntimeConfig, error) {
	cfg := &BillingRuntimeConfig{ID: 1, UnitPriceCentsPerMinute: unitPrice, LowBalanceAlertCents: lowBalance}
	if err := s.repo.saveConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *runtimeServiceImpl) ListSlices(userID, page, size int) ([]RuntimeUsageSlice, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return s.repo.listSlices(userID, (page-1)*size, size)
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
