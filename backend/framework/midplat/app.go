package midplat

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// 应用上传方式枚举（spec §1.3）。
const (
	UploadMethodURLDownload = "URL_DOWNLOAD"
	UploadMethodLocalUpload = "LOCAL_UPLOAD"

	// DefaultUploadChunkSize 是分片上传的兜底块大小（4 MiB）。
	// 正常流程会用 §1.3 initiate 返回的 partSize；仅当服务端未给出时回退到此值。
	DefaultUploadChunkSize = 4 * 1024 * 1024
)

// 应用类型枚举（spec §1.3）。
const (
	AppTypeAPK  = "apk"
	AppTypeXAPK = "xapk"
)

// 上传任务状态枚举（spec §1.9）。
const (
	UploadStatusOSSUploading = "OSS_UPLOADING"
	UploadStatusOSSSuccess   = "OSS_SUCCESS"
	UploadStatusOSSFailed    = "OSS_FAILED"
)

// ============================================================
// §1.1 应用列表分页  POST /app/page
// ============================================================

// AppInfo 是 /app/page 返回的应用列表条目（spec §1.1 records[]）。
// AppInfo 对应中台 /app/page 返回的单条应用（字段以中台**实际返回**为准，
// 与文档 §1.1 的示例不一致：实际是 id(number)/appMd5/iconPath，而非 id(string)/iconUrl）。
type AppInfo struct {
	ID            int64  `json:"id"`
	AppGenerateID string `json:"appGenerateId"`
	AppName       string `json:"appName"`
	PackageName   string `json:"packageName"`
	Version       string `json:"version"`
	FileSize      string `json:"fileSize"` // 已格式化字符串，如 "256MB"（非字节数）
	UploadMethod  string `json:"uploadMethod"`
	AppMD5        string `json:"appMd5"`
	IconPath      string `json:"iconPath"`
	DownloadURL   string `json:"downloadUrl"`
	AppType       string `json:"appType"`
	OriginAppName string `json:"originAppName"`
	CreateTime    string `json:"createTime"`
	CreateBy      string `json:"createBy"`
}

// ListAppsRequest 是 /app/page 的查询条件（spec §1.1 请求），所有过滤字段可省略。
type ListAppsRequest struct {
	Page          int    `json:"page,omitempty"`
	PageSize      int    `json:"pageSize,omitempty"`
	AppGenerateID string `json:"appGenerateId,omitempty"` // 应用 code 模糊匹配
	AppName       string `json:"appName,omitempty"`
	PackageName   string `json:"packageName,omitempty"`
	CreateBy      string `json:"createBy,omitempty"`
}

// ListAppsResponse 是分页返回。外层 envelope.data 是一个对象，里面是 MyBatis-Plus
// 的 IPage 序列化形状：total + records。
// ListAppsResponse 对应中台 /app/page 的 data。
// ⚠️ 中台**实际**字段是 data / totalSize（文档 §1.1 写的 records / total 已过时）。
type ListAppsResponse struct {
	List      []AppInfo `json:"data"`
	TotalSize int64     `json:"totalSize"`
}

// ListApps 调用 POST /open/api/vendor/v1/app/page 查询应用列表。
func (c *Client) ListApps(ctx context.Context, req ListAppsRequest) (*ListAppsResponse, error) {
	const path = "/open/api/vendor/v1/app/page"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &ListAppsResponse{}, nil
	}
	var out ListAppsResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ============================================================
// §1.2 批量删除应用  POST /app/batch/deleteAppInfo
// ============================================================

// BatchDeleteApps 按 app_info 主键列表软删应用。
func (c *Client) BatchDeleteApps(ctx context.Context, ids []int64) error {
	const path = "/open/api/vendor/v1/app/batch/deleteAppInfo"
	_, err := c.doJSON(ctx, http.MethodPost, path, nil, map[string][]int64{"ids": ids})
	return err
}

// ============================================================
// §1.3 启动分片上传会话  POST /app/upload/initiate-app
// ============================================================

