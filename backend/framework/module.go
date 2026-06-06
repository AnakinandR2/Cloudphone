package framework

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Module 业务模块接口
type Module interface {
	Name() string
	Init(db *gorm.DB) error
	RegisterRoutes(router *gin.RouterGroup, middleware ...gin.HandlerFunc)
	OnStart() error
	OnStop() error
}

// ModuleRegistry 模块注册表
type ModuleRegistry struct {
	modules []Module
}

var GlobalModule *ModuleRegistry

func init() {
	GlobalModule = NewModuleRegistry()
}

func NewModuleRegistry() *ModuleRegistry {
	return &ModuleRegistry{
		modules: make([]Module, 0),
	}
}

func (r *ModuleRegistry) Register(module Module) {
	r.modules = append(r.modules, module)
}

func (r *ModuleRegistry) GetAll() []Module {
	return r.modules
}
