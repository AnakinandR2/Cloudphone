package midplat

import (
	"context"
	"encoding/json"
	"net/http"
)

// ===== ADB Token 接管（spec v3.25.9 §3.5）=====
//
// 🔑 v3.25.9 重大更正：对外仅保留 /adb/token/enable + /adb/token/disable 两个接口。
// 旧 /adb/operate（及 /adb/enable、/adb/disable、/adb/whitelist/*）全部已废弃，
// 之前 operate 报「所有服务器端口都被用了」+ adbAddress 为 null 的根因就是调用了废弃接口。
// 渠道侧不管理白名单。

// ADBTokenRequest 是 enable/disable 共用的请求体（§3.5.1/§3.5.2）。
//   - Containers：云手机 cpId 列表（必填）。
//   - ValidTime：有效期天数，校验范围 1-999；实测被中台忽略（固定 +60 天，由后台配置决定），
//     这里一律不传（零值 omitempty）。
type ADBTokenRequest struct {
	Containers []string `json:"containers"`
	ValidTime  int      `json:"validTime,omitempty"`
}

// ADBTokenContainer 是 enable/disable 响应里每个容器的 ADB 接入信息。
// 字段为中台下划线命名。disable 时 AdbAddress 为 null（已吊销）。
type ADBTokenContainer struct {
	ContainerID string `json:"container_id"`
	LoginCode   string `json:"login_code"`  // ⭐ ADB 登录 Token，用于 xlogin
	AdbAddress  string `json:"adb_address"` // ⭐ ADB 连接地址（DNS 域名:端口），用于 adb connect
}

// ADBTokenResponse 对应 envelope.data 这一层。
// 中台 enable/disable 的响应是双层嵌套：
//
//	{ code:"SUCCESS", data:{ code:200, message, data:{ containers:[...] } } }
//
// doJSON 已校验并剥掉最外层 envelope.code，这里的 Code/Message 是中层 data.code/message，
// Data.Containers 对应内层 data.data.containers。
type ADBTokenResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Containers []ADBTokenContainer `json:"containers"`
	} `json:"data"`
}

// EnableADBToken 调用 POST /open/api/vendor/v1/adb/token/enable 为指定容器签发 ADB Token。
// 注意：重复调用会签发全新 token（非幂等），旧 token 随即失效——「续期」即重发本接口。
func (c *Client) EnableADBToken(ctx context.Context, req ADBTokenRequest) (*ADBTokenResponse, error) {
	const path = "/open/api/vendor/v1/adb/token/enable"
	return c.doADBToken(ctx, path, req)
}

// DisableADBToken 调用 POST /open/api/vendor/v1/adb/token/disable 吊销 ADB Token（幂等）。
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

// CloudPhoneAdbInfo 是从 §2.6 v2 /cloud-phone/page 抽出的 ADB 连接信息。
//
// ⚠️ 字段名陷阱（§3.5.3）：连接地址在 adbAddr（旧字段），adbAddress 恒为 null 不可用。
// 优先用 enable 响应里的 adb_address；§2.6 仅用于补 adbToken / adbTokenExpiredAt。
type CloudPhoneAdbInfo struct {
	CpID              string `json:"cpId"`
	Status            string `json:"status"`
	AdbAddress        string `json:"adbAddr"`
	AdbToken          string `json:"adbToken"`
	AdbTokenExpiredAt string `json:"adbTokenExpiredAt"`
	IsRooted          bool   `json:"isRooted"` // §2.6：当前是否已 root（root 开关的前置判定从此读）
}

// BatchQueryAdbEnabled 用 §2.6 page（cpIds 过滤）批量判断哪些 cp 已开 ADB（adbToken 非空）。
// 供列表标记「已开 ADB」用；best-effort，查不到的 cp 不出现在返回 map 里。
func (c *Client) BatchQueryAdbEnabled(ctx context.Context, cpIDs []string) (map[string]bool, error) {
	if len(cpIDs) == 0 {
		return map[string]bool{}, nil
	}
	const path = "/open/api/vendor/v1/cloud-phone/page"
	body := map[string]any{"page": 1, "pageSize": len(cpIDs), "cpIds": cpIDs}
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, body)
	if err != nil {
		if IsDataNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	var page struct {
		Data []CloudPhoneAdbInfo `json:"data"`
	}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, err
		}
	}
	out := make(map[string]bool, len(page.Data))
	for _, d := range page.Data {
		out[d.CpID] = d.AdbToken != ""
	}
	return out, nil
}

// BatchQueryRootEnabled 用 §2.6 page（cpIds 过滤）批量判断哪些 cp 已 root（isRooted=true）。
// 供列表标记「已 root」用；best-effort，查不到的 cp 不出现在返回 map 里。
func (c *Client) BatchQueryRootEnabled(ctx context.Context, cpIDs []string) (map[string]bool, error) {
	if len(cpIDs) == 0 {
		return map[string]bool{}, nil
	}
	const path = "/open/api/vendor/v1/cloud-phone/page"
	body := map[string]any{"page": 1, "pageSize": len(cpIDs), "cpIds": cpIDs}
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, body)
	if err != nil {
		if IsDataNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	var page struct {
		Data []CloudPhoneAdbInfo `json:"data"`
	}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, err
		}
	}
	out := make(map[string]bool, len(page.Data))
	for _, d := range page.Data {
		out[d.CpID] = d.IsRooted
	}
	return out, nil
}

// GetCloudPhoneAdbInfo 调用 POST /open/api/vendor/v1/cloud-phone/page（§2.6 v2）取单台云机的
// ADB 连接信息（list 接口不返回 adbToken，只能走 page 详情）。
//
// P1-A10：对不存在的 cpId，中台返回 HTTP 400 + code="DATA_NOT_EXIST"。
// 读不到 ≠ 报错——此时降级为「未开启」（仅带 CpID），交由上层呈现为未开通。
func (c *Client) GetCloudPhoneAdbInfo(ctx context.Context, cpID string) (*CloudPhoneAdbInfo, error) {
	const path = "/open/api/vendor/v1/cloud-phone/page"
	body := map[string]any{"page": 1, "pageSize": 1, "cpId": cpID}
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, body)
	if err != nil {
		if IsDataNotExist(err) {
			return &CloudPhoneAdbInfo{CpID: cpID}, nil
		}
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &CloudPhoneAdbInfo{CpID: cpID}, nil
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
