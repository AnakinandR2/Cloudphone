package openapi

import (
	"time"

	"gorm.io/gorm"
)

type repository interface {
	create(k *APIKey) error
	findByHash(hash string) (*APIKey, error)
	listByUser(userID int) ([]APIKey, error)
	ownedByID(userID int, id uint) (*APIKey, error)
	revoke(id uint, at time.Time) error
	touchLastUsed(id uint, at time.Time) error
}

type gormRepository struct{ db *gorm.DB }

func newRepository(db *gorm.DB) repository { return &gormRepository{db: db} }

func (r *gormRepository) create(k *APIKey) error { return r.db.Create(k).Error }

func (r *gormRepository) findByHash(hash string) (*APIKey, error) {
	var k APIKey
	if err := r.db.Where("key_hash = ?", hash).First(&k).Error; err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *gormRepository) listByUser(userID int) ([]APIKey, error) {
	var out []APIKey
	err := r.db.Where("user_id = ?", userID).Order("id DESC").Find(&out).Error
	return out, err
}

func (r *gormRepository) ownedByID(userID int, id uint) (*APIKey, error) {
	var k APIKey
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&k).Error; err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *gormRepository) revoke(id uint, at time.Time) error {
	return r.db.Model(&APIKey{}).Where("id = ?", id).Update("revoked_at", at).Error
}

func (r *gormRepository) touchLastUsed(id uint, at time.Time) error {
	return r.db.Model(&APIKey{}).Where("id = ?", id).Update("last_used_at", at).Error
}
