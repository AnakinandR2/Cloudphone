// cloud_phone_provision.go：创建云手机所需的「服务器→套餐→镜像」级联查询，
// 以及批量状态查询。端点与请求/响应形态对齐官方控制台 cp-glory-service（生产在用）。
package midplat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ---- 服务器（租户已分配的云虚机，server/page）----

// Server 是 /server/page 返回的单台服务器（spec §5.4 data[]，即「租户已分配的云虚机/云主机」）。
type Server struct {
	ID                int64  `json:"id"`
	VmID              string `json:"vmId"`  // 虚机编号；创建云手机时用它（cp/create 的 vmId）
	VmUID             string `json:"vmUid"` // 虚机唯一标识，人类可读编号如 VM00001043（前端展示用）
	VmIP              string `json:"vmIp"`
	VmStatus          string `json:"vmStatus"`
	IsMaintain        bool   `json:"isMaintain"`
	SpecificationID   int    `json:"specificationId"`
	SpecName          string `json:"specificationName"`
	Core              int    `json:"core"`
	Memory            int    `json:"memory"`
	Storage           int    `json:"storage"`
	Bandwidth         int    `json:"bandwidth"`
	MaxStartCount     int    `json:"maxStartCount"`
	MaxPhone          int    `json:"maxPhone"`
	CreatePhoneNumber int    `json:"createPhoneNumber"`
	CreateTime        string `json:"createTime"`
	ExpireTime        string `json:"expireTime"`
	Expired           bool   `json:"expired"`
}

// ListServersRequest 是 /server/page 的过滤条件（spec §5.4 请求）。
type ListServersRequest struct {
	PageNum      int      `json:"page"`
	PageSize     int      `json:"pageSize"`
	VmUID        string   `json:"vmUid,omitempty"`
	VmIP         string   `json:"vmIp,omitempty"`
	VmStatusList []string `json:"vmStatusList,omitempty"`
}

// 中台分页返回把记录数组放在 data 字段（MyBatis-Plus 风格），兼容 list 兜底。
type serversPage struct {
	Data      []Server `json:"data"`
	List      []Server `json:"list"`
	TotalSize int      `json:"totalSize"`
}

func (p serversPage) rows() []Server {
	if len(p.Data) > 0 {
		return p.Data
	}
	return p.List
}

// ListServers 调用 POST /server/page 查询租户已分配的服务器（分页）。
func (c *Client) ListServers(ctx context.Context, req ListServersRequest) ([]Server, error) {
	const path = "/open/api/vendor/v1/server/page"
	if req.PageNum == 0 {
		req.PageNum = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 100
	}
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	var page serversPage
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, err
	}
	return page.rows(), nil
}

// ---- 套餐（plan）与启动参数 ----

// BootPlan 是 /plan-mng/list/by-spec-and-scenario 返回的套餐行（含创建所需的资源约束）。
// 实测响应里「单台云手机」资源是 minCore/maxCore/minMemory/maxMemory/storage/bandwidth（core/memory 常为 null），
// 而 specCore/specMemory/specStorage/specBandwidth 是「宿主云主机」的总量——两者别混用。
// cp/create（§2.1）的 minCore/maxCore/minMemory/maxMemory/storage/bandwidth 直接用前一组，映射在 modules/phone 的 Create 里。
type BootPlan struct {
	ID                  int    `json:"id"`
	SpecID              int    `json:"specId"`
	PlanNameID          int    `json:"planNameId"`
	PlanName            string `json:"planName"`
	ScenarioName        string `json:"scenarioName"`
	SpecName            string `json:"specName"`
	Supplier            string `json:"supplier"`
	EnableStatus        int    `json:"enableStatus"`
	ResourceUtilization string `json:"resourceUtilization"` // SHARED / EXCLUSIVE
	Width               int    `json:"width"`
	Height              int    `json:"height"`
	FPS                 int    `json:"fps"`
	CreateNum           int    `json:"createNum"`
	Status              int    `json:"status"`
	// --- 云手机（单机）资源：开机后这台手机拿到的规格 ---
	Core      int     `json:"core"`      // 多为 null；优先用 min/maxCore
	MinCore   float64 `json:"minCore"`   // 手机 CPU 下限（如 0.2）
	MaxCore   float64 `json:"maxCore"`   // 手机 CPU 上限（如 0.8）
	Memory    int     `json:"memory"`    // 多为 null；优先用 min/maxMemory
	MinMemory int     `json:"minMemory"` // 手机内存下限 GB（如 1）
	MaxMemory int     `json:"maxMemory"` // 手机内存上限 GB（如 4）
	Storage   int     `json:"storage"`   // 手机存储 GB（如 20）
	Bandwidth int     `json:"bandwidth"` // 手机带宽
	// --- 宿主规格（这台云主机本身的资源，不是单台手机的）---
	SpecCore       int          `json:"specCore"`      // 宿主总核数（如 30）
	SpecMemory     int          `json:"specMemory"`    // 宿主总内存 GB（如 112）
	SpecStorage    int          `json:"specStorage"`   // 宿主总存储 GB（如 500）
	SpecBandwidth  int          `json:"specBandwidth"` // 宿主带宽
	MaxPhone       int          `json:"maxPhone"`
	PhoneCount     int          `json:"phoneCount"`
	BootParamsList []BootParams `json:"bootParamsList"`
}

