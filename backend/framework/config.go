package framework

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 应用配置
type Config struct {
	BaseURL string // 对外服务的URL

	// 数据库
	DBType     string // mysql, postgres, sqlite
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	// 服务
	ServerPort string
	GinMode    string
	LogLevel   string // debug, info, warn, error, silent

	// CORS 允许的来源白名单；为空或含 "*" 表示放行任意来源（仅建议开发环境）。
	CORSAllowedOrigins []string

	// TrustedProxies 信任的反向代理 IP / CIDR 列表（逗号分隔，从 TRUSTED_PROXIES 解析）。
	// 仅当请求 RemoteAddr 落在该列表内时，gin 才解析 X-Forwarded-For / X-Real-IP 来确定真实
	// 客户端 IP（c.ClientIP()，写入 access_logs.client_ip）。为空时回退 gin 默认：信任所有来源，
	// 任何客户端都可伪造 XFF 篡改日志 IP——仅适合本地开发，生产务必显式配置反代/网关 IP 段。
	TrustedProxies []string

	// AccessLogUserEnabled 是否把前台（user 身份域）请求写入访问日志表。
	// 默认关闭：前台流量大，建议单独走数仓/OLAP，而非塞进业务库的 access_logs。
	AccessLogUserEnabled bool

	// JWT
	JWTSecret       string
	JWTExpireHours  int
	JWTCookieName   string // 非空时登录接口会 Set-Cookie，认证中间件也会从该 Cookie 读取
	JWTCookieSecure bool   // Cookie 的 Secure 标志（HTTPS 生产环境建议 true）

	// 前台用户独立 JWT 密钥（与后台分离做纵深防御）；为空时回退到 JWTSecret，
	// 此时前后台仅靠令牌 Scope 隔离。生产建议显式配置一个不同的值。
	UserJWTSecret string

	// APIKeyEncKey 用于加密「可重复查看」的 API 密钥明文（AES-256，hex/base64 的 32 字节）。
	// 为空时由 JWTSecret 派生；生产务必显式配置，否则改 JWTSecret 会导致旧密文不可解。
	APIKeyEncKey string

	// 前台 JWT Cookie 名（独立于后台 JWTCookieName，避免互相覆盖）；
	// 非空时前台登录/注册会 Set-Cookie，鉴权中间件也会从该 Cookie 读令牌。Secure 标志复用 JWTCookieSecure。
	UserJWTCookieName string

	// 企业微信通知
	WeChatWebhookURL string

	// 小西通行证 SSO
	SSOProjectID string

	// 云手机中台（midplat SDK）：AKSK 直连中台 OpenAPI。
	// 三者齐备时 cloudphone 模块才会构造客户端；缺任一则相关接口返回未配置错误。
	MidplatBaseURL   string
	MidplatAccessKey string
	MidplatSecretKey string
	// MidplatTenantUID 是 X-Tenant-UID header 的值（中台据此解租户，非 AKSK 反推）。
	// 只读接口可空；创建等写接口缺它中台会 NPE。
	MidplatTenantUID    string
	MidplatOperatorName string

	// S3 / S3 兼容对象存储。AccessKeyID/SecretAccessKey/Bucket 齐备时框架才会构造客户端
	// （见 InitS3），缺任一则 S3 全局客户端保持 nil，相关接口返回未配置错误。
	S3Endpoint        string // 自定义端点（MinIO/OSS/COS 等）；为空走 AWS 官方端点
	S3Region          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3Bucket          string
	S3UsePathStyle    bool   // MinIO 等自建兼容服务多需 true；AWS/多数云厂商用 false
	S3PublicBaseURL   string // 公开访问/CDN 前缀；为空时由端点+桶拼接

	// 素材库专用【私有】对象存储——一套与公开 S3 完全独立的配置（可以是不同服务/厂商）。
	// 私有桶无匿名读、无匿名 list、无公开前缀。AK/SK/Bucket 配全框架才构造 framework.S3Library；
	// 缺任一则 S3Library 为 nil（素材库存储不可用，相关接口返回未配置错误）——不回退公开 S3。
	S3LibraryEndpoint        string
	S3LibraryRegion          string
	S3LibraryAccessKeyID     string
	S3LibrarySecretAccessKey string
	S3LibraryBucket          string
	S3LibraryUsePathStyle    bool
	// S3PresignGetTTL / S3PresignPutTTL 素材库 presigned 下载/上传短链有效期（Go duration）。
	// 解析失败或 <=0 回退默认（GET 15m / PUT 30m）。
	S3PresignGetTTL time.Duration
	S3PresignPutTTL time.Duration
	// S3LibraryInstallGetTTL 应用按 URL 安装时给中台下载用的 presigned GET 有效期（Go duration）。
	// 必须够中台从该 URL 下载完成，故默认放大到 1h（远长于普通浏览 15m）。
	S3LibraryInstallGetTTL time.Duration
}

