package staff

import (
	"log"

	"manager-backend/framework/apperr"
	"manager-backend/framework/query"

	"golang.org/x/crypto/bcrypt"
)

// serviceImpl 用户业务服务，依赖注入的仓储（不直接接触全局 DB / GORM）。
type serviceImpl struct {
	users userRepository
	roles roleRepository
}

// Service 用户服务单例；其仓储字段由 wire 在 module.Init 时注入。
var Service = &serviceImpl{}

// builtinUsers 内置用户列表（迁移自动创建的默认账号，promoteIfFirst 排除这些用户）
var builtinUsers = []string{"admin", "test", "readonly"}

// userOrderable 用户列表允许排序的字段白名单（杜绝 SQL 注入）。
var userOrderable = map[string]bool{"id": true, "username": true, "created_at": true}

// IsFirstExternalStaff 检查除内置用户外是否没有其他用户（用于首个外部用户自动设为超管）
func IsFirstExternalStaff() bool {
	count, err := Service.users.countExcludingUsernames(builtinUsers)
	return err == nil && count == 0
}

// promoteIfFirst 如果是第一个外部用户（SSO/注册），自动提升为超管
func promoteIfFirst(username string) bool {
	if IsFirstExternalStaff() {
		log.Printf("首个外部用户 %s 自动设为超级管理员", username)
		return true
	}
	return false
}

func (s *serviceImpl) GetStaffByUsername(username string) (*Staff, error) {
	u, err := s.users.findByUsername(username)
	if err != nil {
		return nil, apperr.NotFound("用户不存在")
	}
	return dbToStaff(u), nil
}

func (s *serviceImpl) GetStaffByID(id int) (*Staff, error) {
	u, err := s.users.findByID(id)
	if err != nil {
		return nil, apperr.NotFound("用户不存在")
	}
	return dbToStaff(u), nil
}

func (s *serviceImpl) VerifyPassword(hashedPassword, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)) == nil
}

func (s *serviceImpl) HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedBytes), nil
}

func (s *serviceImpl) CreateStaff(u *Staff) error {
	exists, err := s.users.existsByUsername(u.Username)
	if err != nil {
		return err
	}
	if exists {
		return apperr.Conflict("用户名已存在")
	}

	hashedPassword := ""
	if u.HashedPassword != "" {
		if hashedPassword, err = s.HashPassword(u.HashedPassword); err != nil {
			return err
		}
	}

	return s.users.create(&StaffDB{
		Username:       u.Username,
		Name:           u.Name,
		HashedPassword: hashedPassword,
		IsActive:       u.IsActive,
		IsSuperuser:    u.IsSuperuser,
		Avatar:         u.Avatar,
	})
}

func (s *serviceImpl) CreateStaffWithForm(req *StaffCreate) (*Staff, error) {
	exists, err := s.users.existsByUsername(req.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.Conflict("用户名已存在")
	}

	hashedPassword, err := s.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	userDB := StaffDB{
		Username:       req.Username,
		Name:           req.Name,
		HashedPassword: hashedPassword,
		IsActive:       req.IsActive,
		IsSuperuser:    req.IsSuperuser,
		Avatar:         req.Avatar,
	}
	if err := s.users.create(&userDB); err != nil {
		return nil, err
	}
	return s.GetStaffByID(int(userDB.ID))
}

func (s *serviceImpl) UpdateStaff(u *Staff) error {
	return s.users.updateFields(u.ID, map[string]interface{}{
		"username":     u.Username,
		"name":         u.Name,
		"is_active":    u.IsActive,
		"is_superuser": u.IsSuperuser,
		"avatar":       u.Avatar,
	})
}

