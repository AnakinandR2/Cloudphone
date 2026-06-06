package midplat

import (
	"context"
	"encoding/json"
	"net/http"
)

// ADBTokenRequest 是 enable/disable 共用的请求体。
//   - Containers 为云手机/容器 ID 列表（必填）。
//   - ValidTime 为有效期，单位/语义按中台约定，可选；零值序列化时省略。
type ADBTokenRequest struct {
	Containers []string `json:"containers"`
	ValidTime  int      `json:"validTime,omitempty"`
}

// ADBTokenContainer 是 enable/disable 响应里每个容器的 ADB 接入信息。
type ADBTokenContainer struct {
	ContainerID string `json:"container_id"`
	LoginCode   string `json:"login_code"`
	AdbAddress  string `json:"adb_address"`
}

// ADBTokenResponse 是 enable/disable 接口返回的结构。
// 中台外层 envelope.data 内还嵌套一层 {code,message,data:{containers:[]}}，
// 这里对应内层结构；外层的 envelope.code 已经由 doJSON 校验过。
type ADBTokenResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Containers []ADBTokenContainer `json:"containers"`
	} `json:"data"`
}

// EnableADBToken 调用 POST /open/api/vendor/v1/adb/token/enable 为指定容器开启 ADB 接入。
func (c *Client) EnableADBToken(ctx context.Context, req ADBTokenRequest) (*ADBTokenResponse, error) {
	const path = "/open/api/vendor/v1/adb/token/enable"
	return c.doADBToken(ctx, path, req)
}

// DisableADBToken 调用 POST /open/api/vendor/v1/adb/token/disable 关闭指定容器的 ADB 接入。
func (c *Client) DisableADBToken(ctx context.Context, req ADBTokenRequest) (*ADBTokenResponse, error) {
	const path = "/open/api/vendor/v1/adb/token/disable"
	return c.doADBToken(ctx, path, req)
}

func (c *Client) doADBToken(ctx context.Context, path string, req ADBTokenRequest) (*ADBTokenResponse, error) {
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &ADBTokenResponse{}, nil
	}
	var out ADBTokenResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ===== ADB 统一操作（spec §3.1 /adb/operate）+ 白名单查询（§3.2）+ 连接信息（§2.16 page）=====

// AdbOperateRequest 是 /adb/operate 的请求体（spec §3.1）。
// operation: enable / disable / update_whitelist（全小写）。
// 不传 vmId：中台按 containers 自行定位宿主 VM。
type AdbOperateRequest struct {
	Operation  string   `json:"operation"`
	Containers []string `json:"containers"`
	WhiteIP    []string `json:"whiteIp,omitempty"`
	TTL        int      `json:"ttl,omitempty"`
}

// AdbContainerDetail 是 /adb/operate 响应里每台机的结果。
type AdbContainerDetail struct {
	ContainerID string `json:"containerId"`
	Proxy       string `json:"proxy"`
	Success     bool   `json:"success"`
	Error       string `json:"error"`
}

// AdbOperateResult 是 /adb/operate 的 data。
type AdbOperateResult struct {
	Operation         string               `json:"operation"`
	VmID              string               `json:"vmId"`
	AllSuccess        bool                 `json:"allSuccess"`
	SuccessContainers []string             `json:"successContainers"`
	FailedContainers  []string             `json:"failedContainers"`
	ErrorMessage      string               `json:"errorMessage"`
	ContainerDetails  []AdbContainerDetail `json:"containerDetails"`
}

// OperateAdb 调用 POST /open/api/vendor/v1/adb/operate 统一开启/关闭 ADB、改白名单。
func (c *Client) OperateAdb(ctx context.Context, req AdbOperateRequest) (*AdbOperateResult, error) {
	const path = "/open/api/vendor/v1/adb/operate"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &AdbOperateResult{}, nil
	}
	var out AdbOperateResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AdbWhitelistEntry 是 /adb/whitelist/cp/{cpId} 返回的一条白名单记录（spec §3.2）。
type AdbWhitelistEntry struct {
	ID         int64  `json:"id"`
	CpID       string `json:"cpId"`
	VmID       string `json:"vmId"`
	IPAddress  string `json:"ipAddress"`
	IPDesc     string `json:"ipDesc"`
	Status     int    `json:"status"`
	StatusDesc string `json:"statusDesc"`
	ExpireTime string `json:"expireTime"`
	Expired    bool   `json:"expired"`
	CreateTime string `json:"createTime"`
	UpdateTime string `json:"updateTime"`
	CreateBy   string `json:"createBy"`
	UpdateBy   string `json:"updateBy"`
}

// GetAdbWhitelist 调用 GET /open/api/vendor/v1/adb/whitelist/cp/{cpId} 查白名单。
func (c *Client) GetAdbWhitelist(ctx context.Context, cpID string) ([]AdbWhitelistEntry, error) {
	path := "/open/api/vendor/v1/adb/whitelist/cp/" + cpID
	raw, err := c.doJSON(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []AdbWhitelistEntry
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CloudPhoneAdbInfo 是从 §2.16 /cloud-phone/page 抽出的 ADB 连接信息。
type CloudPhoneAdbInfo struct {
	CpID              string `json:"cpId"`
	Status            string `json:"status"`
	AdbAddress        string `json:"adbAddress"`
	AdbToken          string `json:"adbToken"`
	AdbTokenExpiredAt string `json:"adbTokenExpiredAt"`
}

// GetCloudPhoneAdbInfo 调用 POST /open/api/vendor/v1/cloud-phone/page（§2.16）取单台云机的 ADB 连接信息
// （list 接口不返回 adbToken，只能走 page 详情）。
func (c *Client) GetCloudPhoneAdbInfo(ctx context.Context, cpID string) (*CloudPhoneAdbInfo, error) {
	const path = "/open/api/vendor/v1/cloud-phone/page"
	body := map[string]any{"page": 1, "pageSize": 1, "cpId": cpID}
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, body)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &CloudPhoneAdbInfo{}, nil
	}
	var page struct {
		Data []CloudPhoneAdbInfo `json:"data"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, err
	}
	if len(page.Data) == 0 {
		return &CloudPhoneAdbInfo{CpID: cpID}, nil
	}
	return &page.Data[0], nil
}
