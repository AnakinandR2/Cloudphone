package accesslog

import (
	"time"

	"gorm.io/gorm"
)

// listFilters 列表查询的过滤条件（均为可信的服务端约束或经处理的参数）。
type listFilters struct {
	username    string
	scope       string // staff / user / anonymous
	method      string
	path        string
	statusGroup string // 2xx / 4xx / 5xx
	startTime   string
	endTime     string
}

// repository 封装访问日志的持久化。
type repository interface {
	count(f listFilters) (int64, error)
	list(offset, limit int, f listFilters, orderClause string) ([]AccessLogListItem, error)
	findByID(id int) (*AccessLog, error)
	createBatch(entries []AccessLog) error
	deleteOlderThan(cutoff time.Time, limit int) (int64, error)
}

type gormRepository struct{ db *gorm.DB }

func newRepository(db *gorm.DB) repository { return &gormRepository{db: db} }

func (r *gormRepository) applyFilters(f listFilters) *gorm.DB {
	q := r.db.Model(&AccessLog{})
	if f.username != "" {
		q = q.Where("username LIKE ?", "%"+f.username+"%")
	}
	if f.scope != "" {
		q = q.Where("scope = ?", f.scope)
	}
	if f.method != "" {
		q = q.Where("method = ?", f.method)
	}
	if f.path != "" {
		q = q.Where("path LIKE ?", "%"+f.path+"%")
	}
	switch f.statusGroup {
	case "2xx":
		q = q.Where("status_code BETWEEN 200 AND 299")
	case "4xx":
		q = q.Where("status_code BETWEEN 400 AND 499")
	case "5xx":
		q = q.Where("status_code BETWEEN 500 AND 599")
	}
	if f.startTime != "" {
		q = q.Where("created_at >= ?", f.startTime)
	}
	if f.endTime != "" {
		q = q.Where("created_at <= ?", f.endTime)
	}
	return q
}

func (r *gormRepository) count(f listFilters) (int64, error) {
	var total int64
	err := r.applyFilters(f).Count(&total).Error
	return total, err
}

func (r *gormRepository) list(offset, limit int, f listFilters, orderClause string) ([]AccessLogListItem, error) {
	var items []AccessLogListItem
	err := r.applyFilters(f).
		Select("id, user_id, scope, username, method, path, status_code, latency_ms, client_ip, created_at").
		Order(orderClause).Offset(offset).Limit(limit).Find(&items).Error
	return items, err
}

func (r *gormRepository) findByID(id int) (*AccessLog, error) {
	var item AccessLog
	if err := r.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *gormRepository) createBatch(entries []AccessLog) error {
	if len(entries) == 0 {
		return nil
	}
	return r.db.CreateInBatches(entries, len(entries)).Error
}

func (r *gormRepository) deleteOlderThan(cutoff time.Time, limit int) (int64, error) {
	result := r.db.Where("created_at < ?", cutoff).Limit(limit).Delete(&AccessLog{})
	return result.RowsAffected, result.Error
}
