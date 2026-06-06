// cloud_phone.go：只读类查询接口（list / getCpIdList / 状态枚举 / 已装应用 / WebRTC 认证 / 仿真信息）。
package midplat

import (
	"context"
	"encoding/json"
	"net/http"
)

// CloudPhone 是 /cloud-phone/list 返回的 cpList 元素。
type CloudPhone struct {
	ID                     int64  `json:"id"`
	CpID                   string `json:"cpId"`
	CpSource               string `json:"cpSource"`
	VmID                   string `json:"vmId"`
	VmIP                   string `json:"vmIp"`
	VmCPU                  int    `json:"vmCpu"`
	VmMemory               int    `json:"vmMemory"`
	VmStorage              int    `json:"vmStorage"`
	VmGpuCount             int    `json:"vmGpuCount"`
	ImageID                string `json:"imageId"`
	ImageName              string `json:"imageName"`
	ImageVersion           string `json:"imageVersion"`
	Status                 string `json:"status"`
	AdbAddress             string `json:"adbAddress"`
	StreamingServerAddress string `json:"streamingServerAddress"`
	WebRTCAddress          string `json:"webrtcAddress"`
	PmID                   string `json:"pmId"`
	PmName                 string `json:"pmName"`
	PmIP                   string `json:"pmIp"`
	PmCPU                  int    `json:"pmCpu"`
	PmMemory               int    `json:"pmMemory"`
	PmStorage              int    `json:"pmStorage"`
	PmGpuCount             int    `json:"pmGpuCount"`
	ZoneID                 int64  `json:"zoneId"`
	ZoneName               string `json:"zoneName"`
	ZoneCode               string `json:"zoneCode"`
	SpecID                 int64  `json:"specId"`
	SpecName               string `json:"specName"`
	SpecSupplier           string `json:"specSupplier"`
	SpecCore               int    `json:"specCore"`
	SpecMemory             int    `json:"specMemory"`
	SpecStorage            int    `json:"specStorage"`
	SpecType               string `json:"specType"`
	SpecFeature            string `json:"specFeature"`
	SpecFlag               string `json:"specFlag"`
	CreateTime             string `json:"createTime"`
	UpdateTime             string `json:"updateTime"`
	CreateBy               string `json:"createBy"`
	UpdateBy               string `json:"updateBy"`
}

// ListCloudPhonesRequest 是 /cloud-phone/list 的过滤条件，所有字段可省略。
type ListCloudPhonesRequest struct {
	CpIDs        []string `json:"cpIds,omitempty"`
	CpID         string   `json:"cpId,omitempty"`
	ImageID      string   `json:"imageId,omitempty"`
	VmID         string   `json:"vmId,omitempty"`
	ZoneName     string   `json:"zoneName,omitempty"`
	Status       string   `json:"status,omitempty"`
	CpSource     string   `json:"cpSource,omitempty"`
	PmID         string   `json:"pmId,omitempty"`
	SpecID       int64    `json:"specId,omitempty"`
	SpecName     string   `json:"specName,omitempty"`
	SpecSupplier string   `json:"specSupplier,omitempty"`
}

// ListCloudPhonesResponse 同时包含详情数组和扁平 ID 列表。
type ListCloudPhonesResponse struct {
	CpList   []CloudPhone `json:"cpList"`
	CpIDList []string     `json:"cpIdList"`
}

// ListCloudPhones 调用 POST /open/api/vendor/v1/cloud-phone/list。
func (c *Client) ListCloudPhones(ctx context.Context, req ListCloudPhonesRequest) (*ListCloudPhonesResponse, error) {
	const path = "/open/api/vendor/v1/cloud-phone/list"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &ListCloudPhonesResponse{}, nil
	}
	var out ListCloudPhonesResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// getCpIdListRequest 是 /cloud-phone/getCpIdList 的请求体（仅内部用）。
type getCpIdListRequest struct {
	AvailZoneID string `json:"availZoneId,omitempty"`
}

