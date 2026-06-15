package midplat

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Config 是创建 Client 所需的配置。
type Config struct {
	BaseURL   string
	AccessKey string
	SecretKey string
	// TenantUID 是中台「第 4 个鉴权 header」X-Tenant-UID 的值：vendor-service 用它解租户，
	// 不是从 AKSK 反推。只读接口可空，但**创建等写接口缺它会在中台侧 NPE（"创建失败: null"）**。
	TenantUID    string
	OperatorName string // 可选：X-Operating-Name（操作人，审计用）
	HTTPTimeout  time.Duration
}

// Client 是调用云手机中台接口的客户端。
type Client struct {
	cfg        Config
	httpClient *http.Client
}

// New 根据配置创建一个 Client，BaseURL/AccessKey/SecretKey 必填。
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("midplat: BaseURL 必填")
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("midplat: AccessKey/SecretKey 必填")
	}
	if cfg.HTTPTimeout == 0 {
		cfg.HTTPTimeout = 30 * time.Second
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.HTTPTimeout},
	}, nil
}

// SetHTTPClient 允许调用方（通常是测试）注入自定义的 *http.Client。
func (c *Client) SetHTTPClient(h *http.Client) { c.httpClient = h }

// BaseURL 返回配置中的服务地址，主要供测试使用。
func (c *Client) BaseURL() string { return c.cfg.BaseURL }

// sign 按中台约定生成 AKSK 签名。
// 待签字符串：AccessKey \n METHOD \n /URI \n QueryString \n \n Timestamp
func (c *Client) sign(method, uri, query, ts string) string {
	s := c.cfg.AccessKey + "\n" +
		strings.ToUpper(method) + "\n" +
		"/" + strings.TrimLeft(uri, "/") + "\n" +
		query + "\n" +
		"\n" +
		ts
	mac := hmac.New(sha256.New, []byte(c.cfg.SecretKey))
	mac.Write([]byte(s))
	return hex.EncodeToString(mac.Sum(nil))
}

// setAuthHeaders 把鉴权相关 header 写入请求。
func (c *Client) setAuthHeaders(req *http.Request, sig, ts string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Access-Key", c.cfg.AccessKey)
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Signature", sig)
	// 第 4 个 header：中台据此解租户（写接口必需，缺它创建会 NPE）。
	if c.cfg.TenantUID != "" {
		req.Header.Set("X-Tenant-UID", c.cfg.TenantUID)
	}
	if c.cfg.OperatorName != "" {
		req.Header.Set("X-Operating-Name", c.cfg.OperatorName)
	}
}

// midplatDebugEnabled 中台调用日志开关：MIDPLAT_DEBUG=1 即开启（打印请求与响应详情）。
func midplatDebugEnabled() bool {
	return os.Getenv("MIDPLAT_DEBUG") == "1"
}

// dumpResponse 打印中台响应详情（状态码 + 原始 body，超长截断）。
func dumpResponse(method, url string, status int, body []byte) {
	const max = 4096
	s := string(body)
	if len(s) > max {
		s = s[:max] + fmt.Sprintf("…(+%d bytes)", len(body)-max)
	}
	log.Printf("\n[midplat resp] %s %s  status=%d\n  body: %s", method, url, status, s)
}

// dumpRequest 打印实际发出的请求（敏感 header 值打码），供 1:1 比对调试。
func dumpRequest(method, url string, header http.Header, body any) {
	var keys []string
	for k := range header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	fmt.Fprintf(&b, "\n[midplat dump] %s %s\n  headers:", method, url)
	for _, k := range keys {
		v := strings.Join(header[k], ",")
		switch k {
		case "X-Access-Key", "X-Signature":
			if len(v) > 10 {
				v = v[:6] + "…(" + strconv.Itoa(len(v)) + ")"
			}
		}
		fmt.Fprintf(&b, "\n    %s: %s", k, v)
	}
	bodyStr := "<nil>"
	if body != nil {
		if raw, err := json.Marshal(body); err == nil {
			bodyStr = string(raw)
		}
	}
	fmt.Fprintf(&b, "\n  body: %s", bodyStr)
	log.Print(b.String())
}

// doJSON 发送鉴权后的请求，并返回响应包络中的 data 字段原文。
func (c *Client) doJSON(ctx context.Context, method, path string, query neturl.Values, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("midplat: 序列化请求体失败: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
	}

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

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), full, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("midplat: 构造请求失败: %w", err)
	}
	c.setAuthHeaders(req, sig, ts)

	// 调试日志开关：MIDPLAT_DEBUG=1 时打印调用中台的请求详情。
	dbg := midplatDebugEnabled()
	if dbg {
		dumpRequest(req.Method, full, req.Header, body)
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

	// 开关打开时打印响应详情（状态码 + 原始 body）。
	if dbg {
		dumpResponse(method, full, resp.StatusCode, respBody)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		he := &HTTPError{Status: resp.StatusCode, Body: string(respBody)}
		// best-effort：4xx 响应体若带统一包络，抽出 code/message（如 P1-A10 DATA_NOT_EXIST），
		// 让上层能结构化判断而不必字符串匹配 Body。解析失败则保持原状。
		var env Envelope
		if json.Unmarshal(respBody, &env) == nil {
			he.Code = env.Code
			he.Message = env.Message
		}
		return nil, he
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

// isSuccessCode 判断包络中的 code 字段是否表示成功。
// 中台对成功的表达多样：空串 / "0" / "200" / "SUCCESS" 都见过。
func isSuccessCode(code string) bool {
	switch code {
	case "", "0", "200", "SUCCESS":
		return true
	}
	return false
}
