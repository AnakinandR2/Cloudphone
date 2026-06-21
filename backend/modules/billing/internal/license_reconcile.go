package billing

import (
	"sort"
	"time"
)

// 席位池化与自动分配（reconcile）的纯逻辑核心。把「未过期席位」与「非回收实例」配对，
// 并判定席位不足时哪些实例溢出回收。不依赖 DB / 时钟，便于测试与复用。

// seatRef 一个未过期席位的引用（仅含分配所需字段）。
type seatRef struct {
	ID       uint
	ExpireAt time.Time
}

// instanceRef 一个非回收实例的引用。
type instanceRef struct {
	CpID      string
	CreatedAt time.Time
}

// seatPair 席位与实例的配对结果。
type seatPair struct {
	SeatID     uint
	InstanceID string
}

// seatPlan reconcile 计划：配对 + 需回收的实例（最新溢出）。
type seatPlan struct {
	Pairs   []seatPair
	Recycle []string
}

// planSeatAssignment 决定实例坐哪个席位、谁溢出回收。
//   - 席位按到期时间倒序（最长命的先分给最老的实例 → 热迁移稳定）
//   - 实例按创建时间正序
//   - 席位不足时，超出部分（最新创建的实例）进回收
func planSeatAssignment(seats []seatRef, instances []instanceRef) seatPlan {
	ss := append([]seatRef(nil), seats...)
	sort.SliceStable(ss, func(i, j int) bool {
		if ss[i].ExpireAt.Equal(ss[j].ExpireAt) {
			return ss[i].ID > ss[j].ID
		}
		return ss[i].ExpireAt.After(ss[j].ExpireAt)
	})
	ins := append([]instanceRef(nil), instances...)
	sort.SliceStable(ins, func(i, j int) bool {
		if ins[i].CreatedAt.Equal(ins[j].CreatedAt) {
			return ins[i].CpID < ins[j].CpID
		}
		return ins[i].CreatedAt.Before(ins[j].CreatedAt)
	})

	plan := seatPlan{}
	n := len(ss)
	if len(ins) < n {
		n = len(ins)
	}
	for i := 0; i < n; i++ {
		plan.Pairs = append(plan.Pairs, seatPair{SeatID: ss[i].ID, InstanceID: ins[i].CpID})
	}
	for i := n; i < len(ins); i++ {
		plan.Recycle = append(plan.Recycle, ins[i].CpID)
	}
	return plan
}
