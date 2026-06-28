package framework

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// RegisterProbes 在引擎根路径（非 /api/v1，不经访问日志/鉴权，仅全局 SecurityHeaders/CORS）挂载
// 运维探针：存活/就绪/健康检查 + 诊断探针。供 k8s liveness/readiness、负载均衡健康检查，
// 以及人工验证异常捕获/告警链路使用。纯运维基础设施，零业务引用。
//
// ⚠️ /debug/* 默认不鉴权、对外可达（/debug/panic 每次打一条堆栈）。生产若担心被扫描器触发，
// 可在此给 /debug 组加 token/开关中间件——不影响 /livez /readyz /health 健康族契约。
func RegisterProbes(r *gin.Engine) {
	r.GET("/livez", liveness)
	r.GET("/readyz", readiness)
	// /health 是 /readyz 的兼容别名，成功响应体 {"status":"ok"} 与历史 healthCheck 字节兼容。
	r.GET("/health", readiness)

	debug := r.Group("/debug")
	{
		debug.GET("/panic", debugPanic)
		debug.GET("/status/:code", debugStatus)
	}
}

// liveness 存活探针：只表明进程在跑、能处理请求，不触碰任何外部依赖，恒 200。
// k8s livenessProbe 用它——依赖故障不应导致 Pod 被重启（那是 readiness 的职责）。
func liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// readiness 就绪探针：ping 主库，不通则 503（把本实例摘出负载均衡轮转）。
// 成功响应体 {"status":"ok"} 与历史 /health 兼容。
func readiness(c *gin.Context) {
	if DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "reason": "db not initialized"})
		return
	}
	sqlDB, err := DB.DB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "reason": "db handle error"})
		return
	}
	if err := sqlDB.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "reason": "db ping failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// debugPanic 故意 panic，用于验证 gin Recovery / 异常监控 / 堆栈采集是否生效。
func debugPanic(c *gin.Context) {
	panic("debug: 故意触发的 panic（用于验证异常捕获/堆栈采集）")
}

// debugStatus 返回路径中指定的任意 HTTP 状态码，用于验证告警/监控对各类状态码的响应链路。
func debugStatus(c *gin.Context) {
	code, err := strconv.Atoi(c.Param("code"))
	if err != nil || code < 100 || code > 599 {
		Fail(c, http.StatusBadRequest, "无效的状态码（须为 100~599）")
		return
	}
	c.JSON(code, gin.H{"status": code})
}
