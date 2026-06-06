package midplat

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	neturl "net/url"
)

// VirtualSpec 对应 /virtual-spec/query/by-zone 返回的虚机规格条目。
type VirtualSpec struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Supplier   string `json:"supplier"`
	Core       int    `json:"core"`
	Memory     int    `json:"memory"`
	Storage    int    `json:"storage"`
	Card       int    `json:"card"`
	MaxPhone   int    `json:"maxPhone"`
	Type       string `json:"type"`
	Feature    string `json:"feature"`
	CreateTime string `json:"createTime"`
	HasVM      bool   `json:"hasVM"`
}

// ListVirtualSpecsByZone 调用 GET /open/api/vendor/v1/virtual-spec/query/by-zone
// 按可用区 ID 查询虚机规格列表。
func (c *Client) ListVirtualSpecsByZone(ctx context.Context, zoneID int64) ([]VirtualSpec, error) {
	const path = "/open/api/vendor/v1/virtual-spec/query/by-zone"
	q := neturl.Values{}
	q.Set("zoneId", strconv.FormatInt(zoneID, 10))
	raw, err := c.doJSON(ctx, http.MethodGet, path, q, nil)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []VirtualSpec
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
