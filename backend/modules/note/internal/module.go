package note

import (
	"manager-backend/framework"
	"manager-backend/modules/user"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type noteModule struct{}

func (m *noteModule) Name() string { return "note" }

// Init 注入数据库并装配服务（依赖注入，替代全局 DB）。
func (m *noteModule) Init(db *gorm.DB) error {
	NoteService = newService(newRepository(db))
	return nil
}

func (m *noteModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	// 「我的笔记」按属主隔离：所有接口都要求 user 登录，仅操作本人数据。
	g := router.Group("/note")
	g.Use(user.AuthMiddleware())
	{
		g.GET("/list", GetNoteList)
		g.GET("/:id", GetNote)
		g.POST("/create", CreateNote)
		g.PUT("/update/:id", UpdateNote)
		g.DELETE("/delete/:id", DeleteNote)
	}
}

func (m *noteModule) OnStart() error { return nil }
func (m *noteModule) OnStop() error  { return nil }

func init() {
	framework.GlobalModule.Register(&noteModule{})

	// 建表（幂等）：笔记表。
	framework.RegisterSetup(func(db *gorm.DB) error {
		return db.AutoMigrate(&Note{})
	})
}
