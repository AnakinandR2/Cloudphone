package staff

import "gorm.io/gorm"

// 本文件定义 user 模块（auth/user/role 同一限界上下文）的持久化仓储接口及其 GORM 实现。
// 服务层只依赖接口，便于单测替换、并将事务/SQL 细节收敛在此层；由 module.Init 注入 DB。

// wire 构造仓储并装配到服务单例。各模块 Init 均会调用（传入同一句柄，幂等）。
func wire(database *gorm.DB) {
	users := newStaffRepository(database)
	roles := newRoleRepository(database)
	Service.users = users
	Service.roles = roles
	RoleService.roles = roles
}

// userRepository 用户持久化。
type userRepository interface {
	countExcludingUsernames(names []string) (int64, error)
	findByUsername(username string) (*StaffDB, error)
	findByID(id int) (*StaffDB, error)
	existsByUsername(username string) (bool, error)
	existsByUsernameExcludingID(username string, excludeID int) (bool, error)
	create(u *StaffDB) error
	updateFields(id int, fields map[string]interface{}) error
	// updateWithRoles 在单个事务内更新用户字段；当 roleIDs 非 nil 时重置该用户的角色关联。
	updateWithRoles(id int, fields map[string]interface{}, roleIDs *[]int) error
	delete(id int) error
	list(offset, limit int, username, orderClause string) ([]StaffDB, int64, error)
	tokenVersion(id int) (int, error)
	bumpTokenVersion(id int) error
	countSuperusers() (int64, error)
}

// roleRepository 角色 / 权限 / 用户-角色关联的持久化。
type roleRepository interface {
	list(offset, limit int, name, orderClause string) ([]RoleDB, int64, error)
	countPermissionsByRoleIDs(roleIDs []uint) (map[uint]int64, error)
	countStaffByRoleIDs(roleIDs []uint) (map[uint]int64, error)
	findByID(id int) (*RoleDB, error)
	permissionsOf(roleID uint) ([]string, error)
	existsByName(name string) (bool, error)
	existsByNameExcludingID(name string, excludeID int) (bool, error)
	create(role *RoleDB, perms []string) error
	update(id int, fields map[string]interface{}, perms []string) error
	delete(id int) error
	userPermissions(userID int) ([]string, error)
	userRoles(userID int) ([]RoleBrief, error)
	userRolesForUsers(userIDs []int) (map[int][]RoleBrief, error)
	allRoles() ([]RoleBrief, error)
}

// --- 用户仓储 GORM 实现 ---

type gormStaffRepository struct{ db *gorm.DB }

func newStaffRepository(db *gorm.DB) userRepository { return &gormStaffRepository{db: db} }

func (r *gormStaffRepository) countExcludingUsernames(names []string) (int64, error) {
	var count int64
	err := r.db.Model(&StaffDB{}).Where("username NOT IN ?", names).Count(&count).Error
	return count, err
}

// countSuperusers 统计超级管理员数量（删除保护用于防止删掉最后一个超管）。
func (r *gormStaffRepository) countSuperusers() (int64, error) {
	var count int64
	err := r.db.Model(&StaffDB{}).Where("is_superuser = ?", true).Count(&count).Error
	return count, err
}

