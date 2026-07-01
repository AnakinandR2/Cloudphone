package phone

import (
	"context"
	"time"

	"manager-backend/framework"
	"manager-backend/modules/billing"
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
	repo := newRepository(db)
	PhoneService = newService(repo, newMidplatPort())
	// 向 billing 注册实例展示信息提供者（依赖反转：billing 仅持函数指针，不依赖 phone）。
	billing.SetInstanceMetaProvider(func(cpIDs []string) map[string]billing.InstanceMeta {
		metas, err := repo.metaByCpIDs(cpIDs)
		if err != nil {
			return nil
		}
		out := make(map[string]billing.InstanceMeta, len(metas))
		for cp, m := range metas {
			out[cp] = billing.InstanceMeta{Name: m.Name, Status: m.Status}
		}
		return out
	})
	// 注册「运行中实例计数」提供者：billing 概览算包月名额「在用」用（依赖反转）。
	billing.SetRunningInstanceCountProvider(func(userID int) int {
		n, err := repo.runningSessionCountByUser(userID)
		if err != nil {
			return 0
		}
		return int(n)
	})
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
		// 回收站（静态路径需在 /:id 之前声明，避免被当作 id）
		g.GET("/recycle-bin", RecycleBinList)
		g.POST("/recycle-bin/:id/restore", RecycleBinRestore)
		g.GET("/:id", GetCloudPhone)
		g.POST("/create", CreateCloudPhone)
		g.PUT("/update/:id", UpdateCloudPhone)
		g.DELETE("/delete/:id", DeleteCloudPhone)

		// 操作类（透传云手机中台，需本人拥有且已开通 cpId）
		g.POST("/:id/power", PowerCloudPhone)                  // 开机/关机
		g.POST("/:id/restart", RestartCloudPhone)              // 重启
		g.POST("/:id/reset", ResetCloudPhone)                  // 重置
		g.POST("/:id/new-device", NewDeviceCloudPhone)         // 一键新机
		g.POST("/:id/destroy", DestroyCloudPhone)              // 销毁（并移除本地档案）
		g.POST("/:id/webrtc-auth", WebRTCAuthCloudPhone)       // 远程控制：申请 WebRTC 凭证
		g.GET("/:id/webrtc-state", WebRTCStateCloudPhone)      // 远程控制：查询串流状态
		g.POST("/:id/volume", VolumeCloudPhone)                // 音量
		g.POST("/:id/rotate", RotateCloudPhone)                // 屏幕旋转
		g.POST("/:id/shake", ShakeCloudPhone)                  // 摇一摇
		g.POST("/:id/files/list", FileListCloudPhone)          // 文件管理：列目录
		g.POST("/:id/files/download", FileDownloadCloudPhone)  // 文件管理：下载
		g.POST("/:id/files/delete", FileDeleteCloudPhone)      // 文件管理：删除
		g.POST("/:id/files/upload", FileUploadCloudPhone)      // 文件管理：上传
		g.POST("/files/push-from-library", PushFromLibrary)    // 文件管理：从素材库推送（远控/群控，target ids 来自 body）
		g.GET("/:id/apps", InstalledAppsCloudPhone)            // 已装应用
		g.POST("/apps/install-by-url", InstallByURLCloudPhone) // 按 URL 安装（自有 S3，多台批量，§7.3）
		g.POST("/:id/apps/uninstall", UninstallAppCloudPhone)
		g.POST("/:id/apps/start", StartAppCloudPhone)
		g.POST("/:id/apps/stop", StopAppCloudPhone)
		g.POST("/:id/apps/kill-all", KillAllAppsCloudPhone)
		g.GET("/:id/run-logs", RunLogsCloudPhone)                     // 运行日志（分页，§2.9）
		g.GET("/:id/runtime", RuntimeCloudPhone)                      // 远程控制：真实开机时长
		g.GET("/:id/adb", AdbInfoCloudPhone)                          // ADB：连接信息
		g.POST("/:id/adb/enable", EnableAdbCloudPhone)                // ADB：开启 / 续期
		g.POST("/:id/adb/disable", DisableAdbCloudPhone)              // ADB：关闭
		g.POST("/:id/root", RootCloudPhone)                           // Root：开启 / 关闭（§3.4.1）
		g.POST("/:id/script/hello", RunHelloScriptCloudPhone)         // 自动化：下发示例脚本（§7）
		g.GET("/:id/script/task/:taskId", ScriptTaskStatusCloudPhone) // 自动化：查任务状态/报告
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

// meterRunner 运行日志同步 + 时长费结算 runner（1 分钟）。
var meterRunner *framework.PeriodicRunner

// guardRunner 准实时护栏 runner（30 秒）：余额/时长不足时关停超额运行中实例。
var guardRunner *framework.PeriodicRunner

// reconcileRunner 席位池巡检 runner（5 分钟）：席位过期/热迁移/溢出回收的兜底。
var reconcileRunner *framework.PeriodicRunner

// recycleCleanupRunner 回收站清理 runner（每日）：回收超保留天数的实例销毁 + 硬删本地记录。
var recycleCleanupRunner *framework.PeriodicRunner

func (m *phoneModule) OnStart() error {
	if PhoneService != nil && PhoneService.ops != nil {
		phoneWorker = newTaskWorker(PhoneService, m.db)
		phoneWorker.start()

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

		// 席位池巡检：每 5 分钟兜底 reconcile（席位过期/热迁移/溢出回收）。
		reconcileRunner = framework.NewPeriodicRunner(m.db, "phone:seat-reconcile", 5*time.Minute, 4*time.Minute, func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
			defer cancel()
			PhoneService.runReconcilePatrol(ctx)
			return nil
		})
		reconcileRunner.Start()

		// 回收站清理：每日销毁超保留天数的回收实例并硬删本地记录。
		recycleCleanupRunner = framework.NewPeriodicRunner(m.db, "phone:recycle-cleanup", 24*time.Hour, 23*time.Hour, func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			PhoneService.runRecycleCleanup(ctx)
			return nil
		})
		recycleCleanupRunner.Start()
	}
	return nil
}

func (m *phoneModule) OnStop() error {
	if phoneWorker != nil {
		phoneWorker.stop()
		phoneWorker = nil
	}
	if meterRunner != nil {
		meterRunner.Stop()
		meterRunner = nil
	}
	if guardRunner != nil {
		guardRunner.Stop()
		guardRunner = nil
	}
	if reconcileRunner != nil {
		reconcileRunner.Stop()
		reconcileRunner = nil
	}
	if recycleCleanupRunner != nil {
		recycleCleanupRunner.Stop()
		recycleCleanupRunner = nil
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
