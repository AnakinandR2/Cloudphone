package billing

import (
	"time"

	"manager-backend/framework/apperr"
)

type entitlementServiceImpl struct{ repo entitlementRepository }

// EntitlementService 模块内实例，由 module.Init 注入 DB 后装配。
var EntitlementService *entitlementServiceImpl

func newEntitlementService(repo entitlementRepository) *entitlementServiceImpl {
	return &entitlementServiceImpl{repo: repo}
}

// Grant 发放一批资源额度 + 写流水。expireAt=nil 表示永久。
// ledgerType ∈ {trial, adjust_grant, ...}（货币侧的 purchase 由订单计划另行处理）。ref=订单号/理由。
func (s *entitlementServiceImpl) Grant(userID int, subject string, quantity int64, expireAt *time.Time, source, ref, ledgerType, operator string) (*EntitlementBatch, error) {
	if !resourceSubjects[subject] {
		return nil, apperr.Validation("非法的资源科目")
	}
	if quantity <= 0 {
		return nil, apperr.Validation("发放数量必须大于0")
	}
	var batch EntitlementBatch
	err := s.repo.txWith(func(tx entitlementRepository) error {
		batch = EntitlementBatch{UserID: uint(userID), Subject: subject, Quantity: quantity, Source: source, SourceRef: ref, ExpireAt: expireAt}
		if err := tx.createBatch(&batch); err != nil {
			return err
		}
		capacity, err := tx.capacity(userID, subject, time.Now())
		if err != nil {
			return err
		}
		return tx.insertLedger(&LedgerEntry{
			UserID: uint(userID), Subject: subject, Type: ledgerType,
			Delta: quantity, BalanceAfter: capacity, Reason: ref, Operator: operator,
		})
	})
	if err != nil {
		return nil, err
	}
	return &batch, nil
}

// Deduct 扣减资源额度（临近到期优先 FIFO）+ 写流水。容量不足报错并回滚。
// ledgerType ∈ {consume, adjust_deduct}。
func (s *entitlementServiceImpl) Deduct(userID int, subject string, amount int64, reason, ledgerType, operator string) error {
	if !resourceSubjects[subject] {
		return apperr.Validation("非法的资源科目")
	}
	if amount <= 0 {
		return apperr.Validation("扣减数量必须大于0")
	}
	return s.repo.txWith(func(tx entitlementRepository) error {
		now := time.Now()
		// 并发硬化（Phase 2 前置）：sqlite 串行化下安全；迁 MySQL/PG 后，
		// 自动按分钟消耗上线前，这里需对批次行加 SELECT ... FOR UPDATE 防丢失更新。
		batches, err := tx.listActiveBatches(userID, subject, now)
		if err != nil {
			return err
		}
		remaining := amount
		for _, b := range batches {
			if remaining <= 0 {
				break
			}
			avail := b.Quantity - b.Used
			if avail <= 0 {
				continue
			}
			take := avail
			if take > remaining {
				take = remaining
			}
			if err := tx.addUsed(int(b.ID), take); err != nil {
				return err
			}
			remaining -= take
		}
		if remaining > 0 {
			return apperr.Validation("额度不足")
		}
		capacity, err := tx.capacity(userID, subject, now)
		if err != nil {
			return err
		}
		return tx.insertLedger(&LedgerEntry{
			UserID: uint(userID), Subject: subject, Type: ledgerType,
			Delta: -amount, BalanceAfter: capacity, Reason: reason, Operator: operator,
		})
	})
}

// AdjustResource 运营手动赠送(delta>0)/扣减(delta<0)资源，理由必填。退款=赠送对应资源。
func (s *entitlementServiceImpl) AdjustResource(userID int, subject string, delta int64, reason, operator string) error {
	if delta == 0 {
		return apperr.Validation("调整数量不能为0")
	}
	if reason == "" {
		return apperr.Validation("调整理由必填")
	}
	if delta > 0 {
		_, err := s.Grant(userID, subject, delta, nil, SourceAdjust, reason, LedgerAdjustGrant, operator)
		return err
	}
	return s.Deduct(userID, subject, -delta, reason, LedgerAdjustDeduct, operator)
}

// Capacity 某科目可用容量。
func (s *entitlementServiceImpl) Capacity(userID int, subject string) (int64, error) {
	if !resourceSubjects[subject] {
		return 0, apperr.Validation("非法的资源科目")
	}
	return s.repo.capacity(userID, subject, time.Now())
}

// Capacities 三类资源容量概览。
func (s *entitlementServiceImpl) Capacities(userID int) (*CapacitySnapshot, error) {
	now := time.Now()
	inst, err := s.repo.capacity(userID, SubjectInstanceSeat, now)
	if err != nil {
		return nil, err
	}
	boot, err := s.repo.capacity(userID, SubjectBootSeat, now)
	if err != nil {
		return nil, err
	}
	mins, err := s.repo.capacity(userID, SubjectRuntimeMinute, now)
	if err != nil {
		return nil, err
	}
	return &CapacitySnapshot{InstanceSeat: inst, BootSeat: boot, RuntimeMinute: mins}, nil
}

// ListBatches 列某用户某科目（空=全部）的批次。
func (s *entitlementServiceImpl) ListBatches(userID int, subject string) ([]EntitlementBatch, error) {
	if subject != "" && !resourceSubjects[subject] {
		return nil, apperr.Validation("非法的资源科目")
	}
	return s.repo.listBatches(userID, subject)
}
