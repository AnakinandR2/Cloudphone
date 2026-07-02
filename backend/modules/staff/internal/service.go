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

// --- 授权收敛层（最小权限边界）---
// 以下 *Checked 方法是 handler 唯一入口：在原始增删改之上做操作者权限边界校验。
// 原始的 CreateStaffWithForm/UpdateStaffWithForm/DeleteStaff 不含授权，仅供包内/测试造数据。

// permSetOf 返回操作者所有角色权限的并集集合（超管在调用处已短路，不会到这里）。
func (s *serviceImpl) permSetOf(actorID int) (map[string]bool, error) {
	perms, err := s.roles.userPermissions(actorID)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(perms))
	for _, p := range perms {
		set[p] = true
	}
	return set, nil
}

// assertRolesWithinActor 校验待分配的每个角色，其权限均在操作者自身权限集内（防越权赋角色，A3）。
func (s *serviceImpl) assertRolesWithinActor(actorID int, roleIDs []int) error {
	allowed, err := s.permSetOf(actorID)
	if err != nil {
		return err
	}
	for _, rid := range roleIDs {
		perms, err := s.roles.permissionsOf(uint(rid))
		if err != nil {
			return err
		}
		for _, p := range perms {
			if !allowed[p] {
				return apperr.Forbidden("无权分配包含自身未拥有权限的角色")
			}
		}
	}
	return nil
}

// CreateStaffChecked 非超管创建时强制 is_superuser=false（A2）。
func (s *serviceImpl) CreateStaffChecked(req *StaffCreate, actorSuper bool) (*Staff, error) {
	if !actorSuper {
		req.IsSuperuser = false
	}
	return s.CreateStaffWithForm(req)
}

// UpdateStaffChecked 授权收敛更新：
//   - 非超管不得改超管位（A1）、不得赋越权角色（A3）；
//   - 禁用（is_active true→false）递增令牌版本，使被禁用者旧令牌立即失效（A4）。
func (s *serviceImpl) UpdateStaffChecked(id int, req *StaffUpdate, actorID int, actorSuper bool) (*Staff, error) {
	if !actorSuper {
		req.IsSuperuser = nil
		if req.RoleIDs != nil {
			if err := s.assertRolesWithinActor(actorID, *req.RoleIDs); err != nil {
				return nil, err
			}
		}
	}
	// A4：禁用需失效旧令牌。先读当前态判断是否发生 true→false 的禁用。
	disabling := false
	if req.IsActive != nil && !*req.IsActive {
		if cur, err := s.GetStaffByID(id); err == nil && cur.IsActive {
			disabling = true
		}
	}
	u, err := s.UpdateStaffWithForm(id, req)
	if err != nil {
		return nil, err
	}
	if disabling {
		if err := s.users.bumpTokenVersion(id); err != nil {
			return nil, err
		}
	}
	return u, nil
}

// DeleteStaffChecked 删除保护（A5）：禁止自删、非超管删超管、删掉最后一个超管。
func (s *serviceImpl) DeleteStaffChecked(id, actorID int, actorSuper bool) error {
	target, err := s.GetStaffByID(id)
	if err != nil {
		return err
	}
	if id == actorID {
		return apperr.Forbidden("不能删除当前登录账号")
	}
	if target.IsSuperuser {
		if !actorSuper {
			return apperr.Forbidden("无权删除超级管理员")
		}
		count, err := s.users.countSuperusers()
		if err != nil {
			return err
		}
		if count <= 1 {
			return apperr.Conflict("不能删除最后一个超级管理员")
		}
	}
	return s.DeleteStaff(id)
}
