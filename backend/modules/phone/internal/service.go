package phone

import (
	"context"
	"time"

	"manager-backend/framework/apperr"
	"manager-backend/framework/midplat"
	"manager-backend/framework/query"
	"manager-backend/modules/billing"
)

// 异步任务超时阈值：超过则 worker 把云手机收敛到失败态（创建→CREATE_FAILED；开机→STOPPED）。
const (
	createTimeout = 5 * time.Minute
	startTimeout  = 3 * time.Minute
)

// serviceImpl 云手机业务服务，依赖注入的 repository 与中台端口 ops。
// 前台方法都带 userID：只操作「当前用户自己的」云手机；Admin* 方法供后台运营；
// 操作类方法（开关机/远控/指令/应用）经 ops 透传到云手机中台。
type serviceImpl struct {
	repo repository
	ops  midplatPort
}

// PhoneService 模块内服务实例，由 module.Init 注入 DB 后装配。
var PhoneService *serviceImpl

func newService(repo repository, ops midplatPort) *serviceImpl {
	return &serviceImpl{repo: repo, ops: ops}
}

var orderable = map[string]bool{"id": true, "name": true, "status": true, "created_at": true}

// --- 前台（属主隔离）---

func (s *serviceImpl) GetList(userID, page, size int, kw, status, tag, order, sort string) ([]CloudPhone, int64, error) {
	total, err := s.repo.count(userID, kw, status, tag)
	if err != nil {
		return nil, 0, err
	}
	orderClause := query.SafeOrder(order, sort, orderable, "id DESC")
	items, err := s.repo.list(userID, (page-1)*size, size, kw, status, tag, orderClause)
	if err != nil {
		return nil, 0, err
	}
	s.enrichStatuses(items)
	return items, total, nil
}

// AdminListTags 列出全用户用过的标签（去重，运营侧标签选项）。
func (s *serviceImpl) AdminListTags() ([]Tag, error) {
	raws, err := s.repo.listAllTagsRaw()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []Tag{}
	for _, raw := range raws {
		for _, t := range parseTags(raw) {
			if t.Name != "" && !seen[t.Name] {
				seen[t.Name] = true
				out = append(out, t)
			}
		}
	}
	return out, nil
}

// SetTags 批量设置云手机标签（覆盖；仅本人拥有的）。
func (s *serviceImpl) SetTags(userID int, ids []int, tags []Tag) error {
	if len(ids) == 0 {
		return apperr.BadRequest("未选择云手机")
	}
	return s.repo.setTags(userID, ids, marshalTags(tags))
}

