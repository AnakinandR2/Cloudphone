package automation

import (
	"strconv"
	"strings"

	"manager-backend/framework/apperr"
)

// ScriptInput 是新建/编辑脚本的入参。
type ScriptInput struct {
	Name        string
	Description string
	LuaContent  string
	FileName    string
}

// ListUserScripts 我的脚本（store=false，本人）。
func (s *serviceImpl) ListUserScripts(userID int) ([]AutomationScript, error) {
	return s.repo.listUserScripts(userID)
}

// ListStoreScripts 脚本商店（store=true）。供 my 商店标签 + admin 商店管理共用。
func (s *serviceImpl) ListStoreScripts() ([]AutomationScript, error) {
	return s.repo.listStoreScripts()
}

// ListUsableScripts 任务创建时可选脚本 = 我的（启用）∪ 商店（启用）。
func (s *serviceImpl) ListUsableScripts(userID int) ([]AutomationScript, error) {
	mine, err := s.repo.listUserScripts(userID)
	if err != nil {
		return nil, err
	}
	store, err := s.repo.listStoreScripts()
	if err != nil {
		return nil, err
	}
	out := make([]AutomationScript, 0, len(mine)+len(store))
	for _, sc := range append(mine, store...) {
		if sc.Status == ScriptEnabled {
			out = append(out, sc)
		}
	}
	return out, nil
}

// createScript 公共建脚本：先落本地拿 id → 生成唯一 mid_name 上传中台 → 取回 scriptId 回写。
func (s *serviceImpl) createScript(ownerID uint, store bool, in ScriptInput) (*AutomationScript, error) {
	if err := s.requireOps(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.LuaContent) == "" {
		return nil, apperr.BadRequest("脚本内容不能为空")
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, apperr.BadRequest("脚本名称不能为空")
	}
	rec := &AutomationScript{
		UserID:      ownerID,
		Store:       store,
		Name:        in.Name,
		Description: in.Description,
		Version:     "1.0.0",
		LuaContent:  in.LuaContent,
		FileName:    in.FileName,
		Status:      ScriptEnabled,
	}
	if err := s.repo.createScript(rec); err != nil {
		return nil, err
	}
	if err := s.uploadAndBind(rec); err != nil {
		// 上传失败回滚本地记录，避免留下没有 scriptId 的孤儿。
		_ = s.repo.deleteScript(rec.ID)
		return nil, err
	}
	return rec, nil
}

// uploadAndBind 用唯一 mid_name 上传 lua 到中台、取回 scriptId 回写记录。
func (s *serviceImpl) uploadAndBind(rec *AutomationScript) error {
	ctx, cancel := opCtx()
	defer cancel()
	midName := uniqueMidName(rec.ID)
	if err := s.ops.UploadTemplate(ctx, midName, rec.Version, rec.Description, []byte(rec.LuaContent)); err != nil {
		return apperr.Internal("上传脚本到中台失败：" + err.Error())
	}
	scriptID, err := s.ops.TemplateScriptID(ctx, midName)
	if err != nil {
		return apperr.Internal("取回 scriptId 失败：" + err.Error())
	}
	if scriptID == 0 {
		return apperr.Internal("上传后未取到 scriptId，请稍后重试")
	}
	rec.MidName = midName
	rec.ScriptID = scriptID
	return s.repo.updateScript(rec)
}

// CreateUserScript 客户新建自己的脚本。
func (s *serviceImpl) CreateUserScript(userID int, in ScriptInput) (*AutomationScript, error) {
	return s.createScript(uint(userID), false, in)
}

// UpdateUserScript 客户编辑自己的脚本：重传得新 scriptId，旧模板 best-effort 删除。
func (s *serviceImpl) UpdateUserScript(userID int, id uint, in ScriptInput) (*AutomationScript, error) {
	rec, err := s.repo.ownedScript(userID, id)
	if err != nil {
		return nil, apperr.NotFound("脚本不存在或不属于你")
	}
	return s.updateScript(rec, in)
}

// updateScript 公共编辑：保存旧 scriptId，重传新版，成功后删旧模板。
func (s *serviceImpl) updateScript(rec *AutomationScript, in ScriptInput) (*AutomationScript, error) {
	if err := s.requireOps(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.LuaContent) == "" {
		return nil, apperr.BadRequest("脚本内容不能为空")
	}
	oldScriptID := rec.ScriptID
	rec.Name = in.Name
	rec.Description = in.Description
	rec.LuaContent = in.LuaContent
	if in.FileName != "" {
		rec.FileName = in.FileName
	}
	rec.Version = bumpVersion(rec.Version)
	if err := s.uploadAndBind(rec); err != nil {
		return nil, err
	}
	// 删旧模板（best-effort，不影响编辑结果；旧任务已发不受影响）。
	if oldScriptID > 0 && oldScriptID != rec.ScriptID {
		ctx, cancel := opCtx()
		defer cancel()
		_ = s.ops.DeleteTemplate(ctx, oldScriptID)
	}
	return rec, nil
}