// InitiateUploadRequest 协商分片参数；contentMd5 命中时触发秒传。
type InitiateUploadRequest struct {
	FileName   string `json:"fileName"`
	FileSize   int64  `json:"fileSize"`
	ContentMD5 string `json:"contentMd5,omitempty"`
}

// UploadedPart 是断点续传里已上传分片的记录。
type UploadedPart struct {
	PartNumber   int    `json:"partNumber"`
	ETag         string `json:"etag"`
	ContentMD5   string `json:"contentMd5"`
	CompleteTime string `json:"completeTime"`
}

// UploadedAppInfo 是秒传命中时返回的已存在应用信息（spec §1.3 appInfo）。
type UploadedAppInfo struct {
	ID            int64  `json:"id"`
	AppGenerateID string `json:"appGenerateId"`
	AppName       string `json:"appName"`
	PackageName   string `json:"packageName"`
	Version       string `json:"version"`
	FileSize      string `json:"fileSize"`
	FileSizeBytes int64  `json:"fileSizeBytes"`
	AppMD5        string `json:"appMd5"`
	IconPath      string `json:"iconPath"`
	DownloadURL   string `json:"downloadUrl"`
	UploadMethod  string `json:"uploadMethod"`
	AppType       string `json:"appType"`
}

// InitiateUploadResponse 是 §1.3 的响应。UploadSuccess=true 表示秒传命中。
type InitiateUploadResponse struct {
	UploadID      *int64           `json:"uploadId"` // 秒传命中时为 null
	PartSize      int64            `json:"partSize"`
	TotalParts    int              `json:"totalParts"`
	UploadedParts []UploadedPart   `json:"uploadedParts"`
	UploadSuccess bool             `json:"uploadSuccess"`
	AppInfo       *UploadedAppInfo `json:"appInfo"`
}

// InitiateAppUpload 调用 POST /open/api/vendor/v1/app/upload/initiate-app。
func (c *Client) InitiateAppUpload(ctx context.Context, req InitiateUploadRequest) (*InitiateUploadResponse, error) {
	const path = "/open/api/vendor/v1/app/upload/initiate-app"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &InitiateUploadResponse{}, nil
	}
	var out InitiateUploadResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ============================================================
// §1.4 上传分片  POST /files/upload/part-new （multipart/form-data）
// ============================================================

// UploadPartResponse 是单个分片上传的回执。
type UploadPartResponse struct {
	UploadID      int64  `json:"uploadId"`
	PartNumber    int    `json:"partNumber"`
	ETag          string `json:"etag"`
	CompleteTime  string `json:"completeTime"`
	Success       bool   `json:"success"`
	UploadSuccess bool   `json:"uploadSuccess"` // 全部分片是否都已完成
	ErrorMessage  string `json:"errorMessage"`
}

// UploadPart 上传单个分片。uploadId/partNumber/contentMd5 走 query，二进制走 form 字段 "file"。
func (c *Client) UploadPart(ctx context.Context, uploadID int64, partNumber int, contentMD5 string, part io.Reader, fileName string) (*UploadPartResponse, error) {
	const path = "/open/api/vendor/v1/files/upload/part-new"
	q := neturl.Values{}
	q.Set("uploadId", strconv.FormatInt(uploadID, 10))
	q.Set("partNumber", strconv.Itoa(partNumber))
	q.Set("contentMd5", contentMD5)

	// 分片大小一般 ≤ partSize（默认 5 MiB），在内存里组装 multipart 可接受。
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	field, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return nil, fmt.Errorf("midplat: 创建 multipart 字段失败: %w", err)
	}
	if _, err := io.Copy(field, part); err != nil {
		return nil, fmt.Errorf("midplat: 写入分片数据失败: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("midplat: 关闭 multipart writer 失败: %w", err)
	}

	raw, err := c.doRawWithQuery(ctx, http.MethodPost, path, q, &buf, w.FormDataContentType())
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &UploadPartResponse{}, nil
	}
	var out UploadPartResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("midplat: 解析 part-new 响应失败: %w", err)
	}
	return &out, nil
}