var AppConfig *Config

// loadDotEnv 把工作目录下的 .env 灌进进程环境变量（已存在的不覆盖）。
// 无外部依赖；让 `go run .` 与 air（air 自带 env_files=[".env"]）行为一致，
// 否则 `go run` 读不到 MIDPLAT_* → 中台未配置 → 创建走降级假成功，极具迷惑性。
func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		// 去掉可选的成对引号
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		}
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
}

// LoadConfig 从环境变量加载配置（应在程序启动时调用一次，之后通过 AppConfig 全局访问）
func LoadConfig() *Config {
	loadDotEnv()
	cfg := &Config{
		BaseURL:    getEnv("BASE_URL", "http://localhost:9981"),
		DBType:     getEnv("DB_TYPE", "sqlite"),
		DBHost:     getEnv("DB_HOST", "127.0.0.1"),
		DBPort:     getEnvAsInt("DB_PORT", 3306),
		DBUser:     getEnv("DB_USER", "root"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		// 默认 sqlite：DBName 即数据库文件路径；切到 mysql/postgres 时为库名。
		DBName: getEnv("DB_NAME", "manage_system.db"),

		ServerPort:           getEnv("SERVER_PORT", "9981"),
		GinMode:              getEnv("GIN_MODE", "release"),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
		CORSAllowedOrigins:   getEnvAsList("CORS_ALLOWED_ORIGINS"),
		TrustedProxies:       getEnvAsList("TRUSTED_PROXIES"),
		AccessLogUserEnabled: getEnvAsBool("ACCESS_LOG_USER_ENABLED", false),

		JWTSecret:       getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpireHours:  getEnvAsInt("JWT_EXPIRE_HOURS", 15*24),
		JWTCookieName:   getEnv("JWT_COOKIE_NAME", "jwt_token"),
		JWTCookieSecure: getEnvAsBool("JWT_COOKIE_SECURE", false),

		UserJWTSecret:     getEnv("USER_JWT_SECRET", ""),
		APIKeyEncKey:      getEnv("APIKEY_ENC_KEY", ""),
		UserJWTCookieName: getEnv("USER_JWT_COOKIE_NAME", "user_token"),

		WeChatWebhookURL: getEnv("WECHAT_WEBHOOK_URL", ""),

		SSOProjectID: getEnv("SSO_PROJECT_ID", "e7srvvcy"),

		MidplatBaseURL:   getEnv("MIDPLAT_BASE_URL", ""),
		MidplatAccessKey: getEnv("MIDPLAT_ACCESS_KEY", ""),
		MidplatSecretKey: getEnv("MIDPLAT_SECRET_KEY", ""),
		MidplatTenantUID: getEnv("MIDPLAT_TENANT_UID", ""),
		// 默认非空：中台 create 等接口会读取操作人(X-Operating-Name)，缺它可能 NPE。
		// 参考官方控制台 cp-glory-service 默认 "cloudphone-demo"。
		MidplatOperatorName: getEnv("MIDPLAT_OPERATOR_NAME", "cloudphone-manager"),

		S3Endpoint:               getEnv("S3_ENDPOINT", ""),
		S3Region:                 getEnv("S3_REGION", "us-east-1"),
		S3AccessKeyID:            getEnv("S3_ACCESS_KEY_ID", ""),
		S3SecretAccessKey:        getEnv("S3_SECRET_ACCESS_KEY", ""),
		S3Bucket:                 getEnv("S3_BUCKET", ""),
		S3UsePathStyle:           getEnvAsBool("S3_USE_PATH_STYLE", false),
		S3PublicBaseURL:          getEnv("S3_PUBLIC_BASE_URL", ""),
		S3LibraryEndpoint:        getEnv("S3_LIBRARY_ENDPOINT", ""),
		S3LibraryRegion:          getEnv("S3_LIBRARY_REGION", "us-east-1"),
		S3LibraryAccessKeyID:     getEnv("S3_LIBRARY_ACCESS_KEY_ID", ""),
		S3LibrarySecretAccessKey: getEnv("S3_LIBRARY_SECRET_ACCESS_KEY", ""),
		S3LibraryBucket:          getEnv("S3_LIBRARY_BUCKET", ""),
		S3LibraryUsePathStyle:    getEnvAsBool("S3_LIBRARY_USE_PATH_STYLE", false),
		S3PresignGetTTL:          getEnvAsDuration("S3_PRESIGN_GET_TTL", 15*time.Minute),
		S3PresignPutTTL:          getEnvAsDuration("S3_PRESIGN_PUT_TTL", 30*time.Minute),
		S3LibraryInstallGetTTL:   getEnvAsDuration("S3_LIBRARY_INSTALL_GET_TTL", time.Hour),
	}

	AppConfig = cfg

	log.Printf("[config] BaseURL=%s  DBType=%s  DBHost=%s:%d  DBName=%s  DBUser=%s  DBPassword=%s",
		cfg.BaseURL, cfg.DBType, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBUser, maskSecret(cfg.DBPassword))
	log.Printf("[config] ServerPort=%s  GinMode=%s  LogLevel=%s",
		cfg.ServerPort, cfg.GinMode, cfg.LogLevel)
	log.Printf("[config] JWTSecret=%s  JWTExpireHours=%d  JWTCookieName=%s  JWTCookieSecure=%v",
		maskSecret(cfg.JWTSecret), cfg.JWTExpireHours, displayOrEmpty(cfg.JWTCookieName), cfg.JWTCookieSecure)
	log.Printf("[config] UserJWTSecret=%s  UserJWTCookieName=%s",
		maskSecret(cfg.UserJWTSecret), displayOrEmpty(cfg.UserJWTCookieName))
	log.Printf("[config] SSOProjectID=%s  WeChatWebhookURL=%s",
		displayOrEmpty(cfg.SSOProjectID), displayOrEmpty(cfg.WeChatWebhookURL))
	log.Printf("[config] CORSAllowedOrigins=%v  AccessLogUserEnabled=%v  TrustedProxies=%v",
		corsDisplay(cfg.CORSAllowedOrigins), cfg.AccessLogUserEnabled, trustedProxiesDisplay(cfg.TrustedProxies))
	log.Printf("[config] MidplatTenantUID=%s  MidplatOperatorName=%s",
		displayOrEmpty(cfg.MidplatTenantUID), displayOrEmpty(cfg.MidplatOperatorName))
	log.Printf("[config] MidplatBaseURL=%s  MidplatAccessKey=%s  MidplatSecretKey=%s",
		displayOrEmpty(cfg.MidplatBaseURL), displayOrEmpty(cfg.MidplatAccessKey), maskSecret(cfg.MidplatSecretKey))
	log.Printf("[config] S3Endpoint=%s  S3Region=%s  S3Bucket=%s  S3UsePathStyle=%v  S3PublicBaseURL=%s",
		displayOrEmpty(cfg.S3Endpoint), cfg.S3Region, displayOrEmpty(cfg.S3Bucket), cfg.S3UsePathStyle, displayOrEmpty(cfg.S3PublicBaseURL))
	log.Printf("[config] S3AccessKeyID=%s  S3SecretAccessKey=%s",
		displayOrEmpty(cfg.S3AccessKeyID), maskSecret(cfg.S3SecretAccessKey))
	log.Printf("[config] S3LibraryEndpoint=%s  S3LibraryRegion=%s  S3LibraryBucket=%s  S3LibraryUsePathStyle=%v",
		displayOrEmpty(cfg.S3LibraryEndpoint), cfg.S3LibraryRegion, displayOrEmpty(cfg.S3LibraryBucket), cfg.S3LibraryUsePathStyle)
	log.Printf("[config] S3LibraryAccessKeyID=%s  S3LibrarySecretAccessKey=%s  S3PresignGetTTL=%s  S3PresignPutTTL=%s",
		displayOrEmpty(cfg.S3LibraryAccessKeyID), maskSecret(cfg.S3LibrarySecretAccessKey), cfg.S3PresignGetTTL, cfg.S3PresignPutTTL)

	if cfg.JWTSecret == "change-me-in-production" {
		log.Println("[config] ⚠️  正在使用默认 JWT_SECRET，请在生产环境务必改为强随机值！")
	}
	if len(cfg.CORSAllowedOrigins) == 0 {
		log.Println("[config] ⚠️  CORS_ALLOWED_ORIGINS 未配置，将放行任意来源（仅建议开发环境）")
	}
	if len(cfg.TrustedProxies) == 0 {
		log.Println("[config] ⚠️  TRUSTED_PROXIES 未配置，将信任所有来源；任何客户端都可伪造 X-Forwarded-For 篡改 access_logs.client_ip（仅建议开发环境）")
	}

	return cfg
}

func corsDisplay(origins []string) string {
	if len(origins) == 0 {
		return "(任意来源/dev)"
	}
	return strings.Join(origins, ",")
}

func trustedProxiesDisplay(proxies []string) string {
	if len(proxies) == 0 {
		return "(信任所有/dev)"
	}
	return strings.Join(proxies, ",")
}

func maskSecret(s string) string {
	if s == "" {
		return "(空)"
	}
	return "******"
}

func displayOrEmpty(s string) string {
	if s == "" {
		return "(空)"
	}
	return s
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// getEnvAsList 解析逗号分隔的环境变量为去空白的字符串切片；未设置时返回 nil。
func getEnvAsList(key string) []string {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnvAsInt(key string, defaultValue int) int {
	s := os.Getenv(key)
	if s == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		log.Fatalf("环境变量 %s 转换为 int 失败: %v", key, err)
	}
	return v
}

// getEnvAsDuration 解析 Go duration 形式的环境变量（如 15m / 1h30m）。
// 未设置、解析失败或 <=0 时回退默认值（不致命，仅记一条提示，便于配错时仍可启动）。
func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		log.Printf("[config] ⚠️  环境变量 %s=%q 解析为 duration 失败，回退默认值 %s: %v", key, s, defaultValue, err)
		return defaultValue
	}
	if d <= 0 {
		log.Printf("[config] ⚠️  环境变量 %s=%q <= 0，回退默认值 %s", key, s, defaultValue)
		return defaultValue
	}
	return d
}

func getEnvAsBool(key string, defaultValue bool) bool {
	s := os.Getenv(key)
	if s == "" {
		return defaultValue
	}
	v, err := strconv.ParseBool(s)
	if err != nil {
		log.Fatalf("环境变量 %s 转换为 bool 失败: %v", key, err)
	}
	return v
}