// ToggleUserScript 启用/停用自己的脚本。
func (s *serviceImpl) ToggleUserScript(userID int, id uint, enabled bool) error {
	rec, err := s.repo.ownedScript(userID, id)
	if err != nil {
		return apperr.NotFound("脚本不存在或不属于你")
	}
	return s.toggleScript(rec, enabled)
}

func (s *serviceImpl) toggleScript(rec *AutomationScript, enabled bool) error {
	if err := s.requireOps(); err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := s.ops.ToggleTemplate(ctx, rec.ScriptID, enabled); err != nil {
		return apperr.Internal("中台启停脚本失败：" + err.Error())
	}
	rec.Status = ScriptDisabled
	if enabled {
		rec.Status = ScriptEnabled
	}
	return s.repo.updateScript(rec)
}

// DeleteUserScript 删除自己的脚本（中台删模板 + 本地删）。
func (s *serviceImpl) DeleteUserScript(userID int, id uint) error {
	rec, err := s.repo.ownedScript(userID, id)
	if err != nil {
		return apperr.NotFound("脚本不存在或不属于你")
	}
	return s.deleteScript(rec)
}

func (s *serviceImpl) deleteScript(rec *AutomationScript) error {
	if s.ops != nil && rec.ScriptID > 0 {
		ctx, cancel := opCtx()
		defer cancel()
		_ = s.ops.DeleteTemplate(ctx, rec.ScriptID) // best-effort
	}
	return s.repo.deleteScript(rec.ID)
}

// ===== 运营侧 =====

// AdminCreateStoreScript 运营上传商店脚本（store=true, user_id=0）。
func (s *serviceImpl) AdminCreateStoreScript(in ScriptInput) (*AutomationScript, error) {
	return s.createScript(0, true, in)
}

// AdminUpdateStoreScript 运营编辑商店脚本。
func (s *serviceImpl) AdminUpdateStoreScript(id uint, in ScriptInput) (*AutomationScript, error) {
	rec, err := s.repo.scriptByID(id)
	if err != nil || !rec.Store {
		return nil, apperr.NotFound("商店脚本不存在")
	}
	return s.updateScript(rec, in)
}

// AdminToggleStoreScript 运营启停商店脚本。
func (s *serviceImpl) AdminToggleStoreScript(id uint, enabled bool) error {
	rec, err := s.repo.scriptByID(id)
	if err != nil || !rec.Store {
		return apperr.NotFound("商店脚本不存在")
	}
	return s.toggleScript(rec, enabled)
}

// AdminDeleteStoreScript 运营删除商店脚本。
func (s *serviceImpl) AdminDeleteStoreScript(id uint) error {
	rec, err := s.repo.scriptByID(id)
	if err != nil || !rec.Store {
		return apperr.NotFound("商店脚本不存在")
	}
	return s.deleteScript(rec)
}

// AdminListUserScripts 治理：全部用户上传的脚本（带上传者 ID）。
func (s *serviceImpl) AdminListUserScripts() ([]AdminScript, error) {
	list, err := s.repo.listAllUserScripts()
	if err != nil {
		return nil, err
	}
	out := make([]AdminScript, 0, len(list))
	for _, sc := range list {
		out = append(out, AdminScript{AutomationScript: sc, UploaderID: sc.UserID})
	}
	return out, nil
}

// AdminToggleUserScript 治理：下架（停用）/恢复任意用户脚本。
func (s *serviceImpl) AdminToggleUserScript(id uint, enabled bool) error {
	rec, err := s.repo.scriptByID(id)
	if err != nil || rec.Store {
		return apperr.NotFound("用户脚本不存在")
	}
	return s.toggleScript(rec, enabled)
}

// AdminDeleteUserScript 治理：删除任意用户脚本。
func (s *serviceImpl) AdminDeleteUserScript(id uint) error {
	rec, err := s.repo.scriptByID(id)
	if err != nil || rec.Store {
		return apperr.NotFound("用户脚本不存在")
	}
	return s.deleteScript(rec)
}

// bumpVersion 把 a.b.c 的末位 +1；非标准格式则回退追加 .1。
func bumpVersion(v string) string {
	parts := strings.Split(v, ".")
	if len(parts) == 3 {
		if n, err := strconv.Atoi(parts[2]); err == nil {
			parts[2] = strconv.Itoa(n + 1)
			return strings.Join(parts, ".")
		}
	}
	return v + ".1"
}
