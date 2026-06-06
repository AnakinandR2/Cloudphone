package accesslog

import (
	"manager-backend/framework/apperr"
	"manager-backend/framework/query"
)

type serviceImpl struct{ repo repository }

// AccessLogService 模块内服务实例，由 module.Init 注入 DB 后装配。
var AccessLogService *serviceImpl

func newService(repo repository) *serviceImpl { return &serviceImpl{repo: repo} }

var orderable = map[string]bool{"id": true, "created_at": true, "status_code": true, "latency_ms": true}

// ListQuery 访问日志列表查询参数。
type ListQuery struct {
	Page, Size                                                     int
	Username, Scope, Method, Path, StatusGroup, StartTime, EndTime string
	Order, Sort                                                    string
}

func (s *serviceImpl) GetList(q ListQuery) ([]AccessLogListItem, int64, error) {
	f := listFilters{
		username:    q.Username,
		scope:       q.Scope,
		method:      q.Method,
		path:        q.Path,
		statusGroup: q.StatusGroup,
		startTime:   q.StartTime,
		endTime:     q.EndTime,
	}

	total, err := s.repo.count(f)
	if err != nil {
		return nil, 0, err
	}

	orderClause := query.SafeOrder(q.Order, q.Sort, orderable, "id DESC")
	items, err := s.repo.list((q.Page-1)*q.Size, q.Size, f, orderClause)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *serviceImpl) GetByID(id int) (*AccessLog, error) {
	item, err := s.repo.findByID(id)
	if err != nil {
		return nil, apperr.NotFound("日志记录不存在")
	}
	return item, nil
}
