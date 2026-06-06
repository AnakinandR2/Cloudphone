package main

// 各文件模板。{{bq}} 渲染为反引号（用于结构体 tag），{{.Xxx}} 为占位字段。

const facadeTmpl = `// Package {{.Name}} 是 {{.Name}} 业务模块的公开入口。
//
// 模块实现位于 modules/{{.Name}}/internal，受 Go internal 机制保护；公开包仅用于在 main 中
// blank-import 以触发自注册。如需对外发布契约（供其他模块依赖），在此再导出。
package {{.Name}}

import _ "manager-backend/modules/{{.Name}}/internal"
`

const modelTmpl = `package {{.Name}}

import "time"

// {{.Title}} 实体（持久化 + 领域模型，按需拆分）。
type {{.Title}} struct {
	ID        uint      {{bq}}gorm:"primaryKey;autoIncrement" json:"id"{{bq}}
	Name      string    {{bq}}gorm:"type:varchar(100);not null" json:"name"{{bq}}
	Remark    string    {{bq}}gorm:"type:text" json:"remark"{{bq}}
	CreatedAt time.Time {{bq}}gorm:"autoCreateTime" json:"created_at"{{bq}}
	UpdatedAt time.Time {{bq}}gorm:"autoUpdateTime" json:"updated_at"{{bq}}
}

func ({{.Title}}) TableName() string { return "{{.Table}}" }

// {{.Title}}Create 创建请求
type {{.Title}}Create struct {
	Name   string {{bq}}json:"name" binding:"required" example:"名称"{{bq}}
	Remark string {{bq}}json:"remark" example:"备注"{{bq}}
}

// {{.Title}}Update 更新请求
type {{.Title}}Update struct {
	Name   string {{bq}}json:"name" example:"新名称"{{bq}}
	Remark string {{bq}}json:"remark" example:"新备注"{{bq}}
}
`

const repoTmpl = `package {{.Name}}

import "gorm.io/gorm"

// repository 封装 {{.Name}} 模块的持久化，隔离 GORM 细节，便于 service 单测与替换实现。
type repository interface {
	count(name string) (int64, error)
	list(offset, limit int, name, orderClause string) ([]{{.Title}}, error)
	findByID(id int) (*{{.Title}}, error)
	create(item *{{.Title}}) error
	update(id int, fields map[string]interface{}) error
	delete(id int) error
}

type gormRepository struct{ db *gorm.DB }

func newRepository(db *gorm.DB) repository { return &gormRepository{db: db} }

func (r *gormRepository) baseQuery(name string) *gorm.DB {
	q := r.db.Model(&{{.Title}}{})
	if name != "" {
		q = q.Where("name LIKE ?", "%"+name+"%")
	}
	return q
}

func (r *gormRepository) count(name string) (int64, error) {
	var total int64
	err := r.baseQuery(name).Count(&total).Error
	return total, err
}

func (r *gormRepository) list(offset, limit int, name, orderClause string) ([]{{.Title}}, error) {
	var items []{{.Title}}
	err := r.baseQuery(name).Order(orderClause).Offset(offset).Limit(limit).Find(&items).Error
	return items, err
}

func (r *gormRepository) findByID(id int) (*{{.Title}}, error) {
	var item {{.Title}}
	if err := r.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *gormRepository) create(item *{{.Title}}) error { return r.db.Create(item).Error }

func (r *gormRepository) update(id int, fields map[string]interface{}) error {
	return r.db.Model(&{{.Title}}{}).Where("id = ?", id).Updates(fields).Error
}

func (r *gormRepository) delete(id int) error { return r.db.Delete(&{{.Title}}{}, id).Error }
`

