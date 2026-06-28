package user

import (
	"errors"
	"math/rand"

	"gorm.io/gorm"
)

// repository 前台用户持久化。
type repository interface {
	existsByPhone(phone string) (bool, error)
	findByPhone(phone string) (*UserDB, error)
	findByID(id int) (*UserDB, error)
	create(c *UserDB) error
	tokenVersion(id int) (int, error)
	bumpTokenVersion(id int) error
	// 管理侧
	list(offset, limit int, phone string, active *bool) ([]UserDB, int64, error)
	setActive(id int, active bool) error
	// 跨模块门面：按 ID 批量取手机号
	phonesByIDs(ids []uint) (map[uint]string, error)
}

type gormRepository struct{ db *gorm.DB }

func newRepository(db *gorm.DB) repository { return &gormRepository{db: db} }

func (r *gormRepository) existsByPhone(phone string) (bool, error) {
	var count int64
	err := r.db.Model(&UserDB{}).Where("phone = ?", phone).Count(&count).Error
	return count > 0, err
}

func (r *gormRepository) findByPhone(phone string) (*UserDB, error) {
	var c UserDB
	if err := r.db.Where("phone = ?", phone).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *gormRepository) findByID(id int) (*UserDB, error) {
	var c UserDB
	if err := r.db.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// create 插入前台用户。主键不走自增，而是「当前最大 ID + 1 + 随机 0..4」，
// 让 ID 之间出现不规则间隔，避免通过 ID 推断用户规模。并发撞键时重算重试。
func (r *gormRepository) create(c *UserDB) error {
	if c.ID != 0 { // 显式指定 ID 时按原样插入（如测试/数据导入）
		return r.db.Create(c).Error
	}
	for attempt := 0; attempt < 8; attempt++ {
		var maxID int64
		if err := r.db.Model(&UserDB{}).Select("COALESCE(MAX(id), 0)").Scan(&maxID).Error; err != nil {
			return err
		}
		c.ID = uint(maxID) + 1 + uint(rand.Intn(5)) // 自增基础(+1) 上额外加 0..4

		err := r.db.Create(c).Error
		if err == nil {
			return nil
		}
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			c.ID = 0 // 并发下 ID 撞车，重算重试（手机号唯一性已在 service 预校验）
			continue
		}
		return err
	}
	return errors.New("分配用户 ID 失败，请重试")
}

func (r *gormRepository) tokenVersion(id int) (int, error) {
	var c UserDB
	if err := r.db.Select("token_version").First(&c, id).Error; err != nil {
		return 0, err
	}
	return c.TokenVersion, nil
}

func (r *gormRepository) bumpTokenVersion(id int) error {
	return r.db.Model(&UserDB{}).Where("id = ?", id).
		UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
}

func (r *gormRepository) list(offset, limit int, phone string, active *bool) ([]UserDB, int64, error) {
	q := r.db.Model(&UserDB{})
	if phone != "" {
		q = q.Where("phone LIKE ?", "%"+phone+"%")
	}
	if active != nil {
		q = q.Where("is_active = ?", *active)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []UserDB
	if err := q.Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *gormRepository) setActive(id int, active bool) error {
	return r.db.Model(&UserDB{}).Where("id = ?", id).Update("is_active", active).Error
}

// phonesByIDs 批量按用户 ID 取手机号，组装成 map[id]phone。
// 空 ids 直接返回空 map（不查库），避免无谓 SQL。
func (r *gormRepository) phonesByIDs(ids []uint) (map[uint]string, error) {
	out := make(map[uint]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []UserDB
	if err := r.db.Model(&UserDB{}).Select("id", "phone").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		out[rows[i].ID] = rows[i].Phone
	}
	return out, nil
}
