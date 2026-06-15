package midplat

import (
	"encoding/json"
	"fmt"
)

// Envelope 是中台所有接口统一的响应包络。
type Envelope struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// HTTPError 表示 HTTP 状态码非 2xx 的错误。
// Code/Message 为 best-effort：4xx 响应体里若带统一包络（如 P1-A10 的
// HTTP 400 + code="DATA_NOT_EXIST"），doJSON 会顺带解析填入，便于上层结构化判断。
type HTTPError struct {
	Status  int
	Body    string
	Code    string
	Message string
}

func (e *HTTPError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("midplat: http %d code=%s: %s", e.Status, e.Code, e.Body)
	}
	return fmt.Sprintf("midplat: http %d: %s", e.Status, e.Body)
}

// IsDataNotExist 判断错误是否为中台「数据不存在」（code=DATA_NOT_EXIST）。
// 同时覆盖 4xx 的 *HTTPError 与包络业务错误 *APIError 两种形态。
func IsDataNotExist(err error) bool {
	switch e := err.(type) {
	case *HTTPError:
		return e.Code == "DATA_NOT_EXIST"
	case *APIError:
		return e.Code == "DATA_NOT_EXIST"
	}
	return false
}

// APIError 表示包络中 code 字段为非成功值时的业务错误。
type APIError struct {
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("midplat: 业务错误 code=%s msg=%s", e.Code, e.Message)
}
