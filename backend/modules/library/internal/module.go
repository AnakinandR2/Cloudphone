package library

import (
	"time"

	"manager-backend/framework"
	"manager-backend/modules/billing"
	"manager-backend/modules/staff"
	"manager-backend/modules/user"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// cronInterval 每日 cron 间隔（可在测试中覆盖）。
var cronInterval = 24 * time.Hour

type libraryModule struct {
	db   *gorm.DB
	stop chan struct{}
}

func (m *libraryModule) Name() string { return "library" }

func (m *libraryModule) Init(db *gorm.DB) error {
	m.db = db
	PricingConfigService = newPricingConfigService(newPricingConfigRepository(db))
	// 注入可配 presigned TTL（来自 framework.AppConfig；缺省/未 LoadConfig 时取 0，
	// service 内部 presignGetTTL/presignPutTTL 会回退默认 15m/30m）。
	var getTTL, putTTL time.Duration
	if framework.AppConfig != nil {
		getTTL = framework.AppConfig.S3PresignGetTTL
		putTTL = framework.AppConfig.S3PresignPutTTL
	}
	Service = newService(newRepository(db), PricingConfigService, getTTL, putTTL)

	// 注册四种业务类型到 billing（依赖反转：billing 仅持有函数指针）。
	// quote 转调 Service.quoteForBilling（权威价 + meta_json）；fulfill 转调 Service.FulfillPackage。
	registerBilling()
	return nil
}

// registerBilling 把 lib_new/lib_upgrade/lib_renew/lib_downgrade 注册进 billing 统一订单。
func registerBilling() {
	quote := func(userID int, params []byte) (billing.BizQuoteResult, error) {
		total, meta, err := Service.quoteForBilling(userID, params)
		if err != nil {
			return billing.BizQuoteResult{}, err
		}
		return billing.BizQuoteResult{TotalCents: total, MetaJSON: meta}, nil
	}
	fulfill := func(tx *gorm.DB, userID int, orderID uint, metaJSON []byte) error {
		// 用 billing 支付事务的 tx 履约，使订阅写入与扣款/订单原子（fulfill 失败整单回滚）。
		return Service.withTx(tx).FulfillPackage(userID, orderID, metaJSON)
	}
	for _, bt := range []string{BizLibNew, BizLibUpgrade, BizLibRenew, BizLibDowngrade} {
		billing.RegisterBizType(bt, quote, fulfill)
	}
}

func (m *libraryModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	// 前台：素材库概览 + 套餐报价，按属主隔离，要求 user 登录。
	// 下单/支付由前端直接打 billing（biz_type=lib_*），本模块不重复封装。
	g := router.Group("/library")
	g.Use(user.AuthMiddleware())
	{
		g.GET("/overview", GetOverview)
		g.POST("/package/quote", QuotePackage)

		// 文件。
		g.POST("/upload/presign", PresignUpload)
		g.POST("/upload/confirm", ConfirmUpload)
		g.GET("/files", ListFiles)
		g.GET("/files/:id/download", DownloadFile)
		g.PUT("/files/:id", UpdateFile)
		g.DELETE("/files/:id", DeleteFile)
		g.GET("/files/:id/tags", GetFileTags)
		g.PUT("/files/:id/tags", SetFileTags)

		// 文件夹。
		g.GET("/folders", ListFolders)
		g.POST("/folders", CreateFolder)
		g.PUT("/folders/:id", UpdateFolder)
		g.POST("/folders/:id/move", MoveFolder)
		g.DELETE("/folders/:id", DeleteFolder)

		// 标签。
		g.GET("/tags", ListTags)
		g.POST("/tags", CreateTag)
		g.PUT("/tags/:id", UpdateTag)
		g.DELETE("/tags/:id", DeleteTag)
	}

	// 后台：定价配置（staff 登录 + 权限）。
	admin := router.Group("/admin/library")
	admin.Use(middlewareFuncs...)
	{
		admin.GET("/pricing", staff.PermissionMiddleware("library:view"), AdminGetPricing)
		admin.PUT("/pricing", staff.PermissionMiddleware("library:manage"), AdminSavePricing)
		// gap-7：查/调单个用户的用量与订阅。
		admin.GET("/users/:userId", staff.PermissionMiddleware("library:view"), AdminGetUser)
		admin.POST("/users/:userId/grant", staff.PermissionMiddleware("library:manage"), AdminGrantUser)
	}
}

func (m *libraryModule) OnStart() error {
	// 每日 cron：过期降级 + 残留 upload 清理。可注入间隔/时钟便于测试。
	m.stop = make(chan struct{})
	go func() {
		ticker := time.NewTicker(cronInterval)
		defer ticker.Stop()
		// 启动即跑一轮（清理服务重启前积压的过期/残留）。
		m.runDailySweeps(time.Now())
		for {
			select {
			case <-ticker.C:
				m.runDailySweeps(time.Now())
			case <-m.stop:
				return
			}
		}
	}()
	return nil
}

func (m *libraryModule) OnStop() error {
	if m.stop != nil {
		close(m.stop)
		m.stop = nil
	}
	return nil
}

func init() {
	framework.GlobalModule.Register(&libraryModule{})

	// 建表（幂等）+ 定价配置 seed（幂等）。
	framework.RegisterSetup(func(db *gorm.DB) error {
		if err := db.AutoMigrate(
			&LibraryFile{}, &LibraryFolder{}, &LibraryTag{}, &LibraryFileTag{},
			&LibrarySubscription{}, &LibraryUsage{}, &LibraryPricingConfig{},
			&LibraryBlob{},
		); err != nil {
			return err
		}
		// 全局去重后多个逻辑文件共享同一 blob 物理 key，library_files.s3_key 不再唯一。
		// 旧库可能残留唯一索引 uq_library_files_s3key（AutoMigrate 不会自动删旧索引）→ 幂等丢弃。
		if db.Migrator().HasIndex(&LibraryFile{}, "uq_library_files_s3key") {
			_ = db.Migrator().DropIndex(&LibraryFile{}, "uq_library_files_s3key")
		}
		return seedLibraryPricingConfig(db)
	})
}

// InitForTest 供其他模块的测试装配 library（建表 + 装配服务 + 注册 billing 类型）。仅测试用。
func InitForTest(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&LibraryFile{}, &LibraryFolder{}, &LibraryTag{}, &LibraryFileTag{},
		&LibrarySubscription{}, &LibraryUsage{}, &LibraryPricingConfig{},
		&LibraryBlob{},
	); err != nil {
		return err
	}
	return (&libraryModule{}).Init(db)
}
