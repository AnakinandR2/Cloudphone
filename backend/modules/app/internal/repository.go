package app

import "gorm.io/gorm"

// uploaderInfo 是某用户应用文件的上传者信息（运营治理列表用，join users）。
type uploaderInfo struct {
	UserID       uint   `gorm:"column:user_id"`
	UserPhone    string `gorm:"column:user_phone"`
	UserNickname string `gorm:"column:user_nickname"`
}

type repository interface {
	// 用户应用元数据（app_user_meta，PK = library_file_id）：
	getUserMeta(fileID uint) (*AppUserMeta, error)
	getUserMetaByIDs(fileIDs []uint) ([]AppUserMeta, error)
	listAllUserMeta() ([]AppUserMeta, error)
	upsertUserMeta(m *AppUserMeta) error
	deleteUserMeta(fileID uint) error

	// 运营治理：按 user_id 取上传者手机号/昵称（library 文件无 user join，故另查 users）。
	listUploaders(userIDs []uint) ([]uploaderInfo, error)

	// 应用市场（app_market，平台资产）：
	createMarket(a *AppMarket) error
	getMarketByID(id uint) (*AppMarket, error)
	getMarketByIDs(ids []uint) ([]AppMarket, error)
	listMarket(readyOnly bool) ([]AppMarket, error)
	deleteMarketByIDs(ids []uint) error
}

type repo struct{ db *gorm.DB }

func newRepository(db *gorm.DB) repository { return &repo{db: db} }

// ---- app_user_meta ----

func (r *repo) getUserMeta(fileID uint) (*AppUserMeta, error) {
	var m AppUserMeta
	err := r.db.Where("library_file_id = ?", fileID).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *repo) getUserMetaByIDs(fileIDs []uint) ([]AppUserMeta, error) {
	var list []AppUserMeta
	if len(fileIDs) == 0 {
		return list, nil
	}
	err := r.db.Where("library_file_id IN ?", fileIDs).Find(&list).Error
	return list, err
}

// upsertUserMeta 按主键 library_file_id 新增或整行覆盖（finalize 可重跑）。
func (r *repo) upsertUserMeta(m *AppUserMeta) error {
	var n int64
	if err := r.db.Model(&AppUserMeta{}).Where("library_file_id = ?", m.LibraryFileID).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return r.db.Create(m).Error
	}
	return r.db.Model(&AppUserMeta{}).Where("library_file_id = ?", m.LibraryFileID).
		Updates(map[string]any{
			"user_id":      m.UserID,
			"package_name": m.PackageName,
			"version":      m.Version,
			"app_name":     m.AppName,
			"icon_url":     m.IconURL,
			"md5":          m.MD5,
			"parse_status": m.ParseStatus,
			"parse_error":  m.ParseError,
		}).Error
}

func (r *repo) listAllUserMeta() ([]AppUserMeta, error) {
	var list []AppUserMeta
	err := r.db.Order("library_file_id DESC").Find(&list).Error
	return list, err
}

func (r *repo) deleteUserMeta(fileID uint) error {
	return r.db.Where("library_file_id = ?", fileID).Delete(&AppUserMeta{}).Error
}

func (r *repo) listUploaders(userIDs []uint) ([]uploaderInfo, error) {
	var list []uploaderInfo
	if len(userIDs) == 0 {
		return list, nil
	}
	err := r.db.Table("users").
		Select("id AS user_id, phone AS user_phone, nickname AS user_nickname").
		Where("id IN ?", userIDs).
		Scan(&list).Error
	return list, err
}

// ---- app_market ----

func (r *repo) createMarket(a *AppMarket) error { return r.db.Create(a).Error }

func (r *repo) getMarketByID(id uint) (*AppMarket, error) {
	var a AppMarket
	err := r.db.Where("id = ?", id).First(&a).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *repo) getMarketByIDs(ids []uint) ([]AppMarket, error) {
	var list []AppMarket
	if len(ids) == 0 {
		return list, nil
	}
	err := r.db.Where("id IN ?", ids).Find(&list).Error
	return list, err
}

func (r *repo) listMarket(readyOnly bool) ([]AppMarket, error) {
	var list []AppMarket
	q := r.db.Order("id DESC")
	if readyOnly {
		q = q.Where("parse_status = ?", ParseStatusReady)
	}
	err := q.Find(&list).Error
	return list, err
}

func (r *repo) deleteMarketByIDs(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Where("id IN ?", ids).Delete(&AppMarket{}).Error
}
