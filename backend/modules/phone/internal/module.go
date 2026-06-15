package phone

import (
	"context"
	"time"

	"manager-backend/framework"
	"manager-backend/modules/staff"
	"manager-backend/modules/user"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type phoneModule struct {
	db *gorm.DB
}

func (m *phoneModule) Name() string { return "phone" }

func (m *phoneModule) Init(db *gorm.DB) error {
	m.db = db
	PhoneService = newService(newRepository(db), newMidplatPort())
	return nil
}

func (m *phoneModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	// 前台：我的云手机，按属主隔离，要求 user 登录。
	g := router.Group("/phone")
	g.Use(user.AuthMiddleware())
	{
		g.GET("/list", GetCloudPhoneList)
		g.GET("/tags", ListPhoneTags) // 已有标签去重列表
		g.POST("/tags", SetPhoneTags) // 批量设置标签
		g.GET("/:id", GetCloudPhone)
		g.POST("/create", CreateCloudPhone)
		g.PUT("/update/:id", UpdateCloudPhone)
		g.DELETE("/delete/:id", DeleteCloudPhone)

		// 操作类（透传云手机中台，需本人拥有且已开通 cpId）
		g.POST("/:id/power", PowerCloudPhone)                 // 开机/关机
		g.POST("/:id/restart", RestartCloudPhone)             // 重启
		g.POST("/:id/reset", ResetCloudPhone)                 // 重置
		g.POST("/:id/new-device", NewDeviceCloudPhone)        // 一键新机
		g.POST("/:id/destroy", DestroyCloudPhone)             // 销毁（并移除本地档案）
		g.POST("/:id/webrtc-auth", WebRTCAuthCloudPhone)      // 远程控制：申请 WebRTC 凭证
		g.GET("/:id/webrtc-state", WebRTCStateCloudPhone)     // 远程控制：查询串流状态
		g.POST("/:id/screenshot", ScreenshotCloudPhone)       // 截屏
		g.POST("/:id/volume", VolumeCloudPhone)               // 音量
		g.POST("/:id/rotate", RotateCloudPhone)               // 屏幕旋转
		g.POST("/:id/shake", ShakeCloudPhone)                 // 摇一摇
		g.POST("/:id/files/list", FileListCloudPhone)         // 文件管理：列目录
		g.POST("/:id/files/download", FileDownloadCloudPhone) // 文件管理：下载
		g.POST("/:id/files/delete", FileDeleteCloudPhone)     // 文件管理：删除
		g.POST("/:id/files/upload", FileUploadCloudPhone)     // 文件管理：上传
		g.GET("/:id/apps", InstalledAppsCloudPhone)           // 已装应用
		g.POST("/:id/apps/install", InstallAppCloudPhone)
		g.POST("/:id/apps/uninstall", UninstallAppCloudPhone)
		g.POST("/:id/apps/start", StartAppCloudPhone)
		g.POST("/:id/apps/stop", StopAppCloudPhone)
		g.POST("/:id/apps/kill-all", KillAllAppsCloudPhone)
		g.GET("/:id/run-logs", RunLogsCloudPhone)        // 运行日志（分页，§2.9）
		g.GET("/:id/adb", AdbInfoCloudPhone)             // ADB：连接信息
		g.POST("/:id/adb/enable", EnableAdbCloudPhone)   // ADB：开启 / 续期
		g.POST("/:id/adb/disable", DisableAdbCloudPhone) // ADB：关闭
		g.POST("/:id/root", RootCloudPhone)              // Root：开启 / 关闭（§3.4.1）
	}

	// 管理侧：实例全量查看/删除（staff 登录 + 权限）。
	admin := router.Group("/admin/phones")
	admin.Use(middlewareFuncs...)
	{
		admin.GET("/list", staff.PermissionMiddleware("phone:view"), AdminListCloudPhones)
		admin.GET("/tags", staff.PermissionMiddleware("phone:view"), AdminListPhoneTags)
		admin.GET("/:id", staff.PermissionMiddleware("phone:view"), AdminGetCloudPhone)
		admin.DELETE("/delete/:id", staff.PermissionMiddleware("phone:manage"), AdminDeleteCloudPhone)
	}
}

// phoneWorker 异步任务收敛 worker（仅中台已配置时运行）。
var phoneWorker *taskWorker

// enforceRunner 欠费执行 runner（仅中台已配置时运行）。
var enforceRunner *framework.PeriodicRunner

// meterRunner 运行日志同步 + 时长费结算 runner（1 分钟）。
var meterRunner *framework.PeriodicRunner

// guardRunner 准实时护栏 runner（30 秒）：余额/时长不足时关停超额运行中实例。
var guardRunner *framework.PeriodicRunner

func (m *phoneModule) OnStart() error {
	if PhoneService != nil && PhoneService.ops != nil {
		phoneWorker = newTaskWorker(PhoneService, m.db)
		phoneWorker.start()

		enforceRunner = framework.NewPeriodicRunner(m.db, "phone:enforcement", time.Minute, 50*time.Second, func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			PhoneService.runEnforcement(ctx)
			return nil
		})
		enforceRunner.Start()

		// 时长费：每分钟同步运行日志 + 结算已发生分钟。
		meterRunner = framework.NewPeriodicRunner(m.db, "phone:metering", time.Minute, 50*time.Second, func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			PhoneService.syncRunSessions(ctx)
			PhoneService.runSettlement(ctx)
			return nil
		})
		meterRunner.Start()

		// 准实时护栏：每 30 秒检查余额/时长，关停无覆盖的超额运行中实例。
		guardRunner = framework.NewPeriodicRunner(m.db, "phone:runtime-guard", 30*time.Second, 25*time.Second, func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			PhoneService.runRuntimeGuard(ctx)
			return nil
		})
		guardRunner.Start()
	}
	return nil
}

func (m *phoneModule) OnStop() error {
	if phoneWorker != nil {
		phoneWorker.stop()
		phoneWorker = nil
	}
	if enforceRunner != nil {
		enforceRunner.Stop()
		enforceRunner = nil
	}
	if meterRunner != nil {
		meterRunner.Stop()
		meterRunner = nil
	}
	if guardRunner != nil {
		guardRunner.Stop()
		guardRunner = nil
	}
	return nil
}

func init() {
	framework.GlobalModule.Register(&phoneModule{})

	// 建表（幂等）：云手机实例表 + 异步任务追踪表。
	framework.RegisterSetup(func(db *gorm.DB) error {
		return db.AutoMigrate(&CloudPhone{}, &CpTask{}, &RunSession{})
	})
}
