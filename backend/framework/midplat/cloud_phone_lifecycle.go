// cloud_phone_lifecycle.go：云手机生命周期（create/destroy/startOrShutdown/batchReset/batchRestart）。
// 全部为异步接口（中台后台执行）。
package midplat

import (
	"context"
	"encoding/json"
	"net/http"
)

// CreateCPRequest 是 POST /cp/create 的请求体。
//
// 重要：真实中台的创建端点是 **/open/api/vendor/v1/cp/create**（不是 cloud-phone/create，
// 后者上游无 handler，会返回 "No static resource"）。且 minCore/maxCore/minMemory/maxMemory/
// storage/width/height/fps/maxBootCount 均为「不可为空」的必填项——取值来自所选套餐(plan)
// 及其启动参数(bootParams)。校验过的字段集见 cp-glory-service 官方控制台。
// 无代理时不要下发 proxyMode（合法枚举仅 ADDED/CUSTOM/SHENLONG）。
type CreateCPRequest struct {
	Number  int64  `json:"number"`
	VmID    string `json:"vmId"` // 云虚机的 UUID（ServerInfo.VmID，不是人类可读的 VmUID）
	ImageID string `json:"imageId"`
	PlanID  int    `json:"planId"`

	// 套餐资源约束（必填，来自 BootPlan）。用非指针确保始终序列化，避免 omitempty 丢字段触发校验失败。
	MinCore   float64 `json:"minCore"`
	MaxCore   float64 `json:"maxCore"`
	MinMemory int     `json:"minMemory"`
	MaxMemory int     `json:"maxMemory"`
	Storage   int     `json:"storage"`

	Bandwidth int `json:"bandwidth,omitempty"`

	// 这些整型字段中台用 Integer 接收并直接拆箱，留空(null)会触发拆箱 NPE（"创建失败: null"），
	// 故用指针 + 显式置值始终下发（官方控制台同样恒发）。
	ScenarioID  *int `json:"scenarioId,omitempty"`
	NetworkType *int `json:"networkType,omitempty"`
	BootParamID int  `json:"bootParamId,omitempty"`

	// 启动参数（必填，来自 BootPlan.BootParamsList）。
	Width        int `json:"width"`
	Height       int `json:"height"`
	FPS          int `json:"fps"`
	MaxBootCount int `json:"maxBootCount"`

	// 中台创建逻辑会对这些字符串字段做 .length()/枚举解析，留空(null)会触发服务端 NPE，
	// 因此即便无业务含义也按官方控制台默认值下发。
	ResourceUtilization string `json:"resourceUtilization,omitempty"` // 默认 SHARED
	AndroidVersion      string `json:"androidVersion,omitempty"`      // 取自镜像
	RegionOption        string `json:"regionOption,omitempty"`        // 无代理时 CUSTOM
	TimezoneOption      string `json:"timezoneOption,omitempty"`      // 无代理时 CUSTOM
	LanguageOption      string `json:"languageOption,omitempty"`      // 无代理时 CUSTOM

	Region string `json:"region,omitempty"`

	// 无代理时按官方控制台显式下发 null（不加 omitempty），与已验证可用的请求逐字节一致。
	ProxyMode    *string          `json:"proxyMode"`
	ProxyConfigs []map[string]any `json:"proxyConfigs"`
}

// CreateCPResponse 是创建异步返回，分配到的 cpId 列表。中台不同版本可能用 cpList 或 cpIds，两者都兼容。
type CreateCPResponse struct {
	CpList []string `json:"cpList"`
	CpIDs  []string `json:"cpIds"`
}

// IDs 返回兼容两种字段名后的 cpId 列表。
func (r *CreateCPResponse) IDs() []string {
	if len(r.CpList) > 0 {
		return r.CpList
	}
	return r.CpIDs
}

// CreateCloudPhones 调用 POST /cp/create 异步创建云手机。
func (c *Client) CreateCloudPhones(ctx context.Context, req CreateCPRequest) (*CreateCPResponse, error) {
	const path = "/open/api/vendor/v1/cp/create"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &CreateCPResponse{}, nil
	}
	var out CreateCPResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DestroyCloudPhones 调用 POST /cloud-phone/destroy 异步销毁云手机。
