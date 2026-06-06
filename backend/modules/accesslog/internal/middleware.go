package accesslog

import (
	"bytes"
	"io"
	"log"
	"time"

	"manager-backend/framework"
	"manager-backend/framework/auth"

	"github.com/gin-gonic/gin"
)

// respCapture 包装 gin.ResponseWriter，旁路捕获前 max 字节响应体用于落库。
type respCapture struct {
	gin.ResponseWriter
	buf *bytes.Buffer
	max int
}

func (w *respCapture) Write(b []byte) (int, error) {
	if w.buf.Len() < w.max {
		remain := w.max - w.buf.Len()
		if len(b) <= remain {
			w.buf.Write(b)
		} else {
			w.buf.Write(b[:remain])
		}
	}
	return w.ResponseWriter.Write(b)
}

// newAccessLogMiddleware 记录请求/响应概要，异步入库（通过 logChan）。
func newAccessLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		var reqBodyBytes []byte
		if c.Request.Body != nil {
			reqBodyBytes, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(reqBodyBytes))
		}

		reqHeaders := sanitizeRequestHeaders(c.Request.Header)

		rw := &respCapture{
			ResponseWriter: c.Writer,
			buf:            &bytes.Buffer{},
			max:            maxBodyBytes + 100,
		}
		c.Writer = rw

		c.Next()

		var userID uint
		var username string
		scope := "anonymous"
		if uid, ok := c.Get("userID"); ok {
			userID = uint(uid.(int))
		}
		if uname, ok := c.Get("username"); ok {
			username = uname.(string)
		}
		if sc, ok := c.Get("scope"); ok {
			if s, _ := sc.(string); s != "" {
				scope = s
			}
		}

		// 前台（user）流量量大，默认不入库（建议单独走数仓/OLAP）；可由开关开启。
		if scope == auth.ScopeUser && !framework.AppConfig.AccessLogUserEnabled {
			return
		}

		reqBody := sanitizeBody(string(reqBodyBytes), c.ContentType())
		respBody := sanitizeBody(rw.buf.String(), "application/json")

		entry := AccessLog{
			UserID:          userID,
			Scope:           scope,
			Username:        username,
			Method:          c.Request.Method,
			Path:            c.Request.URL.RequestURI(),
			StatusCode:      rw.Status(),
			LatencyMs:       int(time.Since(start).Milliseconds()),
			ClientIP:        c.ClientIP(),
			UserAgent:       truncStr(c.Request.UserAgent(), 500),
			RequestHeaders:  reqHeaders,
			RequestBody:     truncBody(reqBody),
			ResponseHeaders: sanitizeResponseHeaders(rw.Header()),
			ResponseBody:    truncBody(respBody),
			CreatedAt:       start,
		}

		select {
		case logChan <- entry:
		default:
			log.Println("[AccessLog] channel 已满，丢弃日志")
		}
	}
}
