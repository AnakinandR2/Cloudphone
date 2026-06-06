package staff

import (
	"log"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// --- 默认认证器（使用内置 Service）---

type defaultAuthenticator struct{}

func (a *defaultAuthenticator) GetStaffByID(userID int) (StaffInfo, error) {
	u, err := Service.GetStaffByID(userID)
	if err != nil {
		return nil, err
	}
	return &userInfoAdapter{user: u}, nil
}

func (a *defaultAuthenticator) IsSuperuser(userID int) (bool, error) {
	u, err := Service.GetStaffByID(userID)
	if err != nil {
		return false, err
	}
	return u.IsSuperuser, nil
}

type userInfoAdapter struct{ user *Staff }

func (u *userInfoAdapter) GetID() int          { return u.user.ID }
func (u *userInfoAdapter) GetUsername() string { return u.user.Username }
func (u *userInfoAdapter) IsSuperuser() bool   { return u.user.IsSuperuser }
func (u *userInfoAdapter) IsActive() bool      { return u.user.IsActive }

// --- 认证模块 ---

type authModule struct{}

func (m *authModule) Name() string           { return "auth" }
func (m *authModule) Init(db *gorm.DB) error { wire(db); return nil }

func (m *authModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	authGroup := router.Group("/staff/auth")
	{
		authGroup.POST("/login", Login)
		authGroup.POST("/sso-login", SSOLogin)

		authRequired := authGroup.Group("")
		authRequired.Use(middlewareFuncs...)
		{
			authRequired.GET("/me", GetCurrentStaff)
			authRequired.POST("/change-password", ChangePassword)
		}
	}
}

func (m *authModule) OnStart() error {
	InitSSOService()
	return nil
}

func (m *authModule) OnStop() error { return nil }

// --- 用户管理模块 ---

type staffModule struct{}

func (m *staffModule) Name() string           { return "staff" }
func (m *staffModule) Init(db *gorm.DB) error { wire(db); return nil }

func (m *staffModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	userGroup := router.Group("/staff")
	userGroup.Use(middlewareFuncs...)
	{
		userGroup.GET("/list", PermissionMiddleware("staff:view"), GetStaffList)
		userGroup.GET("/:id", PermissionMiddleware("staff:view"), GetStaff)
		userGroup.POST("/create", PermissionMiddleware("staff:create"), CreateStaff)
		userGroup.PUT("/update/:id", PermissionMiddleware("staff:edit"), UpdateStaff)
		userGroup.DELETE("/delete/:id", PermissionMiddleware("staff:delete"), DeleteStaff)
	}
}

func (m *staffModule) OnStart() error { return nil }
func (m *staffModule) OnStop() error  { return nil }

// --- 角色管理模块 ---

type roleModule struct{}

func (m *roleModule) Name() string           { return "role" }
func (m *roleModule) Init(db *gorm.DB) error { wire(db); return nil }

func (m *roleModule) RegisterRoutes(router *gin.RouterGroup, middlewareFuncs ...gin.HandlerFunc) {
	roleGroup := router.Group("/role")
	roleGroup.Use(middlewareFuncs...)
	{
		roleGroup.GET("/permissions", GetPermissions)
		roleGroup.GET("/list", PermissionMiddleware("role:view"), GetRoleList)
		roleGroup.GET("/:id", PermissionMiddleware("role:view"), GetRole)
		roleGroup.POST("/create", PermissionMiddleware("role:create"), CreateRole)
		roleGroup.PUT("/update/:id", PermissionMiddleware("role:edit"), UpdateRole)
		roleGroup.DELETE("/delete/:id", PermissionMiddleware("role:delete"), DeleteRole)
	}
}

func (m *roleModule) OnStart() error { return nil }
func (m *roleModule) OnStop() error  { return nil }

// seedDefaultUsers 创建内置管理员和测试用户
func seedDefaultUsers(db *gorm.DB) error {
	type seedUser struct {
		Username    string
		Password    string
		IsSuperuser bool
	}
	users := []seedUser{
		{"admin", "admin123", true},
		{"test", "test123", false},
	}
	for _, u := range users {
		var count int64
		db.Model(&StaffDB{}).Where("username = ?", u.Username).Count(&count)
		if count > 0 {
			continue
		}
		hashed, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		userDB := StaffDB{
			Username:       u.Username,
			HashedPassword: string(hashed),
			IsActive:       true,
			IsSuperuser:    u.IsSuperuser,
		}
		if err := db.Create(&userDB).Error; err != nil {
			return err
		}
		role := "普通用户"
		if u.IsSuperuser {
			role = "超管"
		}
		log.Printf("创建默认用户: %s（%s）", u.Username, role)
	}
	return nil
}

// seedReadonlyUser 创建示例只读角色和 readonly 演示用户
// 如果角色或用户已存在（手动创建），会修正属性确保符合预期
func seedReadonlyUser(db *gorm.DB) error {
	const roleName = "示例只读"
	const userName = "readonly"
	const userPass = "readonly123"
	wantPerms := []string{"example:view"}

	// 确保角色存在且属性正确
	var roleDB RoleDB
	if err := db.Where("name = ?", roleName).First(&roleDB).Error; err != nil {
		roleDB = RoleDB{Name: roleName, Description: "仅可查看示例模块", IsBuiltin: true}
		if err := db.Create(&roleDB).Error; err != nil {
			return err
		}
		log.Printf("创建内置角色: %s", roleName)
	} else if !roleDB.IsBuiltin {
		db.Model(&RoleDB{}).Where("id = ?", roleDB.ID).Update("is_builtin", true)
		log.Printf("修正角色 %s 为内置角色", roleName)
	}

	// 确保权限正确：清空后重建
	db.Where("role_id = ?", roleDB.ID).Delete(&RolePermissionDB{})
	for _, p := range wantPerms {
		db.Create(&RolePermissionDB{RoleID: roleDB.ID, Permission: p})
	}

	// 确保用户存在
	var userDB StaffDB
	if err := db.Where("username = ?", userName).First(&userDB).Error; err != nil {
		hashed, err := bcrypt.GenerateFromPassword([]byte(userPass), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		userDB = StaffDB{
			Username:       userName,
			HashedPassword: string(hashed),
			IsActive:       true,
			IsSuperuser:    false,
		}
		if err := db.Create(&userDB).Error; err != nil {
			return err
		}
		log.Printf("创建默认用户: %s", userName)
	}

	// 确保用户-角色关联存在
	var urCount int64
	db.Model(&StaffRoleDB{}).Where("staff_id = ? AND role_id = ?", userDB.ID, roleDB.ID).Count(&urCount)
	if urCount == 0 {
		db.Create(&StaffRoleDB{StaffID: userDB.ID, RoleID: roleDB.ID})
		log.Printf("关联用户 %s → 角色 %s", userName, roleName)
	}

	return nil
}

// --- 自动注册 ---

func init() {
	framework.AuthMiddlewareFunc = AuthMiddleware
	SetAuthenticator(&defaultAuthenticator{})
	framework.GlobalModule.Register(&authModule{})
	framework.GlobalModule.Register(&staffModule{})
	framework.GlobalModule.Register(&roleModule{})

	// 建表 + 初始数据（幂等）：员工表、角色权限表，内置超管/测试账号、示例只读角色。
	framework.RegisterSetup(func(db *gorm.DB) error {
		if err := db.AutoMigrate(&StaffDB{}, &RoleDB{}, &RolePermissionDB{}, &StaffRoleDB{}); err != nil {
			return err
		}
		if err := seedDefaultUsers(db); err != nil {
			return err
		}
		return seedReadonlyUser(db)
	})
}
