package phone

import (
	"context"
	"log"

	"manager-backend/modules/billing"
)

// runEnforcement 读 billing 欠费执行目标：frozen→强制关机运行中实例；recycled→销毁超量实例。
// 并以 phone 真实计数校正 billing 占用(reconcile)。幂等：按状态/数量条件执行。
func (s *serviceImpl) runEnforcement(ctx context.Context) {
	if s.ops == nil {
		return // 本地降级无中台，跳过
	}
	targets, err := billing.ListDunningEnforcement()
	if err != nil {
		return
	}
	for _, tgt := range targets {
		phones, err := s.repo.listByUser(tgt.UserID)
		if err != nil {
			continue
		}
		_ = billing.ReconcileInstanceSeats(tgt.UserID, len(phones))

		switch tgt.State {
		case billing.DunningFrozen:
			for _, p := range phones {
				if p.Status == StatusRunning && p.CpID != "" {
					_ = s.ops.StartOrShutdown(ctx, p.CpID, "关机")
					_ = s.repo.setStatus(p.ID, StatusStopping)
				}
			}
		case billing.DunningRecycled:
			if int64(len(phones)) <= tgt.Capacity {
				continue
			}
			// 保留容量内最早创建的；回收其余(最新的超量部分)。
			overTail := phones[int(tgt.Capacity):]
			for _, p := range overTail {
				// 跳过过渡态实例(创建中/开机中/关机中/销毁中)，避免破坏状态机与重复销毁，留待下轮收敛后回收。
				if p.Status == StatusCreating || p.Status == StatusStarting || p.Status == StatusStopping || p.Status == StatusDestroying {
					continue
				}
				if p.CpID != "" {
					// 审计：不可逆销毁前留痕(含容量=0 时的全量回收)。
					log.Printf("[enforcement] 回收销毁超量实例 user=%d cp=%s phoneID=%d", tgt.UserID, p.CpID, p.ID)
					if err := s.ops.Destroy(ctx, p.CpID); err != nil {
						continue // 中台销毁失败：不删本地，留待下轮重试，避免中台孤儿
					}
				}
				if err := s.repo.deleteByID(p.ID); err == nil {
					_ = billing.ReleaseInstanceSeat(tgt.UserID)
				}
			}
		}
	}
}