// doRawWithQuery 是 multipart 等非 JSON body 请求专用的传输层。
// 签名规则与 doJSON 一致（query 参与签名，body 不参与）。
func (c *Client) doRawWithQuery(ctx context.Context, method, path string, query neturl.Values, body io.Reader, contentType string) (json.RawMessage, error) {
	qs := ""
	if query != nil {
		qs = query.Encode()
	}
	full := c.cfg.BaseURL + "/" + strings.TrimLeft(path, "/")
	if qs != "" {
		full += "?" + qs
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := c.sign(method, path, qs, ts)

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), full, body)
	if err != nil {
		return nil, fmt.Errorf("midplat: 构造请求失败: %w", err)
	}
	c.setAuthHeaders(req, sig, ts)
	// 覆盖 setAuthHeaders 设置的默认 application/json。
	req.Header.Set("Content-Type", contentType)

	// 调试日志开关：MIDPLAT_DEBUG=1 时打印请求（multipart/二进制 body 不展开）。
	dbg := midplatDebugEnabled()
	if dbg {
		dumpRequest(req.Method, full, req.Header, "<"+contentType+" body>")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if dbg {
			log.Printf("[midplat] %s %s 请求失败: %v", method, full, err)
		}
		return nil, fmt.Errorf("midplat: 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("midplat: 读取响应失败: %w", err)
	}
	if dbg {
		dumpResponse(req.Method, full, resp.StatusCode, respBody)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{Status: resp.StatusCode, Body: string(respBody)}
	}
	var env Envelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, fmt.Errorf("midplat: 解析响应包络失败: %w", err)
	}
	if !isSuccessCode(env.Code) {
		return nil, &APIError{Code: env.Code, Message: env.Message}
	}
	return env.Data, nil
}

// ============================================================
// §1.5 完成分片合并  POST /files/upload/complete-new
// ============================================================

// CompleteAppUpload 触发 OSS 合并，返回合并后的下载 URL。
func (c *Client) CompleteAppUpload(ctx context.Context, uploadID int64) (string, error) {
	const path = "/open/api/vendor/v1/files/upload/complete-new"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, map[string]int64{"uploadId": uploadID})
	if err != nil {
		return "", err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var url string
	if err := json.Unmarshal(raw, &url); err != nil {
		return "", fmt.Errorf("midplat: 解析 complete-new 响应失败: %w", err)
	}
	return url, nil
}

// ============================================================
// §1.6 解析已上传 APK 元信息  POST /app/getAppInfoFromFile
// ============================================================

// ParsedAppInfo 是 apksig 离线解析回填的应用元信息（spec §1.6）。
type ParsedAppInfo struct {
	ID          *int64 `json:"id"` // 解析阶段还未入库时为 null
	AppName     string `json:"appName"`
	PackageName string `json:"packageName"`
	Version     string `json:"version"`
	MD5         string `json:"md5"`
	FileSize    string `json:"fileSize"`
	IconPath    string `json:"iconPath"`
}

// GetAppInfoFromFile 调用 POST /open/api/vendor/v1/app/getAppInfoFromFile 解析 APK。
func (c *Client) GetAppInfoFromFile(ctx context.Context, uploadID int64) (*ParsedAppInfo, error) {
	const path = "/open/api/vendor/v1/app/getAppInfoFromFile"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, map[string]int64{"uploadId": uploadID})
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &ParsedAppInfo{}, nil
	}
	var out ParsedAppInfo
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ============================================================
// §1.7 同租户重名校验  POST /app/checkAppNameExists
// ============================================================

// CheckAppNameExists 返回 true 表示已存在重名应用（同租户维度）。
func (c *Client) CheckAppNameExists(ctx context.Context, appName string) (bool, error) {
	const path = "/open/api/vendor/v1/app/checkAppNameExists"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, map[string]string{"appName": appName})
	if err != nil {
		return false, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return false, nil
	}
	var exists bool
	if err := json.Unmarshal(raw, &exists); err != nil {
		return false, fmt.Errorf("midplat: 解析 checkAppNameExists 响应失败: %w", err)
	}
	return exists, nil
}

