// cloud_phone_app.go：应用相关操作（installApp/uninstallApp 异步，startApp/stopApp 同步）。
package midplat

import (
	"context"
	"encoding/json"
	"net/http"
)

// InstallAppRequest 是 /cloud-phone/installApp 的请求体。
type InstallAppRequest struct {
	CpIDs  []string `json:"cpIds"`
	AppIDs []int64  `json:"appIds"`
}

// AppInstallTaskInfo 描述 installApp 返回的单条任务关联（任务 ID ↔ 实例 ID）。
type AppInstallTaskInfo struct {
	TaskID     string `json:"taskId"`
	InstanceID string `json:"instanceId"`
}

// InstallAppResponse 是 installApp 异步返回。
type InstallAppResponse struct {
	TaskInfoList []AppInstallTaskInfo `json:"taskInfoList"`
}

// InstallApp 调用 POST /cloud-phone/installApp 异步安装应用。
func (c *Client) InstallApp(ctx context.Context, req InstallAppRequest) (*InstallAppResponse, error) {
	const path = "/open/api/vendor/v1/cloud-phone/installApp"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &InstallAppResponse{}, nil
	}
	var out InstallAppResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// InstallByURLApp 是 install-by-url 请求中的单个待安装应用。
// 纯下发：中台不写入应用库、不创建 app_info，downloadUrl/md5 需调用方保证可访问且一致。
type InstallByURLApp struct {
	AppName     string `json:"appName,omitempty"`
	DownloadURL string `json:"downloadUrl"`
	MD5         string `json:"md5"`
	PackageName string `json:"packageName"`
	Version     string `json:"version"`
	FileSize    string `json:"fileSize,omitempty"`
}

// InstallByURLRequest 是 /cp/apps/install-by-url 的请求体。
type InstallByURLRequest struct {
	CpIDs []string          `json:"cpIds"`
	Apps  []InstallByURLApp `json:"apps"`
}

// InstallAppByURL 调用 POST /cp/apps/install-by-url 按 URL 异步安装应用。
// 复用现有 AKSK 签名 doJSON 与 InstallAppResponse(taskInfoList)。
func (c *Client) InstallAppByURL(ctx context.Context, req InstallByURLRequest) (*InstallAppResponse, error) {
	const path = "/open/api/vendor/v1/cp/apps/install-by-url"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &InstallAppResponse{}, nil
	}
	var out InstallAppResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UninstallAppRequest 是 /cloud-phone/uninstallApp 的请求体。
// appIds 与 packageNames 至少传一个。
type UninstallAppRequest struct {
	CpIDs        []string `json:"cpIds"`
	AppIDs       []int64  `json:"appIds,omitempty"`
	PackageNames []string `json:"packageNames,omitempty"`
}

// UninstallApp 调用 POST /cloud-phone/uninstallApp 异步卸载应用。
func (c *Client) UninstallApp(ctx context.Context, req UninstallAppRequest) error {
	const path = "/open/api/vendor/v1/cloud-phone/uninstallApp"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	return err
}

// StartStopAppRequest 是 startApp / stopApp 共用的请求体。
type StartStopAppRequest struct {
	CpIDs        []string `json:"cpIds"`
	AppIDs       []int64  `json:"appIds,omitempty"`
	PackageNames []string `json:"packageNames,omitempty"`
}

// StartAppResponse 是 startApp 返回，列出启动失败的 cp 列表。
type StartAppResponse struct {
	StartFailedCpIDs []string `json:"startFailedCpIds"`
}

// StartApp 调用 POST /cloud-phone/startApp 启动应用。
func (c *Client) StartApp(ctx context.Context, req StartStopAppRequest) (*StartAppResponse, error) {
	const path = "/open/api/vendor/v1/cloud-phone/startApp"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &StartAppResponse{}, nil
	}
	var out StartAppResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// StopAppResponse 是 stopApp 返回，列出停止失败的 cp 列表。
type StopAppResponse struct {
	StopFailedCpIDs []string `json:"stopFailedCpIds"`
}

// StopApp 调用 POST /cloud-phone/stopApp 停止应用。
func (c *Client) StopApp(ctx context.Context, req StartStopAppRequest) (*StopAppResponse, error) {
	const path = "/open/api/vendor/v1/cloud-phone/stopApp"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &StopAppResponse{}, nil
	}
	var out StopAppResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
