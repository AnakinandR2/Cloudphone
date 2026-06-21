package phone

import (
	"context"
	"log"
	"time"

	"manager-backend/modules/billing"
)

// settlementLookback 结算时回看的会话窗口：覆盖正常 1 分钟水位推进 + 短暂停机补算。
const settlementLookback = 6 * time.Hour

// runSettlement 结算编排：按用户取近窗口运行区间，调 billing 逐分钟覆盖扣费（billing 内部按水位裁剪、幂等）。
func (s *serviceImpl) runSettlement(ctx context.Context) {
	now := time.Now()
	sessions, err := s.repo.sessionsOverlapping(now.Add(-settlementLookback))
	if err != nil {
		return
	}
	byUser := map[uint][]billing.RuntimeInterval{}
	for _, ss := range sessions {
		// 填充 CpID/RunSessionRef：新引擎据此按台封顶、按会话幂等结算 + 费用日志聚合。
		iv := billing.RuntimeInterval{Start: ss.PowerOnAt, CpID: ss.CpID, RunSessionRef: ss.LogNo}
		if ss.PowerOffAt != nil {
			iv.End = ss.PowerOffAt
		}
		byUser[ss.UserID] = append(byUser[ss.UserID], iv)
	}
	for uid, ivs := range byUser {
		if _, err := billing.SettleRuntime(int(uid), now, ivs); err != nil {
			log.Printf("[metering] 结算失败 user=%d: %v", uid, err)
		}
	}
	_ = ctx
}

// runRuntimeGuard 准实时护栏：对每个用户，若余额/时长包不足以支撑「超出开机席位」的运行中台的下一分钟，
// 关停其中无法覆盖的台（后开先关，保护先开 + 席位内的台）。
func (s *serviceImpl) runRuntimeGuard(ctx context.Context) {
	if s.ops == nil {
		return
	}
	running, err := s.repo.runningSessions() // 已按 user_id, power_on_at 升序
	if err != nil {
		return
	}
	byUser := map[uint][]RunSession{}
	order := []uint{}
	for _, rs := range running {
		if _, ok := byUser[rs.UserID]; !ok {
			order = append(order, rs.UserID)
		}
		byUser[rs.UserID] = append(byUser[rs.UserID], rs)
	}

	for _, uid := range order {
		sess := byUser[uid] // power_on_at 升序：越靠后越晚开机
		// 新模型：包月开机名额(boot_slot)免费，超出名额的台靠临时时长支撑下一分钟。
		bootCap, _ := billing.BootSlotCapacity(int(uid))
		remain, _ := billing.RuntimeMinutesRemaining(int(uid))
		over := int64(len(sess)) - int64(bootCap) // 超出包月名额的运行中台数
		if over <= 0 {
			continue // 全在名额内，免费
		}
		// 下一分钟可支撑的超额台数 = 剩余临时时长分钟（每台·分钟）。新模型不再用余额折算分钟。
		budget := remain
		unfundable := over - budget
		if unfundable <= 0 {
			continue // 下一分钟付得起，暂不关
		}
		// 关最后开机的 unfundable 台（后开先关）。
		toClose := sess[int64(len(sess))-unfundable:]
		s.shutdownSessions(ctx, int(uid), toClose)
	}
}

// shutdownSessions 关停给定运行中会话对应的实例（幂等：跳过过渡态/已关）。
func (s *serviceImpl) shutdownSessions(ctx context.Context, userID int, sessions []RunSession) {
	phones, err := s.repo.listByUser(userID)
	if err != nil {
		return
	}
	byCp := map[string]*CloudPhone{}
	for i := range phones {
		byCp[phones[i].CpID] = &phones[i]
	}
	for _, ss := range sessions {
		p, ok := byCp[ss.CpID]
		if !ok || p.CpID == "" || p.Status != StatusRunning {
			continue
		}
		log.Printf("[runtime-guard] 余额不足关停超额实例 user=%d cp=%s", userID, p.CpID)
		if err := s.ops.StartOrShutdown(ctx, p.CpID, "关机"); err != nil {
			continue
		}
		_ = s.repo.setStatus(p.ID, StatusStopping)
	}
}
