package staff

import "time"

// RoleDB 角色数据库模型
type RoleDB struct {
	ID          uint      `gorm:"primaryKey;autoIncrement"`
	Name        string    `gorm:"type:varchar(50);uniqueIndex;not null"`
	Description string    `gorm:"type:varchar(200);not null;default:''"`
	IsBuiltin   bool      `gorm:"not null;default:false"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (RoleDB) TableName() string { return "roles" }

// RolePermissionDB 角色权限关联
type RolePermissionDB struct {
	ID         uint   `gorm:"primaryKey;autoIncrement"`
	RoleID     uint   `gorm:"not null;uniqueIndex:uk_role_perm;index:idx_role_id"`
	Permission string `gorm:"type:varchar(50);not null;uniqueIndex:uk_role_perm"`
}

func (RolePermissionDB) TableName() string { return "role_permissions" }

// StaffRoleDB 用户角色关联
type StaffRoleDB struct {
	ID      uint `gorm:"primaryKey;autoIncrement"`
	StaffID uint `gorm:"not null;uniqueIndex:uk_staff_role;index:idx_staff_id"`
	// 索引名独立于 role_permissions.idx_role_id：MySQL 索引按表隔离无影响，
	// 但 SQLite 索引名全局唯一，重名会导致建表失败。
	RoleID uint `gorm:"not null;uniqueIndex:uk_staff_role;index:idx_ur_role_id"`
}

func (StaffRoleDB) TableName() string { return "staff_roles" }

// Role 角色业务模型（详情）
type Role struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsBuiltin   bool      `json:"is_builtin"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RoleListItem 角色列表项
type RoleListItem struct {
	ID              int       `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	IsBuiltin       bool      `json:"is_builtin"`
	PermissionCount int       `json:"permission_count"`
	StaffCount      int       `json:"user_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// RoleCreate 创建角色请求
type RoleCreate struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// RoleUpdate 更新角色请求
type RoleUpdate struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// RoleBrief 角色简要信息（用于用户列表中展示）
type RoleBrief struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