// ListTags 列出当前用户所有云手机用过的标签（去重）。
func (s *serviceImpl) ListTags(userID int) ([]Tag, error) {
	raws, err := s.repo.listTagsRaw(userID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []Tag{}
	for _, raw := range raws {
		for _, t := range parseTags(raw) {
			if t.Name != "" && !seen[t.Name] {
				seen[t.Name] = true
				out = append(out, t)
			}
		}
	}
	return out, nil
}

// mapMidplatStatus 把中台云手机实时状态(CloudPhoneEnum)映射为业务展示状态，
// 保证「状态始终和中台保持一致」。返回空串表示不覆盖（保留本地态）。
func mapMidplatStatus(m string) string {
	switch m {
	case "NORMAL":
		return StatusRunning
	case "STARTING":
		return StatusStarting
	case "STOPPING":
		return StatusStopping
	case "STOPPED":
		return StatusStopped
	case "INITIALIZING":
		return StatusCreating
	case "INIT_FAILED":
		return StatusCreateFailed
	case "DISTROYING", "DESTROYING": // 中台 wire 协议拼写为 DISTROYING
		return StatusDestroying
	case "DESTROYED", "":
		return "" // 已销毁 / 查不到：不覆盖，交给本地态与销毁 worker 处理
	default:
		return m // 其它中台态（REBOOTING/RESETTING/FAULTED…）原样透出，保持与中台一致
	}
}

// enrichStatuses 用中台实时状态覆盖「已开通（有 cpId）」云手机的展示状态（只改返回值，不写库）。
// best-effort：中台未配置/查询失败/查不到该 cp 时静默保留本地档案状态。
func (s *serviceImpl) enrichStatuses(items []CloudPhone) {
	if s.ops == nil {
		return
	}
	cpIDs := make([]string, 0, len(items))
	for i := range items {
		if items[i].CpID != "" {
			cpIDs = append(cpIDs, items[i].CpID)
		}
	}
	if len(cpIDs) == 0 {
		return
	}
	ctx, cancel := opCtx()
	defer cancel()
	statuses, err := s.ops.Statuses(ctx, cpIDs)
	if err != nil {
		return
	}
	for i := range items {
		if items[i].CpID == "" {
			continue
		}
		if b := mapMidplatStatus(statuses[items[i].CpID]); b != "" {
			items[i].Status = b
		}
	}
}

func (s *serviceImpl) GetByID(userID, id int) (*CloudPhone, error) {
	item, err := s.repo.findByID(userID, id)
	if err != nil {
		return nil, apperr.NotFound("云手机不存在")
	}
	return item, nil
}

// Create 创建一台云手机。
//   - 已配置中台：调中台异步创建 → 落 CREATING + 创建任务，worker 轮询收敛到 CREATED/CREATE_FAILED。
//   - 未配置中台（本地/测试降级）：仅落本地档案，直接置 CREATED，不触达中台、不建任务。
func (s *serviceImpl) Create(userID int, req *CloudPhoneCreate) (*CloudPhone, error) {
	if err := billing.TryOccupyInstanceSeat(userID); err != nil {
		return nil, err
	}
	item := CloudPhone{
		UserID:  uint(userID),
		Name:    req.Name,
		Region:  req.Region,
		ImageID: req.ImageID,
		ProxyID: req.ProxyID,
		Remark:  req.Remark,
	}
	if s.ops == nil {
		item.Status = StatusCreated
		if err := s.repo.create(&item); err != nil {
			_ = billing.ReleaseInstanceSeat(userID)
			return nil, err
		}
		return &item, nil
	}

	ctx, cancel := opCtx()
	defer cancel()
	res, err := s.ops.Create(ctx, CreateArgs{Region: req.Region, ImageID: req.ImageID})
	if err != nil {
		_ = billing.ReleaseInstanceSeat(userID)
		return nil, apperr.Internal("创建云手机失败：" + err.Error())
	}
	item.Status = StatusCreating
	item.CpID = res.CpID
	item.VmID = res.VmID
	if res.ImageID != "" {
		item.ImageID = res.ImageID
	}
	if res.Region != "" {
		item.Region = res.Region
	}
	if err := s.repo.create(&item); err != nil {
		_ = billing.ReleaseInstanceSeat(userID)
		return nil, err
	}
	// 入库创建任务：worker 轮询中台直到 cpId 状态 = ONLINE → CREATED；超时 → CREATE_FAILED。
	_ = s.repo.createTask(&CpTask{
		UserID:        uint(userID),
		CloudPhoneID:  item.ID,
		CpID:          res.CpID,
		Type:          TaskTypeCreate,
		ExpectedState: MidplatReady,
		Status:        TaskPending,
		Deadline:      time.Now().Add(createTimeout),
	})
	return &item, nil
}

func (s *serviceImpl) Update(userID, id int, req *CloudPhoneUpdate) (*CloudPhone, error) {
	if _, err := s.GetByID(userID, id); err != nil {
		return nil, err
	}
	if err := s.repo.update(userID, id, updateFields(req)); err != nil {
		return nil, err
	}
	return s.GetByID(userID, id)
}

// Delete 销毁一台云手机（异步）。
// 状态门禁：CREATING/STARTING/RUNNING/DESTROYING 不可销毁；CREATE_FAILED/CREATED/STOPPED 可销毁。
//   - 有中台实例（cp_id 非空且中台已配置）：调中台 destroy → 置 DESTROYING + 销毁任务，
//     worker 轮询中台直到实例消失（DESTROYED/查不到）再删本地档案。
//   - 无中台实例（cp_id 空 或 无中台）：直接删本地档案。
func (s *serviceImpl) Delete(userID, id int) error {
	p, err := s.GetByID(userID, id)
	if err != nil {
		return err
	}
	switch p.Status {
	case StatusCreating, StatusStarting, StatusStopping, StatusRunning, StatusDestroying:
		return apperr.Validation("当前状态不可销毁，请先停止")
	}
	if p.CpID == "" || s.ops == nil {
		if err := s.repo.delete(userID, id); err != nil {
			return err
		}
		_ = billing.ReleaseInstanceSeat(userID)
		return nil
	}

	ctx, cancel := opCtx()
	defer cancel()
	if err := s.ops.Destroy(ctx, p.CpID); err != nil {
		return err
	}
	if err := s.repo.setStatus(p.ID, StatusDestroying); err != nil {
		return err
	}
	_ = s.repo.createTask(&CpTask{
		UserID:        uint(userID),
		CloudPhoneID:  p.ID,
		CpID:          p.CpID,
		Type:          TaskTypeDestroy,
		ExpectedState: MidplatDestroyed,
		Status:        TaskPending,
		Deadline:      time.Now().Add(startTimeout),
	})
	return nil
}

// updateFields 把更新请求里非零字段收敛成 map（零值/空串表示不更新）。
func updateFields(req *CloudPhoneUpdate) map[string]interface{} {
	fields := map[string]interface{}{}
	if req.Name != "" {
		fields["name"] = req.Name
	}
	if req.Status != "" {
		fields["status"] = req.Status
	}
	if req.Region != "" {
		fields["region"] = req.Region
	}
	if req.ImageID != "" {
		fields["image_id"] = req.ImageID
	}
	if req.ProxyID != 0 {
		fields["proxy_id"] = req.ProxyID
	}
	if req.Remark != "" {
		fields["remark"] = req.Remark
	}
	return fields
}

// --- 管理侧（不限属主）---

func (s *serviceImpl) AdminList(page, size int, kw, status, tag string, userID int, order, sort string) ([]CloudPhone, int64, error) {
	total, err := s.repo.adminCount(kw, status, tag, userID)
	if err != nil {
		return nil, 0, err
	}
	orderClause := query.SafeOrder(order, sort, orderable, "id DESC")
	items, err := s.repo.adminList((page-1)*size, size, kw, status, tag, userID, orderClause)
	if err != nil {
		return nil, 0, err
	}
	s.enrichStatuses(items)
	return items, total, nil
}

func (s *serviceImpl) AdminGetByID(id int) (*CloudPhone, error) {
	item, err := s.repo.adminFindByID(id)
	if err != nil {
		return nil, apperr.NotFound("云手机不存在")
	}
	return item, nil
}

func (s *serviceImpl) AdminDelete(id int) error {
	if _, err := s.AdminGetByID(id); err != nil {
		return err
	}
	return s.repo.adminDelete(id)
}

// --- 中台操作（开关机 / 远控 / 指令 / 应用）---
// 一律先经 resolveCp 做「本人拥有 + 已开通 + 中台已配置」三重校验，再透传到中台。

// resolveCp 解析「本人拥有且已开通」的云手机，供所有操作类方法复用。
func (s *serviceImpl) resolveCp(userID, id int) (*CloudPhone, error) {
	p, err := s.GetByID(userID, id) // 非本人 → NotFound
	if err != nil {
		return nil, err
	}
	if p.CpID == "" {
		return nil, apperr.Validation("云手机尚未开通（无中台实例），暂不能执行该操作")
	}
	if s.ops == nil {
		return nil, apperr.Internal("云手机中台未配置")
	}
	return p, nil
}

// Power 开机 / 关机，带状态机门禁：
//   - 开机：仅 CREATED / STOPPED 可开机 → 置 STARTING + 开机任务，worker 收敛到 RUNNING（失败/超时 → STOPPED）。
//   - 关机：仅 RUNNING 可关机 → 透传中台关机，置 STOPPED（无中间态）。
func (s *serviceImpl) Power(userID, id int, operation string) error {
	if operation != "开机" && operation != "关机" {
		return apperr.BadRequest("operation 只能是 开机 / 关机")
	}
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()

	if operation == "开机" {
		frozen, err := billing.IsFrozen(userID)
		if err != nil {
			return err
		}
		if frozen {
			return apperr.Forbidden("账户已冻结，无法开机，请续费实例席位")
		}
		if p.Status != StatusCreated && p.Status != StatusStopped {
			return apperr.Validation("当前状态不可开机")
		}
		if err := s.ops.StartOrShutdown(ctx, p.CpID, "开机"); err != nil {
			return err
		}
		if err := s.repo.setStatus(p.ID, StatusStarting); err != nil {
			return err
		}
		_ = s.repo.createTask(&CpTask{
			UserID:        uint(userID),
			CloudPhoneID:  p.ID,
			CpID:          p.CpID,
			Type:          TaskTypeStart,
			ExpectedState: MidplatReady,
			Status:        TaskPending,
			Deadline:      time.Now().Add(startTimeout),
		})
		return nil
	}

	// 关机：仅 RUNNING 可关机 → 置 STOPPING + 关机任务，worker 轮询中台到 STOPPED 再收敛。
	if p.Status != StatusRunning {
		return apperr.Validation("当前状态不可关机")
	}
	if err := s.ops.StartOrShutdown(ctx, p.CpID, "关机"); err != nil {
		return err
	}
	if err := s.repo.setStatus(p.ID, StatusStopping); err != nil {
		return err
	}
	_ = s.repo.createTask(&CpTask{
		UserID:        uint(userID),
		CloudPhoneID:  p.ID,
		CpID:          p.CpID,
		Type:          TaskTypeStop,
		ExpectedState: MidplatStopped,
		Status:        TaskPending,
		Deadline:      time.Now().Add(startTimeout),
	})
	return nil
}

func (s *serviceImpl) Restart(userID, id int) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.Restart(ctx, p.CpID)
}