const serviceTmpl = `package {{.Name}}

import (
	"manager-backend/framework/apperr"
	"manager-backend/framework/query"
)

// serviceImpl {{.Name}} 业务服务，依赖注入的 repository（不直接接触全局 DB）。
type serviceImpl struct{ repo repository }

// {{.Title}}Service 模块内服务实例，由 module.Init 注入 DB 后装配。
var {{.Title}}Service *serviceImpl

func newService(repo repository) *serviceImpl { return &serviceImpl{repo: repo} }

// orderable 列表允许排序的字段白名单（杜绝 SQL 注入）。
var orderable = map[string]bool{"id": true, "name": true, "created_at": true}

func (s *serviceImpl) GetList(page, size int, name, order, sort string) ([]{{.Title}}, int64, error) {
	total, err := s.repo.count(name)
	if err != nil {
		return nil, 0, err
	}
	orderClause := query.SafeOrder(order, sort, orderable, "id DESC")
	items, err := s.repo.list((page-1)*size, size, name, orderClause)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *serviceImpl) GetByID(id int) (*{{.Title}}, error) {
	item, err := s.repo.findByID(id)
	if err != nil {
		return nil, apperr.NotFound("数据不存在")
	}
	return item, nil
}

func (s *serviceImpl) Create(req *{{.Title}}Create) (*{{.Title}}, error) {
	item := {{.Title}}{Name: req.Name, Remark: req.Remark}
	if err := s.repo.create(&item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *serviceImpl) Update(id int, req *{{.Title}}Update) (*{{.Title}}, error) {
	if _, err := s.GetByID(id); err != nil {
		return nil, err
	}
	fields := make(map[string]interface{})
	if req.Name != "" {
		fields["name"] = req.Name
	}
	if req.Remark != "" {
		fields["remark"] = req.Remark
	}
	if len(fields) > 0 {
		if err := s.repo.update(id, fields); err != nil {
			return nil, err
		}
	}
	return s.GetByID(id)
}

func (s *serviceImpl) Delete(id int) error {
	if _, err := s.GetByID(id); err != nil {
		return err
	}
	return s.repo.delete(id)
}
`

const apiTmpl = `package {{.Name}}

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// Get{{.Title}}List 列表（分页）
// @Summary 获取{{.Name}}列表
// @Tags {{.Name}}
// @Produce json
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(10)
// @Param name query string false "名称（模糊搜索）"
// @Param order query string false "排序字段"
// @Param sort query string false "排序方式（ascending/descending）"
// @Success 200 {object} framework.Response{data=framework.PageResponse}
// @Router /{{.Name}}/list [get]
func Get{{.Title}}List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	list, total, err := {{.Title}}Service.GetList(page, size, c.Query("name"), c.Query("order"), c.Query("sort"))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// Get{{.Title}} 详情
// @Summary 获取{{.Name}}详情
// @Tags {{.Name}}
// @Produce json
// @Param id path int true "ID"
// @Success 200 {object} framework.Response{data={{.Title}}}
// @Failure 404 {object} framework.Response
// @Router /{{.Name}}/{id} [get]
func Get{{.Title}}(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	item, err := {{.Title}}Service.GetByID(id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// Create{{.Title}} 创建（需登录 + 权限）
// @Summary 创建{{.Name}}
// @Tags {{.Name}}
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body {{.Title}}Create true "数据"
// @Success 200 {object} framework.Response{data={{.Title}}}
// @Router /{{.Name}}/create [post]
func Create{{.Title}}(c *gin.Context) {
	var req {{.Title}}Create
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := {{.Title}}Service.Create(&req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// Update{{.Title}} 更新（需登录 + 权限）
// @Summary 更新{{.Name}}
// @Tags {{.Name}}
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body {{.Title}}Update true "数据"
// @Success 200 {object} framework.Response{data={{.Title}}}
// @Router /{{.Name}}/update/{id} [put]
func Update{{.Title}}(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	var req {{.Title}}Update
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := {{.Title}}Service.Update(id, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// Delete{{.Title}} 删除（需登录 + 权限）
// @Summary 删除{{.Name}}
// @Tags {{.Name}}
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Success 200 {object} framework.Response
// @Router /{{.Name}}/delete/{id} [delete]
func Delete{{.Title}}(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	if err := {{.Title}}Service.Delete(id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}
`

