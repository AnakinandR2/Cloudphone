package note

import (
	"manager-backend/framework/apperr"
	"manager-backend/framework/query"
)

// serviceImpl note 业务服务，依赖注入的 repository（不直接接触全局 DB）。
// 所有方法都带 userID：服务只操作「当前用户自己的」笔记。
type serviceImpl struct{ repo repository }

// NoteService 模块内服务实例，由 module.Init 注入 DB 后装配。
var NoteService *serviceImpl

func newService(repo repository) *serviceImpl { return &serviceImpl{repo: repo} }

// orderable 列表允许排序的字段白名单（杜绝 SQL 注入）。
var orderable = map[string]bool{"id": true, "title": true, "created_at": true}

func (s *serviceImpl) GetList(userID, page, size int, title, order, sort string) ([]Note, int64, error) {
	total, err := s.repo.count(userID, title)
	if err != nil {
		return nil, 0, err
	}
	orderClause := query.SafeOrder(order, sort, orderable, "id DESC")
	items, err := s.repo.list(userID, (page-1)*size, size, title, orderClause)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *serviceImpl) GetByID(userID, id int) (*Note, error) {
	item, err := s.repo.findByID(userID, id)
	if err != nil {
		return nil, apperr.NotFound("笔记不存在")
	}
	return item, nil
}

func (s *serviceImpl) Create(userID int, req *NoteCreate) (*Note, error) {
	item := Note{UserID: uint(userID), Title: req.Title, Content: req.Content}
	if err := s.repo.create(&item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *serviceImpl) Update(userID, id int, req *NoteUpdate) (*Note, error) {
	if _, err := s.GetByID(userID, id); err != nil {
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
		if err := s.repo.update(userID, id, fields); err != nil {
			return nil, err
		}
	}
	return s.GetByID(userID, id)
}

func (s *serviceImpl) Delete(userID, id int) error {
	if _, err := s.GetByID(userID, id); err != nil {
		return err
	}
	return s.repo.delete(userID, id)
}
