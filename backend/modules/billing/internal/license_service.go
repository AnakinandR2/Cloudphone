package billing

import (
	"strings"
	"time"
)

// licenseServiceImpl 授权单元服务：池容量、席位 reconcile（自动分配/热迁移/溢出判定）。
type licenseServiceImpl struct{ repo licenseRepository }

// LicenseService 模块内实例，由 module.Init 注入 DB 后装配。
var LicenseService *licenseServiceImpl

func newLicenseService(repo licenseRepository) *licenseServiceImpl {
	return &licenseServiceImpl{repo: repo}
}

// InstanceRef 跨模块传入的实例引用（由 phone 提供：实例 cpId + 创建时间）。
type InstanceRef struct {
	CpID      string
	CreatedAt time.Time
}

// Capacity 某用户某类可用授权单元数（未过期）。
func (s *licenseServiceImpl) Capacity(userID int, kind string) (int, error) {
	units, err := s.repo.activeUnits(userID, kind, time.Now())
	if err != nil {
		return 0, err
	}
	return len(units), nil
}

// LicenseUnitView 续费 tab 列表项（契约 §1.4）。current_instance_id 为空表示空闲单元；
// instance 由实例富化 provider 填充（名称/状态），无 provider 或空闲单元时为 nil。
type LicenseUnitView struct {
	ID                uint          `json:"id"`
	Kind              string        `json:"kind"`
	CreatedAt         time.Time     `json:"created_at"`
	ExpireAt          time.Time     `json:"expire_at"`
	CurrentInstanceID string        `json:"current_instance_id"`
	Instance          *InstanceView `json:"instance"`
}

// InstanceView 续费列表里坐在单元上的实例信息（契约 §1.4 的 instance 对象）。
type InstanceView struct {
	CpID   string `json:"cp_id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// filterUnitsByKeyword 按实例名称或 cpId 模糊过滤续费列表（依赖已富化的 instance 信息）。
func filterUnitsByKeyword(views []LicenseUnitView, kw string) []LicenseUnitView {
	kw = strings.ToLower(kw)
	out := make([]LicenseUnitView, 0, len(views))
	for _, v := range views {
		hay := strings.ToLower(v.CurrentInstanceID)
		if v.Instance != nil {
			hay += " " + strings.ToLower(v.Instance.Name)
		}
		if strings.Contains(hay, kw) {
			out = append(out, v)
		}
	}
	return out
}

// enrichUnitViews 用实例富化 provider 给有占用的单元填充 instance 信息。
func enrichUnitViews(views []LicenseUnitView) {
	cpIDs := make([]string, 0, len(views))
	for _, v := range views {
		if v.CurrentInstanceID != "" {
			cpIDs = append(cpIDs, v.CurrentInstanceID)
		}
	}
	meta := lookupInstanceMeta(cpIDs)
	for i := range views {
		cp := views[i].CurrentInstanceID
		if cp == "" {
			continue
		}
		m := meta[cp]
		views[i].Instance = &InstanceView{CpID: cp, Name: m.Name, Status: m.Status}
	}
}

// ListActiveUnits 列某用户某类未过期授权单元（续费用）。expiringBefore 为零值时不过滤。
func (s *licenseServiceImpl) ListActiveUnits(userID int, kind string, expiringBefore time.Time) ([]LicenseUnitView, error) {
	units, err := s.repo.activeUnits(userID, kind, time.Now())
	if err != nil {
		return nil, err
	}
	out := make([]LicenseUnitView, 0, len(units))
	for _, u := range units {
		if !expiringBefore.IsZero() && u.ExpireAt.After(expiringBefore) {
			continue
		}
		out = append(out, LicenseUnitView{
			ID: u.ID, Kind: u.Kind, CreatedAt: u.CreatedAt,
			ExpireAt: u.ExpireAt, CurrentInstanceID: u.CurrentInstanceID,
		})
	}
	enrichUnitViews(out)
	return out, nil
}

// ReconcileSeats 重新分配席位池：物化「实例坐哪个席位」，返回因席位不足需进回收站的实例 cpId（最新溢出）。
func (s *licenseServiceImpl) ReconcileSeats(userID int, instances []InstanceRef) ([]string, error) {
	now := time.Now()
	units, err := s.repo.activeUnits(userID, KindSeat, now)
	if err != nil {
		return nil, err
	}
	seats := make([]seatRef, 0, len(units))
	for _, u := range units {
		seats = append(seats, seatRef{ID: u.ID, ExpireAt: u.ExpireAt})
	}
	insRefs := make([]instanceRef, 0, len(instances))
	for _, in := range instances {
		insRefs = append(insRefs, instanceRef{CpID: in.CpID, CreatedAt: in.CreatedAt})
	}
	plan := planSeatAssignment(seats, insRefs)

	// 物化占用：先清空本用户所有席位的占用，再按计划逐个落座（热迁移即重新落座）。
	if err := s.repo.clearAllOccupancy(userID, KindSeat); err != nil {
		return nil, err
	}
	for _, p := range plan.Pairs {
		if err := s.repo.setInstance(p.SeatID, p.InstanceID); err != nil {
			return nil, err
		}
	}
	return plan.Recycle, nil
}
