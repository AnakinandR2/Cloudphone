package user

import "time"

// UserDB 前台用户数据库模型（独立于后台员工 users 表）。
type UserDB struct {
	ID             uint   `gorm:"primaryKey;autoIncrement"`
	Phone          string `gorm:"type:varchar(20);uniqueIndex;not null"`
	Nickname       string `gorm:"type:varchar(100)"`
	HashedPassword string `gorm:"type:varchar(255);not null"`
	Avatar         string `gorm:"type:varchar(255)"`
	IsActive       bool   `gorm:"not null;default:true"`
	// TokenVersion 令牌版本：递增即令该用户此前签发的所有 JWT 失效（强制登出）。
	TokenVersion int       `gorm:"not null;default:0"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

func (UserDB) TableName() string { return "users" }

// User 前台用户业务模型（对外，不含密码）。
type User struct {
	ID           int       `json:"id"`
	Phone        string    `json:"phone"`
	Nickname     string    `json:"nickname"`
	Avatar       string    `json:"avatar"`
	IsActive     bool      `json:"is_active"`
	TokenVersion int       `json:"-"` // 仅内部使用（签发/校验令牌版本）
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func dbToUser(c *UserDB) *User {
	return &User{
		ID:           int(c.ID),
		Phone:        c.Phone,
		Nickname:     c.Nickname,
		Avatar:       c.Avatar,
		IsActive:     c.IsActive,
		TokenVersion: c.TokenVersion,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Phone    string `json:"phone" binding:"required" example:"13800138000"`
	Password string `json:"password" binding:"required" example:"pass123"`
	Nickname string `json:"nickname" example:"小明"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Phone    string `json:"phone" binding:"required" example:"13800138000"`
	Password string `json:"password" binding:"required" example:"pass123"`
}

// AdminStatusRequest 管理侧启用/禁用请求
type AdminStatusRequest struct {
	IsActive *bool `json:"is_active" binding:"required" example:"false"`
}

// LoginResponse 登录/注册响应
type LoginResponse struct {
	ID       int    `json:"id"`
	Phone    string `json:"phone"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Token    string `json:"token"`
}
