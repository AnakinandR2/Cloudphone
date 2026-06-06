package midplat

import (
	"context"
	"encoding/json"
	"net/http"
)

// GetSuppliers 调用 GET /open/api/vendor/v1/region/suppliers 获取供应商枚举。
//
// 文档示例里 data 是一个 {"key":"value"} 形状的对象，看起来是 code→描述 的字典，
// 因此返回 map[string]string。如果 data 实际为 null 或空对象，会返回空 map。
func (c *Client) GetSuppliers(ctx context.Context) (map[string]string, error) {
	const path = "/open/api/vendor/v1/region/suppliers"
	raw, err := c.doJSON(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