func (r *gormStaffRepository) findByUsername(username string) (*StaffDB, error) {
	var u StaffDB
	if err := r.db.Where("username = ?", username).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *gormStaffRepository) findByID(id int) (*StaffDB, error) {
	var u StaffDB
	if err := r.db.First(&u, id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *gormStaffRepository) existsByUsername(username string) (bool, error) {
	var count int64
	err := r.db.Model(&StaffDB{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}

func (r *gormStaffRepository) existsByUsernameExcludingID(username string, excludeID int) (bool, error) {
	var count int64
	err := r.db.Model(&StaffDB{}).Where("username = ? AND id != ?", username, excludeID).Count(&count).Error
	return count > 0, err
}

func (r *gormStaffRepository) create(u *StaffDB) error { return r.db.Create(u).Error }

func (r *gormStaffRepository) updateFields(id int, fields map[string]interface{}) error {
	return r.db.Model(&StaffDB{}).Where("id = ?", id).Updates(fields).Error
}

func (r *gormStaffRepository) updateWithRoles(id int, fields map[string]interface{}, roleIDs *[]int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if len(fields) > 0 {
			if err := tx.Model(&StaffDB{}).Where("id = ?", id).Updates(fields).Error; err != nil {
				return err
			}
		}
		if roleIDs != nil {
			if err := tx.Where("staff_id = ?", id).Delete(&StaffRoleDB{}).Error; err != nil {
				return err
			}
			if len(*roleIDs) > 0 {
				urs := make([]StaffRoleDB, len(*roleIDs))
				for i, rid := range *roleIDs {
					urs[i] = StaffRoleDB{StaffID: uint(id), RoleID: uint(rid)}
				}
				if err := tx.Create(&urs).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *gormStaffRepository) delete(id int) error { return r.db.Delete(&StaffDB{}, id).Error }

func (r *gormStaffRepository) tokenVersion(id int) (int, error) {
	var u StaffDB
	if err := r.db.Select("token_version").First(&u, id).Error; err != nil {
		return 0, err
	}
	return u.TokenVersion, nil
}

func (r *gormStaffRepository) bumpTokenVersion(id int) error {
	return r.db.Model(&StaffDB{}).Where("id = ?", id).
		UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
}

func (r *gormStaffRepository) list(offset, limit int, username, orderClause string) ([]StaffDB, int64, error) {
	q := r.db.Model(&StaffDB{})
	if username != "" {
		q = q.Where("username LIKE ?", "%"+username+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []StaffDB
	if err := q.Order(orderClause).Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// --- 角色仓储 GORM 实现 ---

type gormRoleRepository struct{ db *gorm.DB }

func newRoleRepository(db *gorm.DB) roleRepository { return &gormRoleRepository{db: db} }

func (r *gormRoleRepository) list(offset, limit int, name, orderClause string) ([]RoleDB, int64, error) {
	q := r.db.Model(&RoleDB{})
	if name != "" {
		q = q.Where("name LIKE ?", "%"+name+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var roles []RoleDB
	if err := q.Order(orderClause).Offset(offset).Limit(limit).Find(&roles).Error; err != nil {
		return nil, 0, err
	}
	return roles, total, nil
}

func (r *gormRoleRepository) countPermissionsByRoleIDs(roleIDs []uint) (map[uint]int64, error) {
	return groupCount(r.db.Model(&RolePermissionDB{}), roleIDs)
}

func (r *gormRoleRepository) countStaffByRoleIDs(roleIDs []uint) (map[uint]int64, error) {
	return groupCount(r.db.Model(&StaffRoleDB{}), roleIDs)
}

// groupCount 按 role_id 聚合计数（一次查询）。
func groupCount(q *gorm.DB, roleIDs []uint) (map[uint]int64, error) {
	out := make(map[uint]int64)
	if len(roleIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		RoleID uint  `gorm:"column:role_id"`
		Cnt    int64 `gorm:"column:cnt"`
	}
	if err := q.Select("role_id, COUNT(*) AS cnt").
		Where("role_id IN ?", roleIDs).Group("role_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.RoleID] = row.Cnt
	}
	return out, nil
}

func (r *gormRoleRepository) findByID(id int) (*RoleDB, error) {
	var role RoleDB
	if err := r.db.First(&role, id).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *gormRoleRepository) permissionsOf(roleID uint) ([]string, error) {
	var perms []RolePermissionDB
	if err := r.db.Where("role_id = ?", roleID).Find(&perms).Error; err != nil {
		return nil, err
	}
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = p.Permission
	}
	return out, nil
}

func (r *gormRoleRepository) existsByName(name string) (bool, error) {
	var count int64
	err := r.db.Model(&RoleDB{}).Where("name = ?", name).Count(&count).Error
	return count > 0, err
}

func (r *gormRoleRepository) existsByNameExcludingID(name string, excludeID int) (bool, error) {
	var count int64
	err := r.db.Model(&RoleDB{}).Where("name = ? AND id != ?", name, excludeID).Count(&count).Error
	return count > 0, err
}

func (r *gormRoleRepository) create(role *RoleDB, perms []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(role).Error; err != nil {
			return err
		}
		return insertPermissions(tx, role.ID, perms)
	})
}

func (r *gormRoleRepository) update(id int, fields map[string]interface{}, perms []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&RoleDB{}).Where("id = ?", id).Updates(fields).Error; err != nil {
			return err
		}
		if err := tx.Where("role_id = ?", id).Delete(&RolePermissionDB{}).Error; err != nil {
			return err
		}
		return insertPermissions(tx, uint(id), perms)
	})
}

func insertPermissions(tx *gorm.DB, roleID uint, perms []string) error {
	if len(perms) == 0 {
		return nil
	}
	rows := make([]RolePermissionDB, len(perms))
	for i, p := range perms {
		rows[i] = RolePermissionDB{RoleID: roleID, Permission: p}
	}
	return tx.Create(&rows).Error
}

func (r *gormRoleRepository) delete(id int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", id).Delete(&RolePermissionDB{}).Error; err != nil {
			return err
		}
		if err := tx.Where("role_id = ?", id).Delete(&StaffRoleDB{}).Error; err != nil {
			return err
		}
		return tx.Delete(&RoleDB{}, id).Error
	})
}

func (r *gormRoleRepository) userPermissions(userID int) ([]string, error) {
	var permissions []string
	err := r.db.Model(&RolePermissionDB{}).
		Select("DISTINCT role_permissions.permission").
		Joins("JOIN staff_roles ON staff_roles.role_id = role_permissions.role_id").
		Where("staff_roles.staff_id = ?", userID).
		Pluck("permission", &permissions).Error
	return permissions, err
}

func (r *gormRoleRepository) userRoles(userID int) ([]RoleBrief, error) {
	var roles []RoleBrief
	err := r.db.Model(&RoleDB{}).
		Select("roles.id, roles.name").
		Joins("JOIN staff_roles ON staff_roles.role_id = roles.id").
		Where("staff_roles.staff_id = ?", userID).
		Find(&roles).Error
	return roles, err
}

func (r *gormRoleRepository) userRolesForUsers(userIDs []int) (map[int][]RoleBrief, error) {
	out := make(map[int][]RoleBrief)
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		StaffID uint   `gorm:"column:staff_id"`
		RoleID  int    `gorm:"column:role_id"`
		Name    string `gorm:"column:name"`
	}
	err := r.db.Table("staff_roles").
		Select("staff_roles.staff_id, roles.id AS role_id, roles.name").
		Joins("INNER JOIN roles ON roles.id = staff_roles.role_id").
		Where("staff_roles.staff_id IN ?", userIDs).
		Order("staff_roles.staff_id ASC, roles.id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		uid := int(row.StaffID)
		out[uid] = append(out[uid], RoleBrief{ID: row.RoleID, Name: row.Name})
	}
	return out, nil
}

func (r *gormRoleRepository) allRoles() ([]RoleBrief, error) {
	var roles []RoleDB
	if err := r.db.Order("id ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	out := make([]RoleBrief, len(roles))
	for i, role := range roles {
		out[i] = RoleBrief{ID: int(role.ID), Name: role.Name}
	}
	return out, nil
}