func (s *serviceImpl) Reset(userID, id int, imageID string) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.Reset(ctx, p.CpID, imageID)
}

// NewDevice 一键新机：保留 cpId 重新初始化（擦数据 + 重装镜像，默认保留代理）。
func (s *serviceImpl) NewDevice(userID, id int) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.RefreshPhone(ctx, p.CpID)
}

// Destroy 销毁中台实例并移除本地档案。
func (s *serviceImpl) Destroy(userID, id int) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := s.ops.Destroy(ctx, p.CpID); err != nil {
		return err
	}
	return s.repo.delete(userID, id)
}

func (s *serviceImpl) WebRTCAuth(userID, id int) (*midplat.WebRTCAuthInfo, error) {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.WebRTCAuth(ctx, p.CpID)
}

// FileList 列出云手机指定目录下的文件 / 子目录（文件管理面板浏览）。
func (s *serviceImpl) FileList(userID, id int, path string) ([]midplat.PhoneFile, error) {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.FileList(ctx, p.VmID, p.CpID, path)
}

// FileDownload 下载云手机上的单个文件，返回其二进制内容（文件管理面板下载）。
func (s *serviceImpl) FileDownload(userID, id int, path string) ([]byte, error) {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.FileDownload(ctx, p.VmID, p.CpID, path)
}

