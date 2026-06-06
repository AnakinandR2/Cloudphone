package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "manager-backend/docs"
	"manager-backend/framework"
	_ "manager-backend/modules/accesslog"
	_ "manager-backend/modules/app"
	_ "manager-backend/modules/cloudphone"
	_ "manager-backend/modules/example"
	_ "manager-backend/modules/note"
	_ "manager-backend/modules/phone"
	_ "manager-backend/modules/proxy"
	_ "manager-backend/modules/staff"
	_ "manager-backend/modules/user"
	// scaffold:module-imports
)

// @title 管理后台 API
// @version 1.0
// @description 管理后台通用模板框架 API 文档

// @host localhost:9981
// @BasePath /api/v1

// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	framework.LoadConfig()

	if err := framework.InitDB(); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer framework.CloseDB()

	if err := framework.InitS3(); err != nil {
		log.Fatalf("对象存储初始化失败: %v", err)
	}

	if err := framework.RunSetup(framework.DB); err != nil {
		log.Fatalf("数据库建表与初始化失败: %v", err)
	}

	for _, module := range framework.GlobalModule.GetAll() {
		if err := module.Init(framework.DB); err != nil {
			log.Fatalf("模块 %s 初始化失败: %v", module.Name(), err)
		}
	}

	gin.SetMode(framework.AppConfig.GinMode)
	r := gin.Default()
	// 仅在显式配置 TRUSTED_PROXIES 时收紧；为空保持 gin 默认（信任所有，开发期方便、生产不安全）。
	// c.ClientIP() 据此判断是否解析 X-Forwarded-For / X-Real-IP。
	if len(framework.AppConfig.TrustedProxies) > 0 {
		if err := r.SetTrustedProxies(framework.AppConfig.TrustedProxies); err != nil {
			log.Fatalf("SetTrustedProxies 失败: %v", err)
		}
	}
	r.Use(framework.SecurityHeaders())
	r.Use(framework.CORSMiddleware(framework.AppConfig.CORSAllowedOrigins))
	r.GET("/health", healthCheck)

	framework.SetupRouter(r)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	for _, module := range framework.GlobalModule.GetAll() {
		if err := module.OnStart(); err != nil {
			log.Printf("模块 %s 启动失败: %v", module.Name(), err)
		}
	}

	srv := &http.Server{Addr: ":" + framework.AppConfig.ServerPort, Handler: r}

	// 在独立 goroutine 中监听，主 goroutine 等待退出信号。
	go func() {
		log.Printf("服务启动在端口 %s", framework.AppConfig.ServerPort)
		log.Printf("Swagger 文档: http://localhost:%s/swagger/index.html", framework.AppConfig.ServerPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	// 等待 SIGINT / SIGTERM。
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	stop()
	log.Println("收到退出信号，开始优雅关闭…")

	// 1) 停止接收新请求并排空在途请求。
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP 优雅关闭超时: %v", err)
	}

	// 2) 逆序停止各模块（如 access_log 会在此 flush 剩余日志）。
	mods := framework.GlobalModule.GetAll()
	for i := len(mods) - 1; i >= 0; i-- {
		if err := mods[i].OnStop(); err != nil {
			log.Printf("模块 %s 停止失败: %v", mods[i].Name(), err)
		}
	}

	log.Println("已优雅关闭")
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
