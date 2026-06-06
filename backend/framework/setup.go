package framework

import (
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
func RunSetup(db *gorm.DB) error {
	for _, fn := range globalSetups {
		if err := fn(db); err != nil {
			return err
		}
	}
	log.Printf("[setup] 已完成 %d 个模块的建表与初始数据", len(globalSetups))
	return nil
}