// moduleTmplStaff 后台风格模块：受保护接口用后台鉴权 + RBAC 权限。
const moduleTmplStaff = `package {{.Name}}

import (
	"manager-backend/framework"
	"manager-backend/modules/staff"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type {{.Name}}Module struct{}

func (m *{{.Name}}Module) Name() string { return "{{.Name}}" }

// Init 注入数据库并装配服务（依赖注入，替代全局 DB）。
func (m *{{.Name}}Module) Init(db *gorm.DB) error {
	{{.Title}}Service = newService(newRepository(db))
	return nil
}

func (m *{{.Name}}Module) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	g := router.Group("/{{.Name}}")
	{
		g.GET("/list", Get{{.Title}}List)
		g.GET("/:id", Get{{.Title}})

		authRequired := g.Group("")
		authRequired.Use(middlewareFuncs...)
		{
			// 权限标识需在 modules/staff/internal/permissions.go 登记后方可授予普通角色（超管默认放行）。
			authRequired.POST("/create", staff.PermissionMiddleware("{{.Name}}:create"), Create{{.Title}})
			authRequired.PUT("/update/:id", staff.PermissionMiddleware("{{.Name}}:edit"), Update{{.Title}})
			authRequired.DELETE("/delete/:id", staff.PermissionMiddleware("{{.Name}}:delete"), Delete{{.Title}})
		}
	}
}

func (m *{{.Name}}Module) OnStart() error { return nil }
func (m *{{.Name}}Module) OnStop() error  { return nil }

func init() {
	framework.GlobalModule.Register(&{{.Name}}Module{})

	// 建表（幂等）。迁移系统已移除，统一在启动时由 RunSetup 执行。
	framework.RegisterSetup(func(db *gorm.DB) error {
		return db.AutoMigrate(&{{.Title}}{})
	})
}
`

// moduleTmplUser 前台风格模块：受保护接口要求 user 登录（复用 user 模块发布的鉴权），无 RBAC。
const moduleTmplUser = `package {{.Name}}

import (
	"manager-backend/framework"
	"manager-backend/modules/user"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type {{.Name}}Module struct{}

func (m *{{.Name}}Module) Name() string { return "{{.Name}}" }

// Init 注入数据库并装配服务（依赖注入，替代全局 DB）。
func (m *{{.Name}}Module) Init(db *gorm.DB) error {
	{{.Title}}Service = newService(newRepository(db))
	return nil
}

func (m *{{.Name}}Module) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	// 前台「本人数据」模块：所有接口都要求 user 登录，仅操作本人数据（忽略后台 staff 中间件）。
	g := router.Group("/{{.Name}}")
	g.Use(user.AuthMiddleware())
	{
		g.GET("/list", Get{{.Title}}List)
		g.GET("/:id", Get{{.Title}})
		g.POST("/create", Create{{.Title}})
		g.PUT("/update/:id", Update{{.Title}})
		g.DELETE("/delete/:id", Delete{{.Title}})
	}
}

func (m *{{.Name}}Module) OnStart() error { return nil }
func (m *{{.Name}}Module) OnStop() error  { return nil }

func init() {
	framework.GlobalModule.Register(&{{.Name}}Module{})

	// 建表（幂等）。迁移系统已移除，统一在启动时由 RunSetup 执行。
	framework.RegisterSetup(func(db *gorm.DB) error {
		return db.AutoMigrate(&{{.Title}}{})
	})
}
`

const mainTestTmpl = `package {{.Name}}

import (
	"os"
	"testing"

	"manager-backend/framework"
)

func TestMain(m *testing.M) {
	tdb, _ := framework.SetupTestDB(m)
	if err := framework.DB.AutoMigrate(&{{.Title}}{}); err != nil {
		panic(err)
	}
	if err := (&{{.Name}}Module{}).Init(framework.DB); err != nil {
		panic(err)
	}

	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}
`

