package staff

import "time"

// StaffDB 用户数据库模型
type StaffDB struct {
	ID             uint   `gorm:"primaryKey;autoIncrement"`
	Username       string `gorm:"type:varchar(50);uniqueIndex;not null"`
	Name           string `gorm:"type:varchar(100)"` // 中文姓名等展示名；空表示展示时回退为 Username
	HashedPassword string `gorm:"type:varchar(255);not null"`
	IsActive       bool   `gorm:"not null;default:true"`
	IsSuperuser    bool   `gorm:"not null;default:false"`
	Avatar         string `gorm:"type:varchar(255)"`
	// TokenVersion 令牌版本：改密/重置后递增，令该用户此前签发的所有 JWT 失效。
	TokenVersion int       `gorm:"not null;default:0"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

func (StaffDB) TableName() string { return "staff" }

// Staff 用户业务模型
type Staff struct {
	ID             int       `json:"id" example:"1"`
	Username       string    `json:"username" example:"admin"`
	Name           string    `json:"name" example:"张三"` // 为空时前端展示可用 username 代替
	HashedPassword string    `json:"-"`
	IsActive       bool      `json:"is_active" example:"true"`
	IsSuperuser    bool      `json:"is_superuser" example:"false"`
	Avatar         string    `json:"avatar" example:"https://example.com/avatar.jpg"`
	TokenVersion   int       `json:"-"` // 仅内部使用（签发/校验令牌版本）
	CreatedAt      time.Time `json:"created_at" example:"2024-01-01T00:00:00Z"`
	UpdatedAt      time.Time `json:"updated_at" example:"2024-01-01T00:00:00Z"`
}

func dbToStaff(db *StaffDB) *Staff {
	return &Staff{
		ID:             int(db.ID),
		Username:       db.Username,
		Name:           db.Name,
		HashedPassword: db.HashedPassword,
		IsActive:       db.IsActive,
		IsSuperuser:    db.IsSuperuser,
		Avatar:         db.Avatar,
		TokenVersion:   db.TokenVersion,
		CreatedAt:      db.CreatedAt,
		UpdatedAt:      db.UpdatedAt,
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Account  string `json:"account" binding:"required" example:"admin"`
	Password string `json:"password" binding:"required" example:"admin123"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Account     string `json:"account" example:"admin"`
	Token       string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	Avatar      string `json:"avatar" example:"https://example.com/avatar.jpg"`
	IsSuperuser bool   `json:"is_superuser" example:"false"`
}

// StaffForm 用户表单（创建）
type StaffForm struct {
	Username    string `json:"username" binding:"required" example:"admin"`
	Name        string `json:"name" example:"张三"`
	Password    string `json:"password" binding:"required" example:"admin123"`
	IsActive    bool   `json:"is_active" example:"true"`
	IsSuperuser bool   `json:"is_superuser" example:"false"`
	Avatar      string `json:"avatar" example:"https://example.com/avatar.jpg"`
}

// StaffCreate 创建用户请求
type StaffCreate = StaffForm

// StaffUpdate 更新用户请求（布尔字段使用指针，nil 表示不更新）
type StaffUpdate struct {
	Username    string  `json:"username" example:"admin"`
	Name        *string `json:"name,omitempty" example:"张三"`
	Password    string  `json:"password" example:"admin123"`
	IsActive    *bool   `json:"is_active,omitempty" example:"true"`
	IsSuperuser *bool   `json:"is_superuser,omitempty" example:"false"`
	Avatar      string  `json:"avatar" example:"https://example.com/avatar.jpg"`
	RoleIDs     *[]int  `json:"role_ids,omitempty"`
}

// StaffWithRoles 用户信息（含角色列表，用于列表展示）
type StaffWithRoles struct {
	ID          int         `json:"id"`
	Username    string      `json:"username"`
	Name        string      `json:"name"`
	IsActive    bool        `json:"is_active"`
	IsSuperuser bool        `json:"is_superuser"`
	Avatar      string      `json:"avatar"`
	Roles       []RoleBrief `json:"roles"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// StaffProfile 当前用户信息（/auth/me 响应，含权限）
type StaffProfile struct {
	ID          int         `json:"id"`
	Username    string      `json:"username"`
	Name        string      `json:"name"`
	IsActive    bool        `json:"is_active"`
	IsSuperuser bool        `json:"is_superuser"`
	Avatar      string      `json:"avatar"`
	Permissions []string    `json:"permissions"`
	Roles       []RoleBrief `json:"roles"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required" example:"old123"`
	NewPassword string `json:"new_password" binding:"required" example:"new123"`
}

// SSOUserInfo 小西通行证用户信息
type SSOUserInfo struct {
	WxUid    string   `json:"wxUid"`
	Name     string   `json:"name"`
	JobNum   *int     `json:"jobNum"`
	Gender   int      `json:"gender"`
	Deps     []int    `json:"deps"`
	Position string   `json:"position"`
	Region   string   `json:"region"`
	Email    string   `json:"email"`
	Avatar   string   `json:"avatar"`
	Leader   []string `json:"leader"`
	JoinDate string   `json:"joinDate"`
}

// SSOVerifyResponse 小西通行证验证响应
type SSOVerifyResponse struct {
	P *SSOUserInfo `json:"P"`
	E string       `json:"E"`
}

// SSOLoginRequest SSO登录请求
type SSOLoginRequest struct {
	Token string `json:"token" binding:"required" example:"abc123"`
}
