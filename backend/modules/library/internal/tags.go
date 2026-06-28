package library

import (
	"strings"

	"manager-backend/framework/apperr"
)

// ListTags 列出用户全部标签。
func (s *serviceImpl) ListTags(userID int) ([]LibraryTag, error) {
	return s.repo.listTags(userID)
}

// CreateTagRequest 新建标签请求体。
type CreateTagRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// CreateTag 新建标签。
func (s *serviceImpl) CreateTag(userID int, req CreateTagRequest) (*LibraryTag, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apperr.Validation("标签名不能为空")
	}
	t := &LibraryTag{UserID: uint(userID), Name: name, Color: req.Color}
	if err := s.repo.createTag(t); err != nil {
		return nil, err
	}
	return t, nil
}

// UpdateTagRequest 改名/改色请求体。
type UpdateTagRequest struct {
	Name  *string `json:"name"`
	Color *string `json:"color"`
}

// UpdateTag 重命名 / 改色标签。
func (s *serviceImpl) UpdateTag(userID int, id uint, req UpdateTagRequest) (*LibraryTag, error) {
	t, err := s.repo.getTag(userID, id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, apperr.NotFound("标签不存在")
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, apperr.Validation("标签名不能为空")
		}
		t.Name = name
	}
	if req.Color != nil {
		t.Color = *req.Color
	}
	if err := s.repo.updateTag(t); err != nil {
		return nil, err
	}
	return t, nil
}

// DeleteTag 删除标签（同时解除文件关联）。
func (s *serviceImpl) DeleteTag(userID int, id uint) error {
	t, err := s.repo.getTag(userID, id)
	if err != nil {
		return err
	}
	if t == nil {
		return apperr.NotFound("标签不存在")
	}
	return s.repo.deleteTag(userID, id)
}

// SetFileTags 给文件覆盖式打标签（属主校验 + 仅绑定本人标签）。
func (s *serviceImpl) SetFileTags(userID int, fileID uint, tagIDs []uint) error {
	f, err := s.repo.getFile(userID, fileID)
	if err != nil {
		return err
	}
	if f == nil {
		return apperr.NotFound("文件不存在")
	}
	valid := s.filterOwnedTags(userID, tagIDs)
	return s.repo.setFileTags(fileID, valid)
}

// GetFileTags 取文件的标签 ID 列表（属主校验）。
func (s *serviceImpl) GetFileTags(userID int, fileID uint) ([]uint, error) {
	f, err := s.repo.getFile(userID, fileID)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, apperr.NotFound("文件不存在")
	}
	return s.repo.listFileTagIDs(fileID)
}
