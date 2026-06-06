package staff

// StaffInfo 用户信息接口
type StaffInfo interface {
	GetID() int
	GetUsername() string
	IsSuperuser() bool
	IsActive() bool
}

// Authenticator 认证器接口
type Authenticator interface {
	GetStaffByID(userID int) (StaffInfo, error)
	IsSuperuser(userID int) (bool, error)
}
