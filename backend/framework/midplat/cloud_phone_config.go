// cloud_phone_config.go：配置类写入（代理 / 仿真 / 租户分配）。
package midplat

import (
	"context"
	"net/http"
)

// PhoneProxyConfig 是 batchUpdateProxy 单台手机的代理配置。
type PhoneProxyConfig struct {
	Enabled  bool     `json:"enabled"`
	Host     string   `json:"host,omitempty"`
	Port     int      `json:"port,omitempty"`
	Username string   `json:"username,omitempty"`
	Password string   `json:"password,omitempty"`
	Type     string   `json:"type,omitempty"` // socks5/http/https
	Blocks   []string `json:"blocks,omitempty"`
}

// PhoneProxyEntry 把 cpId 和它的代理配置绑定。
type PhoneProxyEntry struct {
	CpID  string           `json:"cpId"`
	Proxy PhoneProxyConfig `json:"proxy"`
}

// batchUpdateProxyRequest 是 /phone-command/proxy/batch-update 的请求体。
// 注意：doc 里外层字段名是 cpIds，但实际是 PhoneProxyEntry 数组。
type batchUpdateProxyRequest struct {
	CpIDs []PhoneProxyEntry `json:"cpIds"`
}

// BatchUpdateProxy 调用 POST /phone-command/proxy/batch-update 批量更新代理配置。
func (c *Client) BatchUpdateProxy(ctx context.Context, entries []PhoneProxyEntry) error {
	const path = "/open/api/vendor/v1/phone-command/proxy/batch-update"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, batchUpdateProxyRequest{CpIDs: entries})
	return err
}

// UpdateSimulationConfigRequest 是 /cloud-phone/updateSimulationConfig 的请求体。
type UpdateSimulationConfigRequest struct {
	CpIDs       []string `json:"cpIds"`
	NetworkType *int     `json:"networkType,omitempty"` // 0:WiFi 1:移动网络
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
	Timezone    string   `json:"timezone,omitempty"`
	Language    string   `json:"language,omitempty"`
	CountryCode string   `json:"countryCode,omitempty"`
}

// UpdateSimulationConfig 调用 POST /cloud-phone/updateSimulationConfig 更新仿真配置。
func (c *Client) UpdateSimulationConfig(ctx context.Context, req UpdateSimulationConfigRequest) error {
	const path = "/open/api/vendor/v1/cloud-phone/updateSimulationConfig"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	return err
}

// AssignToTenantRequest 是 /cloud-phone/assign-to-tenant 的请求体。
type AssignToTenantRequest struct {
	CpIDs      []string `json:"cpIds"`
	TenantID   int64    `json:"tenantId"`
	AssignTime string   `json:"assignTime"` // 过期时间
}

// AssignCloudPhonesToTenant 调用 POST /cloud-phone/assign-to-tenant 把手机分配给租户。
func (c *Client) AssignCloudPhonesToTenant(ctx context.Context, req AssignToTenantRequest) error {
	const path = "/open/api/vendor/v1/cloud-phone/assign-to-tenant"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	return err
}