const serviceTestTmpl = `package {{.Name}}

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test{{.Title}}CRUD(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("{{.Table}}") })

	created, err := {{.Title}}Service.Create(&{{.Title}}Create{Name: "甲", Remark: "r"})
	require.NoError(t, err)
	assert.NotZero(t, created.ID)
	id := int(created.ID)

	got, err := {{.Title}}Service.GetByID(id)
	require.NoError(t, err)
	assert.Equal(t, "甲", got.Name)

	updated, err := {{.Title}}Service.Update(id, &{{.Title}}Update{Name: "乙"})
	require.NoError(t, err)
	assert.Equal(t, "乙", updated.Name)

	require.NoError(t, {{.Title}}Service.Delete(id))
	_, err = {{.Title}}Service.GetByID(id)
	assert.Error(t, err)
}

func Test{{.Title}}GetByIDNotFound(t *testing.T) {
	_, err := {{.Title}}Service.GetByID(999999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func Test{{.Title}}ListAndFilter(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("{{.Table}}") })

	for _, n := range []string{"苹果", "香蕉", "苹果汁"} {
		_, err := {{.Title}}Service.Create(&{{.Title}}Create{Name: n})
		require.NoError(t, err)
	}

	list, total, err := {{.Title}}Service.GetList(1, 10, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, list, 3)

	list, total, err = {{.Title}}Service.GetList(1, 10, "苹果", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)
}
`

// ---- 前台（user）风格：按属主隔离的 per-user CRUD ----

const modelTmplUser = `package {{.Name}}

import "time"

// {{.Title}} 前台用户的{{.Name}}（按属主隔离：每条归属一个 user）。
type {{.Title}} struct {
	ID         uint      {{bq}}gorm:"primaryKey;autoIncrement" json:"id"{{bq}}
	UserID uint      {{bq}}gorm:"not null;index:idx_{{.Name}}_user" json:"user_id"{{bq}}
	Name       string    {{bq}}gorm:"type:varchar(100);not null" json:"name"{{bq}}
	Remark     string    {{bq}}gorm:"type:text" json:"remark"{{bq}}
	CreatedAt  time.Time {{bq}}gorm:"autoCreateTime" json:"created_at"{{bq}}
	UpdatedAt  time.Time {{bq}}gorm:"autoUpdateTime" json:"updated_at"{{bq}}
}

func ({{.Title}}) TableName() string { return "{{.Table}}" }

// {{.Title}}Create 创建请求
type {{.Title}}Create struct {
	Name   string {{bq}}json:"name" binding:"required" example:"名称"{{bq}}
	Remark string {{bq}}json:"remark" example:"备注"{{bq}}
}

// {{.Title}}Update 更新请求
type {{.Title}}Update struct {
	Name   string {{bq}}json:"name" example:"新名称"{{bq}}
	Remark string {{bq}}json:"remark" example:"新备注"{{bq}}
}
`

const repoTmplUser = `package {{.Name}}

import "gorm.io/gorm"

// repository 持久化。所有读写一律以 userID 约束，从数据层杜绝越权（IDOR）。
type repository interface {
	count(userID int, name string) (int64, error)
	list(userID, offset, limit int, name, orderClause string) ([]{{.Title}}, error)
	findByID(userID, id int) (*{{.Title}}, error)
	create(item *{{.Title}}) error
	update(userID, id int, fields map[string]interface{}) error
	delete(userID, id int) error
}

type gormRepository struct{ db *gorm.DB }

func newRepository(db *gorm.DB) repository { return &gormRepository{db: db} }

// owned 限定为某 user 的数据。
func (r *gormRepository) owned(userID int) *gorm.DB {
	return r.db.Model(&{{.Title}}{}).Where("user_id = ?", userID)
}

func (r *gormRepository) count(userID int, name string) (int64, error) {
	q := r.owned(userID)
	if name != "" {
		q = q.Where("name LIKE ?", "%"+name+"%")
	}
	var total int64
	err := q.Count(&total).Error
	return total, err
}

func (r *gormRepository) list(userID, offset, limit int, name, orderClause string) ([]{{.Title}}, error) {
	q := r.owned(userID)
	if name != "" {
		q = q.Where("name LIKE ?", "%"+name+"%")
	}
	var items []{{.Title}}
	err := q.Order(orderClause).Offset(offset).Limit(limit).Find(&items).Error
	return items, err
}

func (r *gormRepository) findByID(userID, id int) (*{{.Title}}, error) {
	var item {{.Title}}
	if err := r.owned(userID).Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *gormRepository) create(item *{{.Title}}) error { return r.db.Create(item).Error }

func (r *gormRepository) update(userID, id int, fields map[string]interface{}) error {
	return r.owned(userID).Where("id = ?", id).Updates(fields).Error
}

func (r *gormRepository) delete(userID, id int) error {
	return r.owned(userID).Where("id = ?", id).Delete(&{{.Title}}{}).Error
}
`

