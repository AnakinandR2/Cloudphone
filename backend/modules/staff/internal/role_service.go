package staff

import (
	"fmt"

	"manager-backend/framework/apperr"
	"manager-backend/framework/query"
)

// roleServiceImpl 角色业务服务，依赖注入的仓储。
type roleServiceImpl struct {
	roles roleRepository
}

// RoleService 角色服务单例；仓储字段由 wire 在 module.Init 时注入。
var RoleService = &roleServiceImpl{}

// roleOrderable 角色列表允许排序的字段白名单（杜绝 SQL 注入）。
var roleOrderable = map[string]bool{"id": true, "name": true, "created_at": true}

func (s *roleServiceImpl) GetList(page, size int, name, order, sort string) ([]RoleListItem, int64, error) {
	orderClause := query.SafeOrder(order, sort, roleOrderable, "id ASC")
	roles, total, err := s.roles.list((page-1)*size, size, name, orderClause)
	if err != nil {
		return nil, 0, err
	}

	roleIDs := make([]uint, len(roles))
	for i := range roles {
		roleIDs[i] = roles[i].ID
	}
	permByRole, err := s.roles.countPermissionsByRoleIDs(roleIDs)
	if err != nil {
		return nil, 0, err
	}
	userByRole, err := s.roles.countStaffByRoleIDs(roleIDs)
	if err != nil {
		return nil, 0, err
	}

	list := make([]RoleListItem, len(roles))
	for i, r := range roles {
		list[i] = RoleListItem{
			ID:              int(r.ID),
			Name:            r.Name,
			Description:     r.Description,
			IsBuiltin:       r.IsBuiltin,
			PermissionCount: int(permByRole[r.ID]),
			StaffCount:      int(userByRole[r.ID]),
			CreatedAt:       r.CreatedAt,
			UpdatedAt:       r.UpdatedAt,
		}
	}
	return list, total, nil
}

func (s *roleServiceImpl) GetByID(id int) (*Role, error) {
	roleDB, err := s.roles.findByID(id)
	if err != nil {
		return nil, apperr.NotFound("角色不存在")
	}
	permissions, err := s.roles.permissionsOf(roleDB.ID)
	if err != nil {
		return nil, err
	}
	return &Role{
		ID:          int(roleDB.ID),
		Name:        roleDB.Name,
		Description: roleDB.Description,
		IsBuiltin:   roleDB.IsBuiltin,
		Permissions: permissions,
		CreatedAt:   roleDB.CreatedAt,
		UpdatedAt:   roleDB.UpdatedAt,
	}, nil
}

func (s *roleServiceImpl) Create(req *RoleCreate) (*Role, error) {
	for _, p := range req.Permissions {
		if !IsValidPermission(p) {
			return nil, apperr.Validation(fmt.Sprintf("无效的权限标识: %s", p))
		}
	}

	exists, err := s.roles.existsByName(req.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.Conflict("角色名已存在")
	}

	roleDB := RoleDB{Name: req.Name, Description: req.Description}
	if err := s.roles.create(&roleDB, req.Permissions); err != nil {
		return nil, err
	}
	return s.GetByID(int(roleDB.ID))
}

func (s *roleServiceImpl) Update(id int, req *RoleUpdate) (*Role, error) {
	roleDB, err := s.roles.findByID(id)
	if err != nil {
		return nil, apperr.NotFound("角色不存在")
	}

	for _, p := range req.Permissions {
		if !IsValidPermission(p) {
			return nil, apperr.Validation(fmt.Sprintf("无效的权限标识: %s", p))
		}
	}

	if req.Name != roleDB.Name {
		if roleDB.IsBuiltin {
			return nil, apperr.Conflict("内置角色不可修改名称")
		}
		exists, err := s.roles.existsByNameExcludingID(req.Name, id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, apperr.Conflict("角色名已存在")
		}
	}

	fields := map[string]interface{}{"name": req.Name, "description": req.Description}
	if err := s.roles.update(id, fields, req.Permissions); err != nil {
		return nil, err
	}
	return s.GetByID(id)
}

func (s *roleServiceImpl) Delete(id int) error {
	roleDB, err := s.roles.findByID(id)
	if err != nil {
		return apperr.NotFound("角色不存在")
	}
	if roleDB.IsBuiltin {
		return apperr.Conflict("内置角色不可删除")
	}
	return s.roles.delete(id)
}

// GetStaffPermissions 获取用户的所有权限（所有角色权限的并集）
func (s *roleServiceImpl) GetStaffPermissions(userID int) ([]string, error) {
	return s.roles.userPermissions(userID)
}

// GetStaffRoles 获取用户的角色列表
func (s *roleServiceImpl) GetStaffRoles(userID int) ([]RoleBrief, error) {
	return s.roles.userRoles(userID)
}

// GetStaffRolesForStaff 批量获取多个用户的角色（一次查询，避免列表场景 N+1）
func (s *roleServiceImpl) GetStaffRolesForStaff(userIDs []int) (map[int][]RoleBrief, error) {
	return s.roles.userRolesForUsers(userIDs)
}

// HasPermission 检查用户是否拥有指定权限中的任意一个
func (s *roleServiceImpl) HasPermission(userID int, perms ...string) (bool, error) {
	userPerms, err := s.roles.userPermissions(userID)
	if err != nil {
		return false, err
	}
	permSet := make(map[string]bool, len(userPerms))
	for _, p := range userPerms {
		permSet[p] = true
	}
	for _, p := range perms {
		if permSet[p] {
			return true, nil
		}
	}
	return false, nil
}

// GetAllRoles 获取所有角色（用于下拉选择）
func (s *roleServiceImpl) GetAllRoles() ([]RoleBrief, error) {
	return s.roles.allRoles()
}