// ============================================================
// §1.8 根据已上传文件创建应用  POST /app/createFromUploadedFile
// ============================================================

// CreateFromUploadedFileRequest 是 §1.8 的请求体。
type CreateFromUploadedFileRequest struct {
	// 秒传命中时 uploadId 为 0：必须 omitempty 省略它，让中台按 MD5 定位已存在文件。
	// 若发成 "uploadId":0，中台会去找编号 0 的上传记录 → 报「上传记录不存在或文件路径为空」。
	UploadID        int64  `json:"uploadId,omitempty"`
	AppName         string `json:"appName"`
	PackageName     string `json:"packageName,omitempty"`
	Version         string `json:"version,omitempty"`
	MD5             string `json:"md5,omitempty"`
	FileSize        string `json:"fileSize,omitempty"`
	IconPath        string `json:"iconPath,omitempty"`
	OriginIconPath  string `json:"originIconPath,omitempty"`
	OriginAppName   string `json:"originAppName,omitempty"`
	AppDesc         string `json:"appDesc,omitempty"`
	CreateBy        string `json:"createBy,omitempty"`
	OpenToSubTenant int    `json:"openToSubTenant,omitempty"` // 0 否 / 1 是
}

// CreatedApp 是 §1.8 / §1.3 秒传创建后的应用元信息。
type CreatedApp struct {
	ID          int64  `json:"id"`
	AppName     string `json:"appName"`
	PackageName string `json:"packageName"`
	Version     string `json:"version"`
	MD5         string `json:"md5"`
	FileSize    string `json:"fileSize"`
	IconPath    string `json:"iconPath"`
}

// CreateAppFromUploadedFile 调用 POST /open/api/vendor/v1/app/createFromUploadedFile 落库。
func (c *Client) CreateAppFromUploadedFile(ctx context.Context, req CreateFromUploadedFileRequest) (*CreatedApp, error) {
	const path = "/open/api/vendor/v1/app/createFromUploadedFile"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, req)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return &CreatedApp{}, nil
	}
	var out CreatedApp
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ============================================================
// §1.9 查询上传任务状态  POST /app/queryUploadStatus
// ============================================================

