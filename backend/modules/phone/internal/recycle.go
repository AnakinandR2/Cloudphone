package phone

import (
	"context"
	"log"
	"math"
	"time"

	"golang.org/x/sync/errgroup"
	"manager-backend/framework/apperr"
	"manager-backend/modules/billing"
)

// recycleReasonOverflow 席位不足导致超额回收的原因文案。
const recycleReasonOverflow = "席位不足，超额实例已进回收站"

// reconcileSeats 席位池协调（Phase 3 §2）：
//  1. 收集某用户全部「非回收」实例（cpId + 创建时间）。
//  2. 调 billing.ReconcileSeats 物化席位占用 + 返回需回收的 cpId（最新溢出，含热迁移）。
//  3. 对返回的 recycle cpId：在运行则强制关机 → 置 RECYCLED + recycled_at=now → 释放席位占用。
//
// 幂等：无 cpId 的未开通实例不参与（billing 按 cpId 物化）；已回收实例不在 reconcile 集合内。
func (s *serviceImpl) reconcileSeats(ctx context.Context, userID int) error {
	phones, err := s.repo.listNonRecycledByUser(userID)
	if err != nil {
		return err
	}
	refs := make([]billing.InstanceRef, 0, len(phones))
	byCp := map[string]*CloudPhone{}
	for i := range phones {
		p := &phones[i]
		if p.CpID == "" {
			continue // 未开通：无 cpId，不纳入席位物化
		}
		byCp[p.CpID] = p
		refs = append(refs, billing.InstanceRef{CpID: p.CpID, CreatedAt: p.CreatedAt})
	}
	recycle, err := billing.ReconcileSeats(userID, refs)
	if err != nil {
		return err
	}
	if len(recycle) == 0 {
		return nil
	}
	now := time.Now()
	for _, cp := range recycle {
		p := byCp[cp]
		if p == nil {
			continue
		}
		// 运行中（或过渡态）先强制关机，避免回收后仍在中台运行。
		if s.ops != nil && p.CpID != "" && p.Status == StatusRunning {
			if err := s.ops.StartOrShutdown(ctx, p.CpID, "关机"); err != nil {
				log.Printf("[reconcile] 回收前关机失败 user=%d cp=%s: %v", userID, p.CpID, err)
			}
		}
		if err := s.repo.markRecycled(p.ID, recycleReasonOverflow, now); err != nil {
			log.Printf("[reconcile] 置回收态失败 user=%d cp=%s: %v", userID, p.CpID, err)
			continue
		}
	}
	// 释放被回收实例的席位占用（billing 侧物化已清，这里显式释放保证一致）。
	_ = billing.ReleaseInstanceOccupancy(userID, recycle)
	return nil
}

// checkSeatAvailable 创建前席位校验（新模型）：当前非回收实例数须 < 未过期 seat 容量。
func (s *serviceImpl) checkSeatAvailable(userID int) error {
	capacity, err := billing.SeatCapacity(userID)
	if err != nil {
		return err
	}
	current, err := s.repo.listNonRecycledByUser(userID)
	if err != nil {
		return err
	}
	if len(current) >= capacity {
		return apperr.Conflict("实例席位不足，请先购买实例席位")
	}
	return nil
}

// runReconcilePatrol 周期席位巡检：对所有有「非回收」实例的用户跑 reconcile，
// 兜底处理席位过期/热迁移/溢出回收（关键事件触发之外的兜底）。best-effort，单用户失败不影响其它。
func (s *serviceImpl) runReconcilePatrol(ctx context.Context) {
	if s.ops == nil {
		return // 本地降级无中台：无 cpId 实例无法物化席位，跳过
	}
	ids, err := s.repo.activeOwnerIDs()
	if err != nil {
		return
	}
	eg := &errgroup.Group{}
	for _, uid := range ids {
		uid := uid
		eg.Go(func() error {
			if err := s.reconcileSeats(ctx, uid); err != nil {
				log.Printf("[reconcile-patrol] user=%d 失败: %v", uid, err)
			}
			return nil // best-effort
		})
	}
	_ = eg.Wait()
}

// RecycleBinItem 回收站列表项：实例信息 + 回收时间 + 剩余清理天数。
type RecycleBinItem struct {
	ID            uint       `json:"id"`
	CpID          string     `json:"cp_id"`
	Name          string     `json:"name"`
	RecycledAt    *time.Time `json:"recycled_at"`
	RecycleReason string     `json:"recycle_reason"`
	DaysRemaining int        `json:"days_remaining"` // 距清理还剩天数（0 表示今日内将清理）
}

