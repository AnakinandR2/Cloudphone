package midplat

import (
	"context"
	"encoding/json"
	"net/http"
)

// VMSource / VMStatus 是中台返回的枚举字符串。
// 这里用 const 而不是 type，便于直接和文档对照。
const (
	VMSourceTencent = "TENCENT"
	VMSourceXiaoxi  = "XIAOXI"
	VMSourceBaidu   = "BAIDU"

	VMStatusOnline    = "ONLINE"
	VMStatusOffline   = "OFFLINE"
	VMStatusDestroyed = "DESTROYED"
)

// VirtualMachine 对应 /virtual-machine/list 返回的 vmList 元素。
// 字段集对齐文档 ResponsesDataSchema。
type VirtualMachine struct {
	ID                int64  `json:"id"`
	VmID              string `json:"vmId"`
	VmSource          string `json:"vmSource"`
	TenantID          int64  `json:"tenantId"`
	TenantName        string `json:"tenantName"`
	SpecificationID   int64  `json:"specificationId"`
	SpecificationName string `json:"specificationName"`
	VmIP              string `json:"vmIp"`
	HostName          string `json:"hostName"`
	VmCPUModel        string `json:"vmCpuModel"`
	VmCPU             int    `json:"vmCpu"`
	VmMemory          int    `json:"vmMemory"`
	MotherBoard       string `json:"motherBoard"`
	TotalCpNumber     int    `json:"totalCpNumber"`
	RunningCpNumber   int    `json:"runningCpNumber"`
	VmStorage         int    `json:"vmStorage"`
	VmGpuCount        int    `json:"vmGpuCount"`
	VmStatus          string `json:"vmStatus"`
	PmID              string `json:"pmId"`
	PmName            string `json:"pmName"`
	PmIP              string `json:"pmIp"`
	ZoneID            int64  `json:"zoneId"`
	ZoneName          string `json:"name"`
	RegionID          int64  `json:"regionid"`
	ZoneCode          string `json:"zoneCode"`
	Comment           string `json:"comment"`
	CreateTime        string `json:"createTime"`
	UpdateTime        string `json:"updateTime"`
	CreateBy          string `json:"createBy"`
	UpdateBy          string `json:"updateBy"`
}

// ListVMsRequest 是 /virtual-machine/list 的过滤条件。
// 文档中过滤字段很多，这里只暴露常用项，其它情况调用方可以传零值忽略。
type ListVMsRequest struct {
	IDs             []int64  `json:"ids,omitempty"`
	VmID            string   `json:"vmId,omitempty"`
	VmIDs           []string `json:"vmIds,omitempty"`
	VmSource        string   `json:"vmSource,omitempty"`
	VmStatus        string   `json:"vmStatus,omitempty"`
	VmIP            string   `json:"vmIp,omitempty"`
	HostName        string   `json:"hostName,omitempty"`
	PmID            string   `json:"pmId,omitempty"`
	TenantID        int64    `json:"tenantId,omitempty"`
	SpecificationID int64    `json:"specificationId,omitempty"`
	ZoneID          int64    `json:"zoneId,omitempty"`
	AvailZoneID     int64    `json:"availZoneId,omitempty"`
}

// ListVMsResponse 是 /virtual-machine/list 的返回结构，
// 同时包含详情数组和扁平 ID 列表，调用方按需用其中之一。
type ListVMsResponse struct {
	VmList   []VirtualMachine `json:"vmList"`
	VmIDList []string         `json:"vmIdList"`
}

// ListVirtualMachines 调用 POST /open/api/vendor/v1/virtual-machine/list。
func (c *Client) ListVirtualMachines(ctx context.Context, req ListVMsRequest) (*ListVMsResponse, error) {
	const path = "/open/api/vendor/v1/virtual-machine/list"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &ListVMsResponse{}, nil
	}
	var out ListVMsResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListVMStatuses 调用 POST /open/api/vendor/v1/virtual-machine/getStatusList。
//
// 文档示例 data 是 [{key: string}]，每条只有一个键，看起来是 code→描述 的枚举对。
// 我们返回 []map[string]string，保留原始键名，调用方自己判断怎么用。
func (c *Client) ListVMStatuses(ctx context.Context) ([]map[string]string, error) {
	const path = "/open/api/vendor/v1/virtual-machine/getStatusList"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, nil)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []map[string]string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// VMBatchRequest 是 reset/restart 等批量操作共享的请求体。
type VMBatchRequest struct {
	VmIDs []string `json:"vmIds"`
}

// ResetVirtualMachines 调用 POST /open/api/vendor/v1/virtual-machine/reset 重置云虚机。
//
// 注意：这是有副作用的写入操作，会影响目标虚机及其上的云手机实例。
// 单测里不应该 mock 后假装通过，应当通过 MIDPLAT_E2E=1 走真实联调。
func (c *Client) ResetVirtualMachines(ctx context.Context, vmIDs []string) error {
	const path = "/open/api/vendor/v1/virtual-machine/reset"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, VMBatchRequest{VmIDs: vmIDs})
	return err
}

// RestartVirtualMachines 调用 POST /open/api/vendor/v1/virtual-machine/restart 重启云虚机。
// 同样是写入操作，e2e 才跑。
func (c *Client) RestartVirtualMachines(ctx context.Context, vmIDs []string) error {
	const path = "/open/api/vendor/v1/virtual-machine/restart"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, VMBatchRequest{VmIDs: vmIDs})
	return err
}