// BootParams 是套餐内一组分辨率/帧率预设（spec §5.6 bootParamsList[]）。
type BootParams struct {
	ID                   int `json:"id"`
	PlanID               int `json:"planId"`
	Width                int `json:"width"`
	Height               int `json:"height"`
	FPS                  int `json:"fps"`
	MaxBootCount         int `json:"maxBootCount"`
	RecommendedBootCount int `json:"recommendedBootCount"`
	MaxStartCount        int `json:"maxStartCount"`
}

// ListBootPlansBySpec 调用 POST /plan-mng/list/by-spec-and-scenario 按规格查套餐。
func (c *Client) ListBootPlansBySpec(ctx context.Context, specID int) ([]BootPlan, error) {
	const path = "/open/api/vendor/v1/plan-mng/list/by-spec-and-scenario"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, map[string]interface{}{"specId": specID})
	if err != nil {
		return nil, err
	}
	return parseMidplatArray[BootPlan](raw)
}

// PlanImage 是 /img/query/by-plan/{planId} 返回的镜像行。
type PlanImage struct {
	ImageID        string `json:"imageId"`
	ImageName      string `json:"imageName"`
	AndroidVersion string `json:"androidVersion"`
	IsForbid       *int   `json:"isForbid"`
}

// ListImagesByPlan 调用 GET /img/query/by-plan/{planId} 查套餐可用镜像（已剔除 isForbid=1）。
func (c *Client) ListImagesByPlan(ctx context.Context, planID int) ([]PlanImage, error) {
	path := fmt.Sprintf("/open/api/vendor/v1/img/query/by-plan/%d", planID)
	raw, err := c.doJSON(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	imgs, err := parseMidplatArray[PlanImage](raw)
	if err != nil {
		return nil, err
	}
	kept := imgs[:0]
	for _, im := range imgs {
		if im.IsForbid != nil && *im.IsForbid == 1 {
			continue
		}
		kept = append(kept, im)
	}
	return kept, nil
}

// ---- 批量状态查询 ----

// CPRuntimeStatus 是 /cloud-phone/batch-query-status 返回的单台实时状态。
type CPRuntimeStatus struct {
	CpUID  string `json:"cpUid"`
	CpID   string `json:"cpId"`
	Status string `json:"status"` // ONLINE 表示运行中（见 MidplatReady）
}

// BatchQueryStatus 调用 POST /cloud-phone/batch-query-status 批量查云手机实时状态。
func (c *Client) BatchQueryStatus(ctx context.Context, cpIDs []string) ([]CPRuntimeStatus, error) {
	if len(cpIDs) == 0 {
		return nil, nil
	}
	const path = "/open/api/vendor/v1/cloud-phone/batch-query-status"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, cpIDsRequest{CpIDs: cpIDs})
	if err != nil {
		return nil, err
	}
	return parseMidplatArray[CPRuntimeStatus](raw)
}

// parseMidplatArray 兼容中台「裸数组」或「{list:[...]}/{records:[...]}」两种返回形态。
func parseMidplatArray[T any](raw json.RawMessage) ([]T, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var arr []T
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	var env struct {
		Data    []T `json:"data"`
		List    []T `json:"list"`
		Records []T `json:"records"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	switch {
	case len(env.Data) > 0:
		return env.Data, nil
	case len(env.List) > 0:
		return env.List, nil
	default:
		return env.Records, nil
	}
}
