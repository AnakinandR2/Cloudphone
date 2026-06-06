package midplat

import (
	"context"
	"encoding/json"
	"net/http"
)

// AvailZone 对应 /avail-zone/query/all 返回的可用区条目。
type AvailZone struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	RegionID        int64  `json:"regionId"`
	RegionName      string `json:"regionName"`
	Supplier        string `json:"supplier"`
	ZoneCode        string `json:"zoneCode"`
	ZoneURL         string `json:"zoneUrl"`
	PushStreamURL   string `json:"pushStreamUrl"`
	SignalURL       string `json:"signalUrl"`
	OssURL          string `json:"ossUrl"`
	ImageRepository string `json:"imageRepository"`
	MqURL           string `json:"mqUrl"`
	CreateTime      string `json:"createTime"`
	UpdateTime      string `json:"updateTime"`
}

// ListAvailZones 调用 GET /open/api/vendor/v1/avail-zone/query/all 查询全部可用区。
func (c *Client) ListAvailZones(ctx context.Context) ([]AvailZone, error) {
	const path = "/open/api/vendor/v1/avail-zone/query/all"
	raw, err := c.doJSON(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []AvailZone
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