// FileDelete 删除云手机上的一个或多个文件（逐个透传中台 file-delete）。
func (s *serviceImpl) FileDelete(userID, id int, paths []string) error {
	if len(paths) == 0 {
		return apperr.BadRequest("未选择文件")
	}
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	for _, path := range paths {
		if path == "" {
			continue
		}
		if err := s.ops.FileDelete(ctx, p.VmID, p.CpID, path); err != nil {
			return err
		}
	}
	return nil
}

// FileUpload 把一个或多个本地文件上传到云手机的 folderPath（文件管理面板上传）。
func (s *serviceImpl) FileUpload(userID, id int, folderPath string, files []midplat.UploadFile) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return apperr.BadRequest("未选择文件")
	}
	// 中台 batch-upload 限制一次 1-10 个文件。
	if len(files) > 10 {
		return apperr.BadRequest("一次最多上传 10 个文件")
	}
	ctx, cancel := opCtxUpload()
	defer cancel()
	return s.ops.FileUpload(ctx, p.VmID, p.CpID, folderPath, files)
}

func (s *serviceImpl) WebRTCState(userID, id int) (bool, error) {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return false, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.WebRTCState(ctx, p.CpID)
}

func (s *serviceImpl) Screenshot(userID, id int, format string) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.Screenshot(ctx, p.CpID, format)
}

func (s *serviceImpl) SetVolume(userID, id, volume int) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.SetVolume(ctx, p.CpID, volume)
}

func (s *serviceImpl) Rotate(userID, id int, orientation string) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	if orientation != "landscape" && orientation != "portrait" {
		return apperr.BadRequest("orientation 只能是 landscape / portrait")
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.Rotate(ctx, p.CpID, orientation)
}

func (s *serviceImpl) Shake(userID, id int) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.Shake(ctx, p.CpID)
}