// QueryUploadStatus 返回上传会话状态（OSS_UPLOADING / OSS_SUCCESS / OSS_FAILED）。
func (c *Client) QueryUploadStatus(ctx context.Context, uploadID int64) (string, error) {
	const path = "/open/api/vendor/v1/app/queryUploadStatus"
	raw, err := c.doJSON(ctx, http.MethodPost, path, nil, map[string]int64{"uploadId": uploadID})
	if err != nil {
		return "", err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var status string
	if err := json.Unmarshal(raw, &status); err != nil {
		return "", fmt.Errorf("midplat: 解析 queryUploadStatus 响应失败: %w", err)
	}
	return status, nil
}

// ============================================================
// 编排：从本地文件走完整上传流程（§1.3 → §1.4 → §1.5 → §1.6 → §1.8）
// ============================================================

// UploadAppOptions 控制 UploadAppFromFile 的行为，零值字段会被填默认值。
type UploadAppOptions struct {
	AppName         string // 默认 = filepath.Base(path)
	AppDesc         string
	CreateBy        string
	OpenToSubTenant int // 0 否 / 1 是
}

// UploadAppFromFile 串起完整的应用上传流程：
//  1. 计算整文件 MD5；
//  2. §1.3 initiate-app 协商分片参数（命中秒传则跳过分片）；
//  3. §1.4 part-new 按缺失分片逐片上传（每片带分片 MD5）；
//  4. §1.5 complete-new 触发合并；
//  5. §1.6 getAppInfoFromFile 解析包名/版本/图标（秒传时复用 appInfo）；
//  6. §1.8 createFromUploadedFile 落库。
//
// 返回 §1.8 的应用元信息。
func (c *Client) UploadAppFromFile(ctx context.Context, path string, opts UploadAppOptions) (*CreatedApp, error) {
	// 注意：opts.AppName 为空时不立刻回退文件名，留到拿到 APK 解析真名后再决定（见下方）。
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("midplat: 打开文件失败: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("midplat: 读取文件元数据失败: %w", err)
	}
	totalSize := info.Size()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("midplat: 计算 MD5 失败: %w", err)
	}
	fileMD5 := hex.EncodeToString(h.Sum(nil))
	fileName := filepath.Base(path)

	init, err := c.InitiateAppUpload(ctx, InitiateUploadRequest{
		FileName: fileName, FileSize: totalSize, ContentMD5: fileMD5,
	})
	if err != nil {
		return nil, fmt.Errorf("midplat: initiate-app 失败: %w", err)
	}

	var uploadID int64
	if init.UploadID != nil {
		uploadID = *init.UploadID
	}

	// 非秒传：按缺失分片上传，然后合并。
	if !init.UploadSuccess {
		if uploadID == 0 {
			return nil, fmt.Errorf("midplat: initiate-app 未返回 uploadId")
		}
		partSize := init.PartSize
		if partSize <= 0 {
			partSize = DefaultUploadChunkSize
		}
		totalParts := init.TotalParts
		if totalParts <= 0 {
			totalParts = int((totalSize + partSize - 1) / partSize)
			if totalParts == 0 {
				totalParts = 1
			}
		}
		done := map[int]bool{}
		for _, p := range init.UploadedParts {
			done[p.PartNumber] = true
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("midplat: 文件 seek 失败: %w", err)
		}
		buf := make([]byte, partSize)
		for part := 1; part <= totalParts; part++ {
			n, readErr := io.ReadFull(f, buf)
			// ErrUnexpectedEOF / EOF 仅代表最后一片不满，正常情况。
			if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
				return nil, fmt.Errorf("midplat: 读取分片 %d 失败: %w", part, readErr)
			}
			if done[part] {
				continue
			}
			sum := md5.Sum(buf[:n])
			chunkMD5 := hex.EncodeToString(sum[:])
			if _, err := c.UploadPart(ctx, uploadID, part, chunkMD5, bytes.NewReader(buf[:n]), fileName); err != nil {
				return nil, fmt.Errorf("midplat: 上传分片 %d 失败: %w", part, err)
			}
		}
		if _, err := c.CompleteAppUpload(ctx, uploadID); err != nil {
			return nil, fmt.Errorf("midplat: complete-new 失败: %w", err)
		}
	}

	// 组装创建参数：秒传命中时复用 §1.3 appInfo，否则解析 §1.6。
	create := CreateFromUploadedFileRequest{
		UploadID:        uploadID,
		AppName:         opts.AppName,
		MD5:             fileMD5,
		AppDesc:         opts.AppDesc,
		CreateBy:        opts.CreateBy,
		OpenToSubTenant: opts.OpenToSubTenant,
	}
	if init.UploadSuccess && init.AppInfo != nil {
		create.PackageName = init.AppInfo.PackageName
		create.Version = init.AppInfo.Version
		create.FileSize = init.AppInfo.FileSize
		create.IconPath = init.AppInfo.IconPath
		if create.AppName == "" {
			create.AppName = init.AppInfo.AppName // 优先用应用真名
		}
		if init.AppInfo.AppMD5 != "" {
			create.MD5 = init.AppInfo.AppMD5
		}
	} else {
		parsed, err := c.GetAppInfoFromFile(ctx, uploadID)
		if err != nil {
			return nil, fmt.Errorf("midplat: getAppInfoFromFile 失败: %w", err)
		}
		create.PackageName = parsed.PackageName
		create.Version = parsed.Version
		create.FileSize = parsed.FileSize
		create.IconPath = parsed.IconPath
		create.OriginIconPath = parsed.IconPath
		create.OriginAppName = parsed.AppName
		if create.AppName == "" {
			create.AppName = parsed.AppName // 优先用解析出的应用真名（而非文件名）
		}
		if parsed.MD5 != "" {
			create.MD5 = parsed.MD5
		}
	}

	// 解析仍拿不到名字时兜底用文件名。
	if create.AppName == "" {
		create.AppName = filepath.Base(path)
	}

	return c.CreateAppFromUploadedFile(ctx, create)
}
