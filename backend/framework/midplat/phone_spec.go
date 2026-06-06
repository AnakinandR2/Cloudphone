package midplat

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	neturl "net/url"
)

// PhoneSpec 对应 /phone-spec/query/by-zone 返回的手机规格条目。
type PhoneSpec struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Supplier   string `json:"supplier"`
	Core       int    `json:"core"`
	Memory     int    `json:"memory"`
	Storage    int    `json:"storage"`
	Type       string `json:"type"`
	Feature    string `json:"feature"`
	CreateTime string `json:"createTime"`
}

// ListPhoneSpecsByZone 调用 GET /open/api/vendor/v1/phone-spec/query/by-zone
// 按可用区 ID 查询手机规格列表。
func (c *Client) ListPhoneSpecsByZone(ctx context.Context, zoneID int64) ([]PhoneSpec, error) {
	const path = "/open/api/vendor/v1/phone-spec/query/by-zone"
	q := neturl.Values{}
	q.Set("zoneId", strconv.FormatInt(zoneID, 10))
	raw, err := c.doJSON(ctx, http.MethodGet, path, q, nil)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []PhoneSpec
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