func (s *serviceImpl) InstalledApps(userID, id int) ([]midplat.InstalledApp, error) {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.InstalledApps(ctx, p.CpID)
}

func (s *serviceImpl) InstallApp(userID, id int, appIDs []int64) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	if len(appIDs) == 0 {
		return apperr.BadRequest("appIds 不能为空")
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.InstallApp(ctx, p.CpID, appIDs)
}

func (s *serviceImpl) UninstallApp(userID, id int, appIDs []int64, pkgs []string) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	if len(appIDs) == 0 && len(pkgs) == 0 {
		return apperr.BadRequest("appIds 与 packageNames 至少传一个")
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.UninstallApp(ctx, p.CpID, appIDs, pkgs)
}

func (s *serviceImpl) StartApp(userID, id int, appIDs []int64, pkgs []string) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.StartApp(ctx, p.CpID, appIDs, pkgs)
}

func (s *serviceImpl) StopApp(userID, id int, appIDs []int64, pkgs []string) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.StopApp(ctx, p.CpID, appIDs, pkgs)
}

func (s *serviceImpl) KillAllApps(userID, id int) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.KillAllApps(ctx, p.CpID)
}

// --- ADB（spec §3.1 operate / §3.2 whitelist / §2.16 page 连接信息）---

// AdbConnInfo 是返回给前台的 ADB 连接信息（含派生的 enabled）。
type AdbConnInfo struct {
	Enabled           bool   `json:"enabled"`
	AdbAddress        string `json:"adbAddress"`
	AdbToken          string `json:"adbToken"`
	AdbTokenExpiredAt string `json:"adbTokenExpiredAt"`
	Status            string `json:"status"`
}

// adbInfo 拉取中台连接信息并派生 enabled（有 token = 已开启）。
func (s *serviceImpl) adbInfo(ctx context.Context, cpID string) (*AdbConnInfo, error) {
	info, err := s.ops.AdbInfo(ctx, cpID)
	if err != nil {
		return nil, err
	}
	addr := info.AdbAddress
	return &AdbConnInfo{
		Enabled:           info.AdbToken != "",
		AdbAddress:        addr,
		AdbToken:          info.AdbToken,
		AdbTokenExpiredAt: info.AdbTokenExpiredAt,
		Status:            info.Status,
	}, nil
}

// AdbInfo 查询 ADB 连接信息（地址 / token / 过期时间 / 是否开启）。
func (s *serviceImpl) AdbInfo(userID, id int) (*AdbConnInfo, error) {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.adbInfo(ctx, p.CpID)
}

// AdbEnable 开启 ADB（可带白名单 IP 与 token 有效期秒数），成功后回查连接信息返回。
func (s *serviceImpl) AdbEnable(userID, id int, whiteIP []string, ttl int) (*AdbConnInfo, error) {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	res, err := s.ops.AdbOperate(ctx, p.CpID, "enable", whiteIP, ttl)
	if err != nil {
		return nil, err
	}
	if res != nil && !res.AllSuccess {
		return nil, apperr.Internal(adbErr(res, "开启 ADB 失败"))
	}
	return s.adbInfo(ctx, p.CpID)
}

// AdbDisable 关闭 ADB。
func (s *serviceImpl) AdbDisable(userID, id int) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	res, err := s.ops.AdbOperate(ctx, p.CpID, "disable", []string{}, 0)
	if err != nil {
		return err
	}
	if res != nil && !res.AllSuccess {
		return apperr.Internal(adbErr(res, "关闭 ADB 失败"))
	}
	return nil
}

// AdbUpdateWhitelist 更新 ADB 白名单 IP（覆盖式）。
func (s *serviceImpl) AdbUpdateWhitelist(userID, id int, whiteIP []string) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	res, err := s.ops.AdbOperate(ctx, p.CpID, "update_whitelist", whiteIP, 0)
	if err != nil {
		return err
	}
	if res != nil && !res.AllSuccess {
		return apperr.Internal(adbErr(res, "更新白名单失败"))
	}
	return nil
}

// AdbWhitelist 查询 ADB 白名单记录。
func (s *serviceImpl) AdbWhitelist(userID, id int) ([]midplat.AdbWhitelistEntry, error) {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.AdbWhitelist(ctx, p.CpID)
}

