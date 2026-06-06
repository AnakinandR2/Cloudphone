package example

import (
	"manager-backend/framework/apperr"
	"manager-backend/framework/query"
)

// serviceImpl 示例业务服务，依赖注入的 repository（不直接接触全局 DB）。
type serviceImpl struct{ repo repository }

// ExampleService 模块内服务实例，由 module.Init 注入 DB 后装配。
var ExampleService *serviceImpl

func newService(repo repository) *serviceImpl { return &serviceImpl{repo: repo} }

// orderable 列表允许排序的字段白名单（杜绝 SQL 注入）。
var orderable = map[string]bool{"id": true, "title": true, "created_at": true, "updated_at": true}

func (s *serviceImpl) GetList(page, size int, title string, order, sort string) ([]ExampleItem, int64, error) {
	total, err := s.repo.count(title)
	if err != nil {
		return nil, 0, err
	}
	orderClause := query.SafeOrder(order, sort, orderable, "id DESC")
	items, err := s.repo.list((page-1)*size, size, title, orderClause)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *serviceImpl) GetByID(id int) (*ExampleItem, error) {
	item, err := s.repo.findByID(id)
	if err != nil {
		return nil, apperr.NotFound("数据不存在")
	}
	return item, nil
}

func (s *serviceImpl) Create(req *ExampleItemCreate) (*ExampleItem, error) {
	item := ExampleItem{Title: req.Title, Content: req.Content}
	if err := s.repo.create(&item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *serviceImpl) Update(id int, req *ExampleItemUpdate) (*ExampleItem, error) {
	if _, err := s.GetByID(id); err != nil {
		return nil, err
	}

	fields := make(map[string]interface{})
	if req.Title != "" {
		fields["title"] = req.Title
	}
	if req.Content != "" {
		fields["content"] = req.Content
	}
	if len(fields) > 0 {
		if err := s.repo.update(id, fields); err != nil {
			return nil, err
		}
	}
	return s.GetByID(id)
}

func (s *serviceImpl) Delete(id int) error {
	if _, err := s.GetByID(id); err != nil {
		return err
	}
	return s.repo.delete(id)
}
