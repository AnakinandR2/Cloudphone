package framework

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB 初始化数据库连接，数据库不存在时自动创建。使用全局 AppConfig 配置。
func InitDB() error {
	cfg := AppConfig
	if cfg == nil {
		return fmt.Errorf("AppConfig 未初始化，请先调用 LoadConfig()")
	}

	dbType := cfg.DBType
	if dbType == "" {
		dbType = "sqlite"
	}

	if err := ensureDatabase(cfg, dbType); err != nil {
		return err
	}

	var dialector gorm.Dialector
	switch dbType {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
		dialector = mysql.Open(dsn)
	case "postgres", "postgresql":
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
			cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
		dialector = postgres.Open(dsn)
	case "sqlite":
		dialector = sqlite.Open(cfg.DBName)
	default:
		return fmt.Errorf("不支持的数据库类型: %s", dbType)
	}

	logLevel := logger.Silent
	if cfg.LogLevel != "" {
		switch strings.ToLower(cfg.LogLevel) {
		case "debug":
			logLevel = logger.Info
		case "info":
			logLevel = logger.Warn
		case "warn":
			logLevel = logger.Warn
		case "error":
			logLevel = logger.Error
		}
	}

	var err error
	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		NowFunc: func() time.Time {
			return time.Now().Local()
		},
		// 将驱动的唯一键冲突归一为 gorm.ErrDuplicatedKey，便于跨数据库判断主键冲突。
		TranslateError: true,
	})
	if err != nil {
		return fmt.Errorf("打开数据库连接失败: %v", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("获取数据库实例失败: %v", err)
	}
	if dbType == "sqlite" {
		// WAL 模式：允许并发读（多读一写），写等待最多 5 秒再报 BUSY。
		DB.Exec("PRAGMA journal_mode=WAL")
		DB.Exec("PRAGMA busy_timeout=5000")
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(time.Hour)
	} else {
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}

	log.Printf("数据库连接成功 (类型: %s, 库名: %s)", dbType, cfg.DBName)
	return nil
}

// CloseDB 关闭数据库连接
func CloseDB() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

// ensureDatabase 如果数据库不存在则自动创建（sqlite 自动创建文件，无需处理）
func ensureDatabase(cfg *Config, dbType string) error {
	switch dbType {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True",
			cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort)
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return fmt.Errorf("连接MySQL服务失败: %v", err)
		}
		defer db.Close()

		_, err = db.Exec(fmt.Sprintf(
			"CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
			cfg.DBName))
		if err != nil {
			return fmt.Errorf("创建数据库失败: %v", err)
		}
	case "postgres", "postgresql":
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable",
			cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword)
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			return fmt.Errorf("连接PostgreSQL服务失败: %v", err)
		}
		defer db.Close()

		var exists bool
		db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)", cfg.DBName).Scan(&exists)
		if !exists {
			_, err = db.Exec(fmt.Sprintf("CREATE DATABASE \"%s\" ENCODING 'UTF8'", cfg.DBName))
			if err != nil {
				return fmt.Errorf("创建数据库失败: %v", err)
			}
		}
	}
	return nil
}