// adbErr 从 operate 结果里挑一条可读错误。
func adbErr(res *midplat.AdbOperateResult, fallback string) string {
	if res.ErrorMessage != "" {
		return res.ErrorMessage
	}
	for _, d := range res.ContainerDetails {
		if !d.Success && d.Error != "" {
			return d.Error
		}
	}
	return fallback
}

// --- 异步任务收敛（worker tick）---

// runDueTasks 处理所有未收敛任务：批量查中台实时状态，把云手机从过渡态收敛到稳定态。
//   - 创建/开机：中台 NORMAL → RUNNING（任务 succeeded）；失败态/超时 → CREATE_FAILED / STOPPED（timeout）。
//   - 销毁：中台 DESTROYED 或查不到（已消失）→ 删本地档案；超时也强制删本地（best-effort）。
//   - 其余：仍在等待，标记任务 running。
//
// 整个过程 best-effort：中台查询失败则本 tick 跳过，下个 tick 重试。
func (s *serviceImpl) runDueTasks(ctx context.Context) {
	if s.ops == nil {
		return
	}
	tasks, err := s.repo.dueTasks()
	if err != nil || len(tasks) == 0 {
		return
	}
	cpIDs := make([]string, 0, len(tasks))
	seen := map[string]bool{}
	for _, t := range tasks {
		if t.CpID != "" && !seen[t.CpID] {
			seen[t.CpID] = true
			cpIDs = append(cpIDs, t.CpID)
		}
	}
	statuses, err := s.ops.Statuses(ctx, cpIDs)
	if err != nil {
		return
	}
	now := time.Now()
	for _, t := range tasks {
		st, present := statuses[t.CpID]

		if t.Type == TaskTypeDestroy {
			// 销毁确认：中台已 DESTROYED 或实例已从列表消失 → 删本地档案；超时也强制删（已发过 destroy）。
			gone := !present || st == MidplatDestroyed
			switch {
			case gone:
				s.settleDestroy(t, TaskSucceeded, "")
			case now.After(t.Deadline):
				s.settleDestroy(t, TaskTimeout, "等待中台销毁超时（已强制移除本地档案）")
			default:
				if t.Status == TaskPending {
					_ = s.repo.updateTask(t.ID, TaskRunning, "")
				}
			}
			continue
		}

		switch {
		case st == midplatTarget(t.Type):
			s.settleTask(t, taskSuccessStatus(t.Type), TaskSucceeded, "")
		case isMidplatFailed(st):
			s.settleTask(t, taskFailureStatus(t.Type), TaskTimeout, "中台实例失败："+st)
		case now.After(t.Deadline):
			s.settleTask(t, taskFailureStatus(t.Type), TaskTimeout, "等待中台状态超时")
		default:
			if t.Status == TaskPending {
				_ = s.repo.updateTask(t.ID, TaskRunning, "")
			}
		}
	}
}

// midplatTarget 该任务期望收敛到的中台实时状态：关机 → STOPPED；创建/开机 → NORMAL。
func midplatTarget(taskType string) string {
	if taskType == TaskTypeStop {
		return MidplatStopped
	}
	return MidplatReady
}

// settleDestroy 销毁任务收敛：删本地档案 + 标记任务终态。
func (s *serviceImpl) settleDestroy(t CpTask, taskStatus, lastErr string) {
	_ = s.repo.delete(int(t.UserID), int(t.CloudPhoneID))
	_ = s.repo.updateTask(t.ID, taskStatus, lastErr)
}

// settleTask 收敛单个任务：先改云手机状态，再标记任务终态。
func (s *serviceImpl) settleTask(t CpTask, phoneStatus, taskStatus, lastErr string) {
	_ = s.repo.setStatus(t.CloudPhoneID, phoneStatus)
	_ = s.repo.updateTask(t.ID, taskStatus, lastErr)
}

// taskSuccessStatus 成功收敛态：关机 → STOPPED；创建/开机 → RUNNING
// （中台 cp/create 强制 autoStart=true，创建完成即自动开机，故创建成功也直接 RUNNING）。
func taskSuccessStatus(taskType string) string {
	if taskType == TaskTypeStop {
		return StatusStopped
	}
	return StatusRunning
}

// taskFailureStatus 失败/超时收敛态：创建 → CREATE_FAILED；开机/关机 → STOPPED。
func taskFailureStatus(taskType string) string {
	if taskType == TaskTypeCreate {
		return StatusCreateFailed
	}
	return StatusStopped
}
