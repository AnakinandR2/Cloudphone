package accesslog

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// runWithScope 经访问日志中间件发一个请求，由 handler 在 Next 期间写入身份域。
func runWithScope(scope string) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(newAccessLogMiddleware())
	r.GET("/x", func(c *gin.Context) {
		if scope != "" {
			c.Set("scope", scope)
		}
		c.Set("userID", 1)
		c.Set("username", "u")
		c.Status(http.StatusOK)
	})
	req, _ := http.NewRequest("GET", "/x", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)
}

// 默认关闭时，user 身份域请求不入队；staff 照常入队；开启后 user 也入队。
func TestMiddlewareUserToggle(t *testing.T) {
	orig := framework.AppConfig.AccessLogUserEnabled
	t.Cleanup(func() { framework.AppConfig.AccessLogUserEnabled = orig })

	logChan = make(chan AccessLog, 10)

	framework.AppConfig.AccessLogUserEnabled = false
	runWithScope("user")
	assert.Equal(t, 0, len(logChan), "关闭时 user 请求不应写入访问日志")

	runWithScope("staff")
	assert.Equal(t, 1, len(logChan), "staff 请求始终写入")
	<-logChan // 排空

	framework.AppConfig.AccessLogUserEnabled = true
	runWithScope("user")
	assert.Equal(t, 1, len(logChan), "开启后 user 请求应写入")
}
