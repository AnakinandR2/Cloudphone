package library

import (
	"strings"

	"manager-backend/framework/apperr"
)

// ListFolders 列出用户全部文件夹（递归树由前端组装）。
func (s *serviceImpl) ListFolders(userID int) ([]LibraryFolder, error) {
	return s.repo.listFolders(userID)
}

// CreateFolderRequest 新建文件夹请求体。
type CreateFolderRequest struct {
	Name     string `json:"name"`
	ParentID uint   `json:"parent_id"`
}

// CreateFolder 新建文件夹（父夹归属校验）。
func (s *serviceImpl) CreateFolder(userID int, req CreateFolderRequest) (*LibraryFolder, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apperr.Validation("文件夹名不能为空")
	}
	if req.ParentID != 0 {
		p, err := s.repo.getFolder(userID, req.ParentID)
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, apperr.NotFound("父文件夹不存在")
		}
	}
	exists, err := s.repo.folderNameExists(userID, req.ParentID, name, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.Conflict("同名文件夹已存在")
	}
	fo := &LibraryFolder{UserID: uint(userID), ParentID: req.ParentID, Name: name}
	if err := s.repo.createFolder(fo); err != nil {
		return nil, err
	}
	return fo, nil
}

// RenameFolder 重命名文件夹。
func (s *serviceImpl) RenameFolder(userID int, id uint, name string) (*LibraryFolder, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, apperr.Validation("文件夹名不能为空")
	}
	fo, err := s.repo.getFolder(userID, id)
	if err != nil {
		return nil, err
	}
	if fo == nil {
		return nil, apperr.NotFound("文件夹不存在")
	}
	exists, err := s.repo.folderNameExists(userID, fo.ParentID, name, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.Conflict("同名文件夹已存在")
	}
	fo.Name = name
	if err := s.repo.updateFolder(fo); err != nil {
		return nil, err
	}
	return fo, nil
}

// DeleteFolder 删除文件夹（拒绝删非空：含文件或子文件夹）。
func (s *serviceImpl) DeleteFolder(userID int, id uint) error {
	fo, err := s.repo.getFolder(userID, id)
	if err != nil {
		return err
	}
	if fo == nil {
		return apperr.NotFound("文件夹不存在")
	}
	nFiles, err := s.repo.countFilesInFolder(userID, id)
	if err != nil {
		return err
	}
	nSub, err := s.repo.countSubFolders(userID, id)
	if err != nil {
		return err
	}
	if nFiles > 0 || nSub > 0 {
		return apperr.Conflict("文件夹非空，请先清空后再删除")
	}
	return s.repo.deleteFolder(userID, id)
}

// MoveFolder 移动文件夹到新父级（0=根）。校验：归属、目标存在、防环（不能移到自身或子孙）、目标下无同名。
func (s *serviceImpl) MoveFolder(userID int, id uint, newParentID uint) (*LibraryFolder, error) {
	fo, err := s.repo.getFolder(userID, id)
	if err != nil {
		return nil, err
	}
	if fo == nil {
		return nil, apperr.NotFound("文件夹不存在")
	}
	if newParentID == fo.ParentID {
		return fo, nil // 未变化
	}
	if newParentID == id {
		return nil, apperr.Validation("不能移动到自身")
	}
	all, err := s.repo.listFolders(userID)
	if err != nil {
		return nil, err
	}
	if newParentID != 0 {
		p, perr := s.repo.getFolder(userID, newParentID)
		if perr != nil {
			return nil, perr
		}
		if p == nil {
			return nil, apperr.NotFound("目标文件夹不存在")
		}
		if isDescendantFolder(all, id, newParentID) {
			return nil, apperr.Validation("不能移动到自己的子文件夹")
		}
	}
	exists, err := s.repo.folderNameExists(userID, newParentID, fo.Name, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.Conflict("目标位置已存在同名文件夹")
	}
	fo.ParentID = newParentID
	if err := s.repo.updateFolder(fo); err != nil {
		return nil, err
	}
	return fo, nil
}

// isDescendantFolder 判断 target 是否为 ancestor 的子孙（沿 parent 链上溯，经过 ancestor 即是）。
func isDescendantFolder(all []LibraryFolder, ancestor, target uint) bool {
	parent := make(map[uint]uint, len(all))
	for _, f := range all {
		parent[f.ID] = f.ParentID
	}
	for cur := target; cur != 0; cur = parent[cur] {
		if cur == ancestor {
			return true
		}
	}
	return false
}
