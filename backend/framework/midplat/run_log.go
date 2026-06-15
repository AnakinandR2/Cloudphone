package midplat

import (
	"context"
	"encoding/json"
	"net/http"
)

// ===== 云机运行日志（spec §2.9 /cloud-phone-status-logs/page）=====
// 查询云手机每次「开机→关机」的运行会话日志。服务端分页（请求 page/pageSize，响应 pageNum/pageSize/totalSize）。

// RunLogQueryRequest 是 /cloud-phone-status-logs/page 的请求体（§2.9.1 CloudPhoneStatusLogQueryDTO）。
// 渠道侧只传业务筛选字段，权限收敛字段（tenantId 等）由服务端按 AK 推导，不要传。
type RunLogQueryRequest struct {
	Page           int    `json:"page,omitempty"`
	PageSize       int    `json:"pageSize,omitempty"`
	CpID           string `json:"cpId,omitempty"`
	VmUID          string `json:"vmUid,omitempty"`
	SessionStatus  string `json:"sessionStatus,omitempty"`  // RUNNING / SHUTDOWN；空=全部
	PowerOffReason string `json:"powerOffReason,omitempty"` // SHUTDOWN / CONTAINER_FAULT / SERVER_FAULT / CONTROLLER_FAULT
}

// RunLogEntry 是一条运行会话日志（§2.9.1 data.data[]）。展示值为中台已格式化的字符串。
type RunLogEntry struct {
	RowNo              int    `json:"rowNo"`
	LogNo              string `json:"logNo"`
	CpID               string `json:"cpId"`
	VmUID              string `json:"vmUid"`
	PowerOnTime        string `json:"powerOnTime"`        // yyyy-MM-dd HH:mm:ss
	PowerOffTime       string `json:"powerOffTime"`       // 运行中返回「运行中」
	Duration           string `json:"duration"`           // 如 25m / 3h12m
	PowerOffReason     string `json:"powerOffReason"`     // 正常关机 / 容器异常 / 服务器异常 / 控制器异常 / —
	PowerOffReasonCode string `json:"powerOffReasonCode"` // SHUTDOWN / CONTAINER_FAULT / ...；运行中为 null
	SessionStatus      string `json:"sessionStatus"`      // 运行中 / 已关机
	TenantName         string `json:"tenantName"`
	UpdateTime         string `json:"updateTime"`
}

// RunLogPage 是 §2.9.1 的 data（PageRespVO）。注意响应分页字段名是 pageNum/totalSize。
type RunLogPage struct {
	PageNum   int           `json:"pageNum"`
	PageSize  int           `json:"pageSize"`
	TotalSize int64         `json:"totalSize"`
	Data      []RunLogEntry `json:"data"`
}

// QueryRunLogs 调用 POST /open/api/vendor/v1/cloud-phone-status-logs/page 分页查询运行日志。
func (c *Client) QueryRunLogs(ctx context.Context, req RunLogQueryRequest) (*RunLogPage, error) {
	const path = "/open/api/vendor/v1/cloud-phone-status-logs/page"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &RunLogPage{}, nil
	}
	var out RunLogPage
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
