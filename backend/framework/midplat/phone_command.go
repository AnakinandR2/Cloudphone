// phone_command.go：云手机指令（root/文件操作/音量/旋转/摇一摇/WebRTC状态/关闭应用）。
package midplat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// root 开关
// ============================================================================

// UpdateRootRequest 是 /cloud-phone/update-root 的请求体。
// vm_id 可选；containers 是 cp 列表。
type UpdateRootRequest struct {
	VmID       string   `json:"vm_id,omitempty"`
	Containers []string `json:"containers"`
	Root       bool     `json:"root"`
}

// UpdateRoot 调用 POST /cloud-phone/update-root 切换 root 权限（仅部分云支持）。
func (c *Client) UpdateRoot(ctx context.Context, req UpdateRootRequest) error {
	const path = "/open/api/vendor/v1/cloud-phone/update-root"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	return err
}

// ============================================================================
// 文件操作（list / download / delete / upload）
// ============================================================================

// PhoneFile 是 /phone-command/file-list 返回的文件条目。
type PhoneFile struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Type       string `json:"type"` // file/directory/symlink/block/char/fifo/socket
	Permission string `json:"permission"`
	Owner      string `json:"owner"`
	Group      string `json:"group"`
	Size       int64  `json:"size"`
	Accessed   int64  `json:"accessed"`
	Modified   int64  `json:"modified"`
	Changed    int64  `json:"changed"`
	Created    int64  `json:"created"`
}

// phoneFileRequest 是 list/download/delete 共用的请求体。
type phoneFileRequest struct {
	VmID        string `json:"vm_id,omitempty"`
	ContainerID string `json:"container_id"`
	Path        string `json:"path"`
}

// ListPhoneFiles 调用 POST /phone-command/file-list 列出云手机指定目录下的文件。
func (c *Client) ListPhoneFiles(ctx context.Context, vmID, cpID, path string) ([]PhoneFile, error) {
	const apiPath = "/open/api/vendor/v1/phone-command/file-list"
	raw, err := c.doJSON(ctx, http.MethodPost, apiPath, nil, phoneFileRequest{
		VmID: vmID, ContainerID: cpID, Path: path,
	})
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []PhoneFile
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeletePhoneFile 调用 POST /phone-command/file-delete 删除云手机上的文件。
func (c *Client) DeletePhoneFile(ctx context.Context, vmID, cpID, path string) error {
	const apiPath = "/open/api/vendor/v1/phone-command/file-delete"
	_, err := c.doJSON(ctx, http.MethodPost, apiPath, nil, phoneFileRequest{
		VmID: vmID, ContainerID: cpID, Path: path,
	})
	return err
}

// DownloadPhoneFile 调用 POST /phone-command/file-download-stream 下载云手机文件。
// 该接口直接返回二进制流，不走 envelope。
func (c *Client) DownloadPhoneFile(ctx context.Context, vmID, cpID, path string) ([]byte, error) {
	const apiPath = "/open/api/vendor/v1/phone-command/file-download-stream"
	body, err := json.Marshal(phoneFileRequest{VmID: vmID, ContainerID: cpID, Path: path})
	if err != nil {
		return nil, fmt.Errorf("midplat: 序列化请求体失败: %w", err)
	}

	full := c.cfg.BaseURL + apiPath
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := c.sign(http.MethodPost, apiPath, "", ts)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, full, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("midplat: 构造请求失败: %w", err)
	}
	c.setAuthHeaders(req, sig, ts)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("midplat: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("midplat: 读取响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{Status: resp.StatusCode, Body: string(respBody)}
	}
	// 服务器可能在出错时返回 JSON envelope，否则就是文件二进制。
	// 用 Content-Type 简单分流：application/json 视为业务错误。
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		var env Envelope
		if err := json.Unmarshal(respBody, &env); err == nil && !isSuccessCode(env.Code) {
			return nil, &APIError{Code: env.Code, Message: env.Message}
		}
	}
	return respBody, nil
}

// UploadFile 是要上传到云机的单个文件（内存中的文件名 + 字节内容）。
type UploadFile struct {
	Name string
	Data []byte
}

