package midplat

import (
	"context"
	"encoding/json"
	"net/http"
)

// PhysicalMachine 对应 /physical-machine/page 返回的物理机条目。
type PhysicalMachine struct {
	ID         int64  `json:"id"`
	PmID       string `json:"pmId"`
	PmName     string `json:"pmName"`
	PmIP       string `json:"pmIp"`
	PmCPU      int    `json:"pmCpu"`
	PmMemory   int    `json:"pmMemory"`
	PmStorage  int    `json:"pmStorage"`
	PmGpuCount int    `json:"pmGpuCount"`
	DcID       string `json:"dcId"`
	DcName     string `json:"dcName"`
	DcRegion   string `json:"dcRegion"`
	CreateTime string `json:"createTime"`
	UpdateTime string `json:"updateTime"`
	CreateBy   string `json:"createBy"`
	UpdateBy   string `json:"updateBy"`
}

// ListPhysicalMachinesRequest 是查询物理机列表的过滤条件。
type ListPhysicalMachinesRequest struct {
	PmIDs []string `json:"pmIds,omitempty"`
}

// ListPhysicalMachines 调用 POST /open/api/vendor/v1/physical-machine/page 查询物理机列表。
//
// 文档示例里 data 是单个对象，但接口名是 list/page，因此兼容数组与单对象两种返回形态，
// 统一返回切片。
func (c *Client) ListPhysicalMachines(ctx context.Context, req ListPhysicalMachinesRequest) ([]PhysicalMachine, error) {
	const path = "/open/api/vendor/v1/physical-machine/page"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	return decodePMList(raw)
}

func decodePMList(raw json.RawMessage) ([]PhysicalMachine, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	// 先按数组解析。
	if raw[0] == '[' {
		var out []PhysicalMachine
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
	// 兜底按单对象解析。
	var one PhysicalMachine
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, err
	}
	return []PhysicalMachine{one}, nil
}
