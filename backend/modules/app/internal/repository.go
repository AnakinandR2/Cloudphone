package app

import "gorm.io/gorm"

// AdminApp 是运营视角的应用条目：本地绑定 + 所属用户信息（join users）。
type AdminApp struct {
	CustomerApp
	UserPhone    string `gorm:"column:user_phone" json:"userPhone"`
	UserNickname string `gorm:"column:user_nickname" json:"userNickname"`
}

type repository interface {
	listByUser(userID int) ([]CustomerApp, error)
	getByIDs(userID int, ids []int) ([]CustomerApp, error)
	create(a *CustomerApp) error
	update(a *CustomerApp) error
	deleteByIDs(userID int, ids []int) error

	// 运营侧（跨用户，仅用户上传，不含应用商店）：
	listAll() ([]AdminApp, error)
	getAllByIDs(ids []int) ([]CustomerApp, error)
	deleteAllByIDs(ids []int) error

	// 应用商店（Store=true，admin 维护）：
	listStore() ([]CustomerApp, error)
	getStoreByIDs(ids []int) ([]CustomerApp, error)
	deleteStoreByIDs(ids []int) error
}

type repo struct{ db *gorm.DB }

func newRepository(db *gorm.DB) repository { return &repo{db: db} }

func (r *repo) listByUser(userID int) ([]CustomerApp, error) {
	var list []CustomerApp
	err := r.db.Where("user_id = ?", userID).Order("id DESC").Find(&list).Error
	return list, err
}

func (r *repo) getByIDs(userID int, ids []int) ([]CustomerApp, error) {
	var list []CustomerApp
	if len(ids) == 0 {
		return list, nil
	}
	err := r.db.Where("user_id = ? AND id IN ?", userID, ids).Find(&list).Error
	return list, err
}

func (r *repo) create(a *CustomerApp) error { return r.db.Create(a).Error }

func (r *repo) update(a *CustomerApp) error { return r.db.Save(a).Error }

func (r *repo) deleteByIDs(userID int, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Where("user_id = ? AND id IN ?", userID, ids).Delete(&CustomerApp{}).Error
}

// listAll 运营查看全部「用户上传」的应用（不含应用商店；join users 取手机号/昵称）。
func (r *repo) listAll() ([]AdminApp, error) {
	var list []AdminApp
	err := r.db.Table("customer_apps AS a").
		Select("a.*, u.phone AS user_phone, u.nickname AS user_nickname").
		Joins("LEFT JOIN users u ON u.id = a.user_id").
		Where("a.store = ?", false).
		Order("a.id DESC").
		Scan(&list).Error
	return list, err
}

func (r *repo) getAllByIDs(ids []int) ([]CustomerApp, error) {
	var list []CustomerApp
	if len(ids) == 0 {
		return list, nil
	}
	err := r.db.Where("store = ? AND id IN ?", false, ids).Find(&list).Error
	return list, err
}

func (r *repo) deleteAllByIDs(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Where("store = ? AND id IN ?", false, ids).Delete(&CustomerApp{}).Error
}

// listStore 应用商店应用列表（admin 上传，面向全部用户）。
func (r *repo) listStore() ([]CustomerApp, error) {
	var list []CustomerApp
	err := r.db.Where("store = ?", true).Order("id DESC").Find(&list).Error
	return list, err
}

func (r *repo) getStoreByIDs(ids []int) ([]CustomerApp, error) {
	var list []CustomerApp
	if len(ids) == 0 {
		return list, nil
	}
	err := r.db.Where("store = ? AND id IN ?", true, ids).Find(&list).Error
	return list, err
}

func (r *repo) deleteStoreByIDs(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Where("store = ? AND id IN ?", true, ids).Delete(&CustomerApp{}).Error
}