// BatchUploadPhoneFiles 调用 POST /phone-command/batch-upload 把一个或多个本地文件上传到云机的 folderPath。
// folderPath 为空时中台默认落在 /sdcard/Download；uniqueName=true 时重名自动改名。
//
// 注意：doc 里这个接口路径写的是 `/phone-command/batch-upload`（缺前缀），
// 这里仍按其它 /phone-command/* 一致补 `/open/api/vendor/v1/` 前缀，若服务端 404 再调整。
func (c *Client) BatchUploadPhoneFiles(ctx context.Context, vmID, cpID, folderPath string, uniqueName bool, files []UploadFile) error {
	const path = "/open/api/vendor/v1/phone-command/batch-upload"

	if midplatDebugEnabled() {
		names := make([]string, len(files))
		for i := range files {
			names[i] = files[i].Name
		}
		log.Printf("[midplat] batch-upload vmId=%q cpId=%q folderPath=%q uniqueName=%v files=%v", vmID, cpID, folderPath, uniqueName, names)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if vmID != "" {
		_ = w.WriteField("vmId", vmID)
	}
	if err := w.WriteField("containerId", cpID); err != nil {
		return fmt.Errorf("midplat: 写 containerId 字段失败: %w", err)
	}
	if folderPath != "" {
		_ = w.WriteField("folderPath", folderPath)
	}
	_ = w.WriteField("generateUniqueFileName", strconv.FormatBool(uniqueName))
	for _, f := range files {
		part, err := w.CreateFormFile("files", f.Name)
		if err != nil {
			return fmt.Errorf("midplat: 创建 multipart 字段失败: %w", err)
		}
		if _, err := part.Write(f.Data); err != nil {
			return fmt.Errorf("midplat: 写文件数据失败: %w", err)
		}
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("midplat: 关闭 multipart writer 失败: %w", err)
	}

	_, err := c.doRawWithQuery(ctx, http.MethodPost, path, nil, &buf, w.FormDataContentType())
	return err
}

// ============================================================================
// 音量 / 旋转 / 摇一摇
// ============================================================================

// UpdateVolumeRequest 是 /phone-command/update/volume 的请求体。
type UpdateVolumeRequest struct {
	VmID       string   `json:"vm_id,omitempty"`
	Containers []string `json:"containers"`
	Volume     int      `json:"volume"`
}

// UpdateVolume 调用 POST /phone-command/update/volume 更新音量。
func (c *Client) UpdateVolume(ctx context.Context, req UpdateVolumeRequest) error {
	const path = "/open/api/vendor/v1/phone-command/update/volume"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	return err
}

// ScreenRotateRequest 是 /phone-command/screen-rotate 的请求体。
type ScreenRotateRequest struct {
	VmID        string   `json:"vm_id,omitempty"`
	Containers  []string `json:"containers"`
	Orientation string   `json:"orientation"` // landscape / portrait
}

// ScreenRotate 调用 POST /phone-command/screen-rotate 切换屏幕方向。
func (c *Client) ScreenRotate(ctx context.Context, req ScreenRotateRequest) error {
	const path = "/open/api/vendor/v1/phone-command/screen-rotate"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	return err
}

// ShakeRequest 是 /phone-command/shake 的请求体。
type ShakeRequest struct {
	VmID       string   `json:"vm_id,omitempty"`
	Containers []string `json:"containers"`
}

// Shake 调用 POST /phone-command/shake 触发摇一摇。
func (c *Client) Shake(ctx context.Context, req ShakeRequest) error {
	const path = "/open/api/vendor/v1/phone-command/shake"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	return err
}

// ============================================================================
// WebRTC 状态查询 / 批量关闭所有应用
// ============================================================================

// WebRTCContainerState 描述单个云手机是否处于 WebRTC 连接中。
type WebRTCContainerState struct {
	ContainerID string `json:"container_id"`
	InWebRTC    bool   `json:"in_webrtc"`
}

type webrtcStateResponse struct {
	Containers []WebRTCContainerState `json:"containers"`
}

// QueryWebRTCState 调用 POST /phone-command/webrtc/state 查询多台云手机的 WebRTC 状态。
func (c *Client) QueryWebRTCState(ctx context.Context, cpIDs []string) ([]WebRTCContainerState, error) {
	const path = "/open/api/vendor/v1/phone-command/webrtc/state"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, struct {
		Containers []string `json:"containers"`
	}{Containers: cpIDs})
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out webrtcStateResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out.Containers, nil
}

// killAllResponse 是 /phone-command/apps/killall 返回，containers 里列的是**失败**的 cp。
type killAllResponse struct {
	Containers []string `json:"containers"`
}

// KillAllApps 调用 POST /phone-command/apps/killall 关闭多台云手机上的所有应用。
// 返回 failed 列表（如非空表示这些 cp 上 killall 没成功）。
func (c *Client) KillAllApps(ctx context.Context, cpIDs []string) ([]string, error) {
	const path = "/open/api/vendor/v1/phone-command/apps/killall"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, struct {
		Containers []string `json:"containers"`
	}{Containers: cpIDs})
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out killAllResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out.Containers, nil
}