const serviceTmplUser = `package {{.Name}}

import (
	"manager-backend/framework/apperr"
	"manager-backend/framework/query"
)

// serviceImpl 业务服务。所有方法都带 userID：只操作「当前用户自己的」数据。
type serviceImpl struct{ repo repository }

// {{.Title}}Service 模块内服务实例，由 module.Init 注入 DB 后装配。
var {{.Title}}Service *serviceImpl

func newService(repo repository) *serviceImpl { return &serviceImpl{repo: repo} }

var orderable = map[string]bool{"id": true, "name": true, "created_at": true}

func (s *serviceImpl) GetList(userID, page, size int, name, order, sort string) ([]{{.Title}}, int64, error) {
	total, err := s.repo.count(userID, name)
	if err != nil {
		return nil, 0, err
	}
	orderClause := query.SafeOrder(order, sort, orderable, "id DESC")
	items, err := s.repo.list(userID, (page-1)*size, size, name, orderClause)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *serviceImpl) GetByID(userID, id int) (*{{.Title}}, error) {
	item, err := s.repo.findByID(userID, id)
	if err != nil {
		return nil, apperr.NotFound("数据不存在")
	}
	return item, nil
}

func (s *serviceImpl) Create(userID int, req *{{.Title}}Create) (*{{.Title}}, error) {
	item := {{.Title}}{UserID: uint(userID), Name: req.Name, Remark: req.Remark}
	if err := s.repo.create(&item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *serviceImpl) Update(userID, id int, req *{{.Title}}Update) (*{{.Title}}, error) {
	if _, err := s.GetByID(userID, id); err != nil {
		return nil, err
	}
	fields := make(map[string]interface{})
	if req.Name != "" {
		fields["name"] = req.Name
	}
	if req.Remark != "" {
		fields["remark"] = req.Remark
	}
	if len(fields) > 0 {
		if err := s.repo.update(userID, id, fields); err != nil {
			return nil, err
		}
	}
	return s.GetByID(userID, id)
}

func (s *serviceImpl) Delete(userID, id int) error {
	if _, err := s.GetByID(userID, id); err != nil {
		return err
	}
	return s.repo.delete(userID, id)
}
`

