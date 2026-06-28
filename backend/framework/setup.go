package framework

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"

	"gorm.io/gorm"
)

// SetupFunc 模块的建表与初始数据装配。
//
// 取代了过去的版本化迁移系统：不再有版本号，也不再有 schema_migrations 记录表。
// 每个模块在 init() 里用 RegisterSetup 登记一个函数，启动时按注册顺序逐个执行。
// 实现必须幂等——建表用 AutoMigrate（已存在则补列），初始数据先查后插。
type SetupFunc func(db *gorm.DB) error

var globalSetups []SetupFunc

// RegisterSetup 由各模块 init() 调用，登记其建表 + 初始数据逻辑。
func RegisterSetup(fn SetupFunc) {
	globalSetups = append(globalSetups, fn)
}

// RunSetup 启动时执行所有模块登记的建表与初始化（顺序与注册顺序一致）。
// 外层包一把「按库」的咨询锁：HA 多实例若同时进入建表，串行化避免并发 DDL 撞车。
func RunSetup(db *gorm.DB) error {
	return withSetupLock(db, func() error {
		for _, fn := range globalSetups {
			if err := fn(db); err != nil {
				return err
			}
		}
		log.Printf("[setup] 已完成 %d 个模块的建表与初始数据", len(globalSetups))
		return nil
	})
}

// withSetupLock 在执行建表/seed 前取一把按库名作用域的咨询锁，避免 HA 多实例并发 DDL。
// sqlite（单文件、无跨实例并发）与未知方言直接执行不加锁；mysql 用 GET_LOCK、postgres 用
// pg_advisory_lock。锁在同一条连接上获取并在结束时释放，故全程绑定到该连接。
func withSetupLock(db *gorm.DB, fn func() error) error {
	switch db.Dialector.Name() {
	case "mysql":
		return withMySQLSetupLock(db, fn)
	case "postgres":
		return withPostgresSetupLock(db, fn)
	default:
		// sqlite 及未知方言：不加锁直接执行。
		return fn()
	}
}

func withMySQLSetupLock(db *gorm.DB, fn func() error) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	name := setupLockName()
	var got int
	// GET_LOCK 第二参为等待秒数；超时返回 0、出错返回 NULL（Scan 到 0）。
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, ?)", name, 30).Scan(&got); err != nil {
		return fmt.Errorf("获取建表咨询锁失败: %w", err)
	}
	if got != 1 {
		return fmt.Errorf("获取建表咨询锁超时（另一实例正在建表？）: %s", name)
	}
	defer conn.ExecContext(ctx, "SELECT RELEASE_LOCK(?)", name)
	return fn()
}

func withPostgresSetupLock(db *gorm.DB, fn func() error) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	ctx := context.Background()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	key := setupLockKey()
	// pg_advisory_lock 阻塞直到拿到锁（会话级，绑定到本连接）。
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", key); err != nil {
		return fmt.Errorf("获取建表咨询锁失败: %w", err)
	}
	defer conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", key)
	return fn()
}

// lockBase 取锁作用域基准名：优先库名，缺省 "manager"。
func lockBase() string {
	if AppConfig != nil && AppConfig.DBName != "" {
		return AppConfig.DBName
	}
	return "manager"
}

// setupLockName MySQL GET_LOCK 的锁名（≤64 字节）。
func setupLockName() string {
	name := "setup_" + lockBase()
	if len(name) > 64 {
		name = name[:64]
	}
	return name
}

// setupLockKey Postgres pg_advisory_lock 的 64 位整型键（由库名 fnv64a 派生）。
func setupLockKey() int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(lockBase()))
	return int64(h.Sum64())
}
