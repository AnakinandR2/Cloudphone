package accesslog

import (
	"sync"
	"time"

	"manager-backend/framework"
	"manager-backend/modules/staff"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// logRepo 供异步写入器与清理器使用的仓储实例，由 Init 注入。
var logRepo repository

// stopOnce 保证日志通道只被关闭一次（OnStop 可能被多次调用）。
var stopOnce sync.Once

type accessLogModule struct{}

func (m *accessLogModule) Name() string { return "access_log" }

// Init 注入数据库：装配查询服务与后台写入/清理所用的仓储。
func (m *accessLogModule) Init(db *gorm.DB) error {
	repo := newRepository(db)
	logRepo = repo
	AccessLogService = newService(repo)
	return nil
}

func (m *accessLogModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	g := router.Group("/access-log")
	g.Use(middlewareFuncs...)
	g.Use(staff.PermissionMiddleware("access_log:view"))
	{
		g.GET("/list", GetAccessLogList)
		g.GET("/:id", GetAccessLog)
	}
}

// writerDone 在 asyncWriter 退出（已 flush 完）时关闭，供 OnStop 等待。
var writerDone chan struct{}

func (m *accessLogModule) OnStart() error {
	writerDone = make(chan struct{})
	go func() {
		asyncWriter()
		close(writerDone)
	}()
	go cleanupLoop()
	return nil
}

// OnStop 关闭日志通道并等待异步写入器把剩余日志落库（最多 5s），
// 确保在 DB 关闭前完成 flush，避免优雅关闭时丢日志。
func (m *accessLogModule) OnStop() error {
	stopOnce.Do(func() {
		if logChan != nil {
			close(logChan)
		}
	})
	if writerDone != nil {
		select {
		case <-writerDone:
		case <-time.After(5 * time.Second):
		}
	}
	return nil
}

func init() {
	logChan = make(chan AccessLog, logChannelSize)
	framework.AccessLogMiddlewareFunc = newAccessLogMiddleware
	framework.GlobalModule.Register(&accessLogModule{})

	// 建表（幂等）：访问日志表。
	framework.RegisterSetup(func(db *gorm.DB) error {
		return db.AutoMigrate(&AccessLog{})
	})
}
