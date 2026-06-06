package midplat

import (
	"context"
	"encoding/json"
	"net/http"
)

// ============================================================================
// 镜像列表 /image/list
// ============================================================================

// ImageInfo 是 /image/list 返回的镜像条目。
// 注意：文档把 isForbid 写成 string，但实际返回是 int（0 启用 / 1 禁用），按实际类型来。
type ImageInfo struct {
	ID           int64  `json:"id"`
	ImageID      string `json:"imageId"`
	ImageName    string `json:"imageName"`
	ImageVersion string `json:"imageVersion"`
	DownloadURL  string `json:"downloadUrl"`
	IsForbid     int    `json:"isForbid"` // 0 启用 / 1 禁用
	CreateTime   string `json:"createTime"`
	UpdateTime   string `json:"updateTime"`
	CreateBy     string `json:"createBy"`
	UpdateBy     string `json:"updateBy"`
}

// ListImagesRequest 是 /image/list 的请求体。imageIds 为空时查全部。
type ListImagesRequest struct {
	ImageIDs []string `json:"imageIds,omitempty"`
}

// ListImages 调用 POST /open/api/vendor/v1/image/list 查询镜像列表。
//
// 文档示例里 data 给的是单对象，但接口名是 list。这里兼容数组与单对象两种返回形态，
// 统一返回切片。
func (c *Client) ListImages(ctx context.Context, req ListImagesRequest) ([]ImageInfo, error) {
	const path = "/open/api/vendor/v1/image/list"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if raw[0] == '[' {
		var out []ImageInfo
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
	var one ImageInfo
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, err
	}
	return []ImageInfo{one}, nil
}

// ============================================================================
// 云虚机镜像 /virtual-machine/images/by-avail-zone
// ============================================================================

// VMImage 是 /virtual-machine/images/by-avail-zone 返回的虚机镜像条目。
type VMImage struct {
	ID               int64  `json:"id"`
	ImageID          string `json:"imageId"`
	ImageName        string `json:"imageName"`
	ImageVersion     string `json:"imageVersion"`
	ImageType        int    `json:"imageType"` // 1:系统 2:自定义
	ImageTag         string `json:"imageTag"`
	DownloadURL      string `json:"downloadUrl"`
	ImageZone        string `json:"imageZone"`
	ImageSource      string `json:"imageSource"`
	ImageDescription string `json:"imageDescription"`
	Deleted          int    `json:"deleted"`  // 0:正常 1:删除
	IsForbid         int    `json:"isForbid"` // 0:启用 1:禁用
	CreateTime       string `json:"createTime"`
	UpdateTime       string `json:"updateTime"`
	CreateBy         string `json:"createBy"`
	UpdateBy         string `json:"updateBy"`
}

// listVMImagesRequest 是上面接口的请求体，仅内部使用。
type listVMImagesRequest struct {
	AvailZoneID int64 `json:"availZoneId"`
}

// ListVMImagesByAvailZone 按可用区查询云虚机支持的镜像列表。
func (c *Client) ListVMImagesByAvailZone(ctx context.Context, availZoneID int64) ([]VMImage, error) {
	const path = "/open/api/vendor/v1/virtual-machine/images/by-avail-zone"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, listVMImagesRequest{AvailZoneID: availZoneID})
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []VMImage
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ============================================================================
// 安卓版本枚举 /image/getAndroidVersionList
// ============================================================================

// AndroidVersion 是安卓版本枚举条目。
type AndroidVersion struct {
	Code string `json:"code"`
	Desc string `json:"desc"`
}

// ListAndroidVersions 调用 GET /open/api/vendor/v1/image/getAndroidVersionList。
func (c *Client) ListAndroidVersions(ctx context.Context) ([]AndroidVersion, error) {
	const path = "/open/api/vendor/v1/image/getAndroidVersionList"
	raw, err := c.doJSON(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []AndroidVersion
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ============================================================================
// 预制手机镜像 /cloud-phone/getPresetImages
// ============================================================================

// PresetPhoneImage 是预制手机镜像条目。
type PresetPhoneImage struct {
	ID           int64  `json:"id"`
	ImageID      string `json:"imageId"`
	ImageName    string `json:"imageName"`
	ImageVersion string `json:"imageVersion"`
	CreateTime   string `json:"createTime"`
}

// presetPhoneImagesRequest 是上面接口的请求体，仅内部使用。
type presetPhoneImagesRequest struct {
	AvailZoneID int64 `json:"availZoneId"`
	CpSpecID    int64 `json:"cpSpecId"`
}

// ListPresetPhoneImages 按可用区 + 云手机规格查询预制镜像列表。
func (c *Client) ListPresetPhoneImages(ctx context.Context, availZoneID, cpSpecID int64) ([]PresetPhoneImage, error) {
	const path = "/open/api/vendor/v1/cloud-phone/getPresetImages"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, presetPhoneImagesRequest{
		AvailZoneID: availZoneID,
		CpSpecID:    cpSpecID,
	})
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []PresetPhoneImage
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
