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
type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("midplat: http %d: %s", e.Status, e.Body)
}

// APIError 表示包络中 code 字段为非成功值时的业务错误。
type APIError struct {
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("midplat: 业务错误 code=%s msg=%s", e.Code, e.Message)
}