const apiTmplUser = `package {{.Name}}

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// currentUserID 取当前登录前台用户 ID（由 user 鉴权中间件写入 context）。
func currentUserID(c *gin.Context) (int, bool) {
	v, ok := c.Get("userID")
	if !ok {
		return 0, false
	}
	id, ok := v.(int)
	return id, ok
}

// Get{{.Title}}List 我的{{.Name}}列表（仅本人）
// @Summary 我的{{.Name}}列表
// @Tags 我的{{.Name}}
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(10)
// @Param name query string false "名称（模糊搜索）"
// @Param order query string false "排序字段"
// @Param sort query string false "排序方式（ascending/descending）"
// @Success 200 {object} framework.Response{data=framework.PageResponse}
// @Router /{{.Name}}/list [get]
func Get{{.Title}}List(c *gin.Context) {
	cid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	list, total, err := {{.Title}}Service.GetList(cid, page, size, c.Query("name"), c.Query("order"), c.Query("sort"))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// Get{{.Title}} 详情（仅本人）
// @Summary {{.Name}}详情
// @Tags 我的{{.Name}}
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Success 200 {object} framework.Response{data={{.Title}}}
// @Failure 404 {object} framework.Response
// @Router /{{.Name}}/{id} [get]
func Get{{.Title}}(c *gin.Context) {
	cid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	item, err := {{.Title}}Service.GetByID(cid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// Create{{.Title}} 新建（归属当前用户）
// @Summary 新建{{.Name}}
// @Tags 我的{{.Name}}
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body {{.Title}}Create true "数据"
// @Success 200 {object} framework.Response{data={{.Title}}}
// @Router /{{.Name}}/create [post]
func Create{{.Title}}(c *gin.Context) {
	cid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req {{.Title}}Create
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := {{.Title}}Service.Create(cid, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// Update{{.Title}} 更新（仅本人）
// @Summary 更新{{.Name}}
// @Tags 我的{{.Name}}
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Param body body {{.Title}}Update true "数据"
// @Success 200 {object} framework.Response{data={{.Title}}}
// @Router /{{.Name}}/update/{id} [put]
func Update{{.Title}}(c *gin.Context) {
	cid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	var req {{.Title}}Update
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	item, err := {{.Title}}Service.Update(cid, id, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, item)
}

// Delete{{.Title}} 删除（仅本人）
// @Summary 删除{{.Name}}
// @Tags 我的{{.Name}}
// @Produce json
// @Security Bearer
// @Param id path int true "ID"
// @Success 200 {object} framework.Response
// @Router /{{.Name}}/delete/{id} [delete]
func Delete{{.Title}}(c *gin.Context) {
	cid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的ID")
		return
	}
	if err := {{.Title}}Service.Delete(cid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}
`

const serviceTestTmplUser = `package {{.Name}}

import (
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ownerA = 1001
	ownerB = 1002
)

func Test{{.Title}}CRUDOwned(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("{{.Table}}") })

	created, err := {{.Title}}Service.Create(ownerA, &{{.Title}}Create{Name: "甲", Remark: "r"})
	require.NoError(t, err)
	assert.Equal(t, uint(ownerA), created.UserID)
	id := int(created.ID)

	got, err := {{.Title}}Service.GetByID(ownerA, id)
	require.NoError(t, err)
	assert.Equal(t, "甲", got.Name)

	updated, err := {{.Title}}Service.Update(ownerA, id, &{{.Title}}Update{Name: "乙"})
	require.NoError(t, err)
	assert.Equal(t, "乙", updated.Name)

	require.NoError(t, {{.Title}}Service.Delete(ownerA, id))
	_, err = {{.Title}}Service.GetByID(ownerA, id)
	assert.Error(t, err)
}

// 核心：用户只能看/改/删自己的数据，访问他人记录一律「不存在」。
func Test{{.Title}}IsolationBetweenUsers(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("{{.Table}}") })

	a, err := {{.Title}}Service.Create(ownerA, &{{.Title}}Create{Name: "A"})
	require.NoError(t, err)
	_, err = {{.Title}}Service.Create(ownerB, &{{.Title}}Create{Name: "B"})
	require.NoError(t, err)

	listA, totalA, err := {{.Title}}Service.GetList(ownerA, 1, 10, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(1), totalA)
	require.Len(t, listA, 1)
	assert.Equal(t, "A", listA[0].Name)

	_, err = {{.Title}}Service.GetByID(ownerB, int(a.ID))
	assert.Error(t, err)
	_, err = {{.Title}}Service.Update(ownerB, int(a.ID), &{{.Title}}Update{Name: "x"})
	assert.Error(t, err)
	assert.Error(t, {{.Title}}Service.Delete(ownerB, int(a.ID)))
}
`