func (s *serviceImpl) UpdateStaffWithForm(id int, req *StaffUpdate) (*Staff, error) {
	u, err := s.GetStaffByID(id)
	if err != nil {
		return nil, err
	}

	if req.Username != "" && req.Username != u.Username {
		exists, err := s.users.existsByUsernameExcludingID(req.Username, id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, apperr.Conflict("用户名已被使用")
		}
	}

	updates := make(map[string]interface{})
	if req.Username != "" {
		updates["username"] = req.Username
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.IsSuperuser != nil {
		updates["is_superuser"] = *req.IsSuperuser
	}
	if req.Avatar != "" {
		updates["avatar"] = req.Avatar
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Password != "" {
		hashedPassword, err := s.HashPassword(req.Password)
		if err != nil {
			return nil, err
		}
		updates["hashed_password"] = hashedPassword
	}

	if err := s.users.updateWithRoles(id, updates, req.RoleIDs); err != nil {
		return nil, err
	}
	// 管理员重置了密码 → 递增令牌版本，强制该用户重新登录。
	if req.Password != "" {
		if err := s.users.bumpTokenVersion(id); err != nil {
			return nil, err
		}
	}
	return s.GetStaffByID(id)
}

// GetStaffListWithRoles 获取用户列表（含角色信息）
func (s *serviceImpl) GetStaffListWithRoles(page, size int, username string, order, sort string) ([]StaffWithRoles, int64, error) {
	users, total, err := s.GetStaffList(page, size, username, order, sort)
	if err != nil {
		return nil, 0, err
	}

	userIDs := make([]int, len(users))
	for i := range users {
		userIDs[i] = users[i].ID
	}
	rolesByUser, err := s.roles.userRolesForUsers(userIDs)
	if err != nil {
		return nil, 0, err
	}

	list := make([]StaffWithRoles, len(users))
	for i, u := range users {
		roles := rolesByUser[u.ID]
		if roles == nil {
			roles = []RoleBrief{}
		}
		list[i] = StaffWithRoles{
			ID:          u.ID,
			Username:    u.Username,
			Name:        u.Name,
			IsActive:    u.IsActive,
			IsSuperuser: u.IsSuperuser,
			Avatar:      u.Avatar,
			Roles:       roles,
			CreatedAt:   u.CreatedAt,
			UpdatedAt:   u.UpdatedAt,
		}
	}
	return list, total, nil
}

// GetStaffProfile 获取用户完整信息（含权限和角色，用于 /auth/me）
func (s *serviceImpl) GetStaffProfile(userID int) (*StaffProfile, error) {
	u, err := s.GetStaffByID(userID)
	if err != nil {
		return nil, err
	}

	var permissions []string
	var roles []RoleBrief

	if u.IsSuperuser {
		permissions = []string{"*"}
		roles = []RoleBrief{}
	} else {
		permissions, _ = s.roles.userPermissions(u.ID)
		if permissions == nil {
			permissions = []string{}
		}
		roles, _ = s.roles.userRoles(u.ID)
		if roles == nil {
			roles = []RoleBrief{}
		}
	}

	return &StaffProfile{
		ID:          u.ID,
		Username:    u.Username,
		Name:        u.Name,
		IsActive:    u.IsActive,
		IsSuperuser: u.IsSuperuser,
		Avatar:      u.Avatar,
		Permissions: permissions,
		Roles:       roles,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}, nil
}

func (s *serviceImpl) UpdatePassword(userID int, oldPassword, newPassword string) error {
	u, err := s.GetStaffByID(userID)
	if err != nil {
		return err
	}
	if !s.VerifyPassword(u.HashedPassword, oldPassword) {
		return apperr.BadRequest("旧密码不正确")
	}
	hashedPassword, err := s.HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.users.updateFields(userID, map[string]interface{}{"hashed_password": hashedPassword}); err != nil {
		return err
	}
	// 改密后递增令牌版本，令旧令牌立即失效（需重新登录）。
	return s.users.bumpTokenVersion(userID)
}

// CurrentTokenVersion 返回用户当前令牌版本（鉴权中间件用于比对）。
func (s *serviceImpl) CurrentTokenVersion(id int) (int, error) {
	return s.users.tokenVersion(id)
}

func (s *serviceImpl) GetStaffList(page, size int, username string, order, sort string) ([]Staff, int64, error) {
	orderClause := query.SafeOrder(order, sort, userOrderable, "id DESC")
	usersDB, total, err := s.users.list((page-1)*size, size, username, orderClause)
	if err != nil {
		return nil, 0, err
	}

	list := make([]Staff, len(usersDB))
	for i := range usersDB {
		list[i] = *dbToStaff(&usersDB[i])
	}
	return list, total, nil
}

func (s *serviceImpl) DeleteStaff(id int) error {
	if _, err := s.GetStaffByID(id); err != nil {
		return err
	}
	return s.users.delete(id)
}