// RecycleBinList 列出当前用户回收站实例（按 recycled_at 倒序），含剩余清理天数。
func (s *serviceImpl) RecycleBinList(userID int) ([]RecycleBinItem, error) {
	phones, err := s.repo.listRecycledByUser(userID)
	if err != nil {
		return nil, err
	}
	retention, err := billing.RecycleRetentionDays()
	if err != nil || retention <= 0 {
		retention = 30
	}
	now := time.Now()
	out := make([]RecycleBinItem, 0, len(phones))
	for i := range phones {
		p := phones[i]
		days := retention
		if p.RecycledAt != nil {
			purgeAt := p.RecycledAt.AddDate(0, 0, retention)
			days = int(math.Ceil(purgeAt.Sub(now).Hours() / 24))
			if days < 0 {
				days = 0
			}
		}
		out = append(out, RecycleBinItem{
			ID: p.ID, CpID: p.CpID, Name: p.Name,
			RecycledAt: p.RecycledAt, RecycleReason: p.RecycleReason,
			DaysRemaining: days,
		})
	}
	return out, nil
}

// RecycleBinRestore 手动恢复一台回收站实例（§3）：
//   - 限本人拥有且当前为回收态（否则 NotFound）。
//   - 校验有空闲未过期席位（SeatCapacity > 当前非回收实例数）才允许，不足返回 422。
//   - 置回 STOPPED → 触发 reconcile 重新物化占用。
func (s *serviceImpl) RecycleBinRestore(ctx context.Context, userID, id int) error {
	p, err := s.repo.findRecycledByID(userID, id)
	if err != nil {
		return apperr.NotFound("回收站实例不存在")
	}
	capacity, err := billing.SeatCapacity(userID)
	if err != nil {
		return err
	}
	current, err := s.repo.listNonRecycledByUser(userID)
	if err != nil {
		return err
	}
	if capacity <= len(current) {
		return apperr.Conflict("没有空闲实例席位，无法恢复，请先购买或续费实例席位")
	}
	if err := s.repo.restoreRecycled(userID, int(p.ID)); err != nil {
		return err
	}
	// 重新纳入席位 reconcile（物化占用）。best-effort：恢复已落库，reconcile 失败下轮巡检兜底。
	if err := s.reconcileSeats(ctx, userID); err != nil {
		log.Printf("[recycle] 恢复后 reconcile 失败 user=%d id=%d: %v", userID, p.ID, err)
	}
	return nil
}

// runRecycleCleanup 回收站清理（每日 cron §3）：回收超保留天数的实例 → 调中台销毁 + 硬删本地记录。
// 中台销毁并行执行；本地清理串行（保一致性）。
func (s *serviceImpl) runRecycleCleanup(ctx context.Context) {
	retention, err := billing.RecycleRetentionDays()
	if err != nil || retention <= 0 {
		retention = 30
	}
	before := time.Now().AddDate(0, 0, -retention)
	phones, err := s.repo.listExpiredRecycled(before)
	if err != nil {
		return
	}
	if len(phones) == 0 {
		return
	}

	// 并行销毁中台实例；results[i].err != nil 表示销毁失败，保留本地记录待下轮重试。
	type destroyResult struct {
		phone CloudPhone
		err   error
	}
	results := make([]destroyResult, len(phones))
	eg, ctx2 := errgroup.WithContext(ctx)
	for i := range phones {
		i, p := i, phones[i]
		eg.Go(func() error {
			if p.CpID == "" || s.ops == nil {
				results[i] = destroyResult{phone: p}
				return nil
			}
			log.Printf("[recycle-cleanup] 销毁过期回收实例 user=%d cp=%s phoneID=%d", p.UserID, p.CpID, p.ID)
			results[i] = destroyResult{phone: p, err: s.ops.Destroy(ctx2, p.CpID)}
			return nil // best-effort：销毁失败不取消其他
		})
	}
	_ = eg.Wait()

	// 串行本地清理：只处理中台销毁成功（或无需销毁）的实例。
	for _, r := range results {
		if r.err != nil {
			continue // 中台销毁失败：保留本地，下轮重试
		}
		if err := s.repo.deleteByID(r.phone.ID); err == nil && r.phone.CpID != "" {
			_ = billing.ReleaseInstanceOccupancy(int(r.phone.UserID), []string{r.phone.CpID})
		}
	}
}