// ListCloudPhoneIDs 调用 POST /cloud-phone/getCpIdList 查询云手机全部编号。
// availZoneID 可空（查全部）。注意文档里 availZoneId 是字符串而非整数。
func (c *Client) ListCloudPhoneIDs(ctx context.Context, availZoneID string) ([]string, error) {
	const path = "/open/api/vendor/v1/cloud-phone/getCpIdList"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, getCpIdListRequest{AvailZoneID: availZoneID})
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CPStatus 是云手机状态枚举条目。
type CPStatus struct {
	Code string `json:"code"`
	Desc string `json:"desc"`
}

// ListCPStatuses 调用 GET /cloud-phone/getStatusList 查询云手机状态枚举。
func (c *Client) ListCPStatuses(ctx context.Context) ([]CPStatus, error) {
	const path = "/open/api/vendor/v1/cloud-phone/getStatusList"
	raw, err := c.doJSON(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []CPStatus
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// InstalledApp 是云手机已安装的单个应用条目。
type InstalledApp struct {
	ID          int64  `json:"id"`
	PackageName string `json:"packageName"`
	Version     string `json:"version"`
	MD5         string `json:"md5"`
	AppName     string `json:"appName"`
	IconPath    string `json:"iconPath"`
	FileSize    string `json:"fileSize"`
}

// CPInstalledApps 把 cp 和它已装应用列表关联起来。
type CPInstalledApps struct {
	CpID string         `json:"cpId"`
	Apps []InstalledApp `json:"apps"`
}

// cpIDsRequest 是若干 /cloud-phone/* 接口共用的简单请求体（仅 cpIds）。
type cpIDsRequest struct {
	CpIDs []string `json:"cpIds"`
}

// GetInstalledApps 调用 POST /cloud-phone/getInstalledApps 查询多台云手机已装应用。
func (c *Client) GetInstalledApps(ctx context.Context, cpIDs []string) ([]CPInstalledApps, error) {
	const path = "/open/api/vendor/v1/cloud-phone/getInstalledApps"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, cpIDsRequest{CpIDs: cpIDs})
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []CPInstalledApps
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// WebRTCAuthInfo 是 webrtc-auth 接口返回的每条认证信息。
type WebRTCAuthInfo struct {
	CpID          string `json:"cpId"`
	VmID          string `json:"vmId"`
	ZoneID        int64  `json:"zoneId"`
	PushStreamURL string `json:"pushStreamUrl"`
	SignalURL     string `json:"signalUrl"`
	AuthToken     string `json:"authToken"`
}

// AuthWebRTC 调用 POST /cloud-phone/webrtc-auth 获取 WebRTC 接入凭证。
func (c *Client) AuthWebRTC(ctx context.Context, cpIDs []string) ([]WebRTCAuthInfo, error) {
	const path = "/open/api/vendor/v1/cloud-phone/webrtc-auth"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, cpIDsRequest{CpIDs: cpIDs})
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []WebRTCAuthInfo
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SimulationInfo 是 batch-query-simulation-info 返回的单条仿真信息。
type SimulationInfo struct {
	HasConfig                bool     `json:"hasConfig"`
	CpID                     string   `json:"cpId"`
	Timezone                 string   `json:"timezone"`
	Region                   string   `json:"region"`
	Latitude                 float64  `json:"latitude"`
	Longitude                float64  `json:"longitude"`
	PhoneNumber              string   `json:"phoneNumber"`
	Brand                    string   `json:"brand"`
	PhoneModel               string   `json:"phoneModel"`
	IMEI                     string   `json:"imei"`
	BluetoothMac             string   `json:"bluetoothMac"`
	WifiMac                  string   `json:"wifiMac"`
	Remark                   string   `json:"remark"`
	BluetoothDeviceAddresses []string `json:"bluetoothDeviceAddresses"`
}

// BatchQuerySimulationInfo 调用 POST /cloud-phone/batch-query-simulation-info。
func (c *Client) BatchQuerySimulationInfo(ctx context.Context, cpIDs []string) ([]SimulationInfo, error) {
	const path = "/open/api/vendor/v1/cloud-phone/batch-query-simulation-info"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, cpIDsRequest{CpIDs: cpIDs})
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []SimulationInfo
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