func (c *Client) DestroyCloudPhones(ctx context.Context, cpIDs []string) error {
	const path = "/open/api/vendor/v1/cloud-phone/destroy"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, cpIDsRequest{CpIDs: cpIDs})
	return err
}

// StartCloudPhones 调用 POST /cp/start 批量开机（异步）。请求体 {cpIds:[...]}。
// 注：真实中台开/关机是分开的两个端点（cp/start、cp/shutdown），不是 cloud-phone/startOrShutdown。
func (c *Client) StartCloudPhones(ctx context.Context, cpIDs []string) error {
	const path = "/open/api/vendor/v1/cp/start"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, cpIDsRequest{CpIDs: cpIDs})
	return err
}

// ShutdownCloudPhones 调用 POST /cp/shutdown 批量关机（异步）。请求体 {cpIds:[...]}。
func (c *Client) ShutdownCloudPhones(ctx context.Context, cpIDs []string) error {
	const path = "/open/api/vendor/v1/cp/shutdown"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, cpIDsRequest{CpIDs: cpIDs})
	return err
}

// ResetSimulation / ResetProxy 是 BatchResetCloudPhones 里子结构。
type ResetSimulation struct {
	DeviceModel string `json:"deviceModel,omitempty"`
	IMEI        string `json:"imei,omitempty"`
	AndroidID   string `json:"androidId,omitempty"`
}

type ResetProxy struct {
	Enabled  bool     `json:"enabled,omitempty"`
	Host     string   `json:"host,omitempty"`
	Port     int      `json:"port,omitempty"`
	Username string   `json:"username,omitempty"`
	Password string   `json:"password,omitempty"`
	Type     string   `json:"type,omitempty"` // socks5/http/https
	Blacks   []string `json:"blacks,omitempty"`
}

// PhoneResetOperation 是 batchReset 单台手机的操作描述。
type PhoneResetOperation struct {
	CpID            string           `json:"cpId"`
	ImageID         string           `json:"imageId,omitempty"`
	AppIDs          []int64          `json:"appIds,omitempty"`
	UninstallAppIDs []int64          `json:"uninstallAppIds,omitempty"`
	Root            *bool            `json:"root,omitempty"`
	DisableAdb      *bool            `json:"disableAdb,omitempty"`
	AdbWhiteIP      []string         `json:"adbWhiteIp,omitempty"`
	AdbTTL          int              `json:"adbTtl,omitempty"`
	Simulation      *ResetSimulation `json:"simulation,omitempty"`
	Proxy           *ResetProxy      `json:"proxy,omitempty"`
}

type batchResetRequest struct {
	PhoneResetOperations []PhoneResetOperation `json:"phoneResetOperations"`
}

// BatchResetCloudPhones 调用 POST /cloud-phone/batchReset 批量重置（异步）。
func (c *Client) BatchResetCloudPhones(ctx context.Context, ops []PhoneResetOperation) error {
	const path = "/open/api/vendor/v1/cloud-phone/batchReset"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, batchResetRequest{PhoneResetOperations: ops})
	return err
}

// RestartCloudPhones 调用 POST /cloud-phone/restart 批量重启（异步）。请求体 {cpIds:[...]}。
// 注：真实中台重启端点是 /cloud-phone/restart（不是 /cp/restart，也不是 batchRestart）。
func (c *Client) RestartCloudPhones(ctx context.Context, cpIDs []string) error {
	const path = "/open/api/vendor/v1/cloud-phone/restart"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, cpIDsRequest{CpIDs: cpIDs})
	return err
}

// refreshPhoneRequest 是 batchRefreshPhone 的请求体。keepbindAgent 默认 true（保留代理绑定）。
type refreshPhoneRequest struct {
	CpIDs         []string `json:"cpIds"`
	KeepBindAgent bool     `json:"keepbindAgent"`
}

// RefreshCloudPhones 调用 POST /cloud-phone/batchRefreshPhone「一键刷新（一键新机）」：
// 保留 cpId 重新初始化（擦数据 + 重装镜像）。keepBindAgent=true 保留代理绑定。
func (c *Client) RefreshCloudPhones(ctx context.Context, cpIDs []string, keepBindAgent bool) error {
	const path = "/open/api/vendor/v1/cloud-phone/batchRefreshPhone"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, refreshPhoneRequest{CpIDs: cpIDs, KeepBindAgent: keepBindAgent})
	return err
}
