package phone

import (
	"context"

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
			over := len(phones) - int(tgt.Capacity)
			if over <= 0 {
				continue
			}
			// phones 按 id 升序；回收末尾 over 台(最近创建)，保留容量内最早创建的。
			victims := phones[len(phones)-over:]
			for _, p := range victims {
				if p.CpID != "" {
					_ = s.ops.Destroy(ctx, p.CpID)
				}
				if err := s.repo.deleteByID(p.ID); err == nil {
					_ = billing.ReleaseInstanceSeat(tgt.UserID)
				}
			}
		}
	}
}
