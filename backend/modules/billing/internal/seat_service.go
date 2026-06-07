package billing

import (
	"time"

	"manager-backend/framework/apperr"
)

type seatServiceImpl struct {
	repo seatRepository
	ent  *entitlementServiceImpl
}

var SeatService *seatServiceImpl

func newSeatService(repo seatRepository, ent *entitlementServiceImpl) *seatServiceImpl {
	return &seatServiceImpl{repo: repo, ent: ent}
}

// TryOccupyInstanceSeat 实例创建门禁：未冻结 且 占用<容量 → 占用+1。
func (s *seatServiceImpl) TryOccupyInstanceSeat(userID int) error {
	frozen, err := s.IsFrozen(userID)
	if err != nil {
		return err
	}
	if frozen {
		return apperr.Forbidden("账户已冻结，无法创建实例")
	}
	cap, err := s.ent.Capacity(userID, SubjectInstanceSeat)
	if err != nil {
		return err
	}
	ok, err := s.repo.occupy(userID, cap)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.Validation("实例席位不足，请购买后再创建")
	}
	return nil
}

func (s *seatServiceImpl) ReleaseInstanceSeat(userID int) error { return s.repo.release(userID) }

func (s *seatServiceImpl) ReconcileInstanceSeats(userID, count int) error {
	return s.repo.reconcile(userID, int64(count))
}

func (s *seatServiceImpl) InstanceSeatCapacity(userID int) (int64, error) {
	return s.ent.Capacity(userID, SubjectInstanceSeat)
}

func (s *seatServiceImpl) IsFrozen(userID int) (bool, error) {
	d, err := s.repo.getDunning(userID)
	if err != nil {
		return false, err
	}
	return d.State == DunningFrozen || d.State == DunningRecycled, nil
}

// ListDunningEnforcement frozen/recycled 用户 + 容量（phone 执行用）。
func (s *seatServiceImpl) ListDunningEnforcement() ([]EnforcementTarget, error) {
	ds, err := s.repo.listDunningByStates([]string{DunningFrozen, DunningRecycled})
	if err != nil {
		return nil, err
	}
	out := make([]EnforcementTarget, 0, len(ds))
	for _, d := range ds {
		cap, err := s.ent.Capacity(int(d.UserID), SubjectInstanceSeat)
		if err != nil {
			return nil, err
		}
		out = append(out, EnforcementTarget{UserID: int(d.UserID), State: d.State, Capacity: cap})
	}
	return out, nil
}

// SetDunningForTest 测试用：直接置某用户欠费状态（EnteredAt=now）。
func SetDunningForTest(userID int, state string) error {
	return SeatService.repo.upsertDunning(userID, state, time.Now())
}

// runDunning cron 一次：扫描所有占用，转换欠费状态（幂等）。X=graceDays, Y=frozenDays。
func (s *seatServiceImpl) runDunning(graceDays, frozenDays int) error {
	now := time.Now()
	usages, err := s.repo.listUsages()
	if err != nil {
		return err
	}
	for _, u := range usages {
		cap, err := s.ent.Capacity(int(u.UserID), SubjectInstanceSeat)
		if err != nil {
			return err
		}
		over := u.InstanceSeatsUsed > cap
		d, err := s.repo.getDunning(int(u.UserID))
		if err != nil {
			return err
		}
		if !over {
			if d.State != DunningActive {
				if err := s.repo.upsertDunning(int(u.UserID), DunningActive, now); err != nil {
					return err
				}
			}
			continue
		}
		// 超量：按时间推进状态
		switch d.State {
		case DunningActive:
			if err := s.repo.upsertDunning(int(u.UserID), DunningGrace, now); err != nil {
				return err
			}
		case DunningGrace:
			if now.Sub(d.EnteredAt) >= time.Duration(graceDays)*24*time.Hour {
				if err := s.repo.upsertDunning(int(u.UserID), DunningFrozen, now); err != nil {
					return err
				}
			}
		case DunningFrozen:
			if now.Sub(d.EnteredAt) >= time.Duration(frozenDays)*24*time.Hour {
				if err := s.repo.upsertDunning(int(u.UserID), DunningRecycled, now); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
