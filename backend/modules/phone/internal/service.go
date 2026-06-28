package phone

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
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
	s.enrichParallel(items)
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
	case "DISTROYING", "DESTROYING", "DESTROYED": // 中台 wire 协议拼写为 DISTROYING；已销毁也展示为销毁中
		return StatusDestroying
	default:
		return m // 其它中台态（REBOOTING/RESETTING/FAULTED…）原样透出，保持与中台一致
	}
}

// resolveLiveStatuses 把「已开通（有 cpId）」云手机的展示状态改为中台实时态（只改返回值，不写库）。
// 这是所有用户可见读的统一入口：中台查询失败 / 该 cp 不在返回里 → UNKNOWN（不回退本地档案值）；
// 未开通（无 cpId）→ 保留本地 CREATED/CREATE_FAILED（中台无可查）。
func (s *serviceImpl) resolveLiveStatuses(items []CloudPhone) {
	if s.ops == nil {
		return // 本地降级无中台：均为未开通(无 cpId)，保留本地态
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
	queryFailed := err != nil
	for i := range items {
		if items[i].CpID == "" {
			continue // 未开通：保留本地态
		}
		if queryFailed {
			items[i].Status = StatusUnknown
			continue
		}
		raw, ok := statuses[items[i].CpID]
		if !ok || raw == "" {
			items[i].Status = StatusUnknown // 中台查不到该 cp
			continue
		}
		items[i].Status = mapMidplatStatus(raw)
	}
}

// enrichAdb 批量标记「已开 ADB」（adbToken 非空），供列表/卡片显示安卓图标。
// best-effort：中台未配置/查询失败时静默保留 false。
func (s *serviceImpl) enrichAdb(items []CloudPhone) {
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
	enabled, err := s.ops.AdbEnabledMap(ctx, cpIDs)
	if err != nil {
		return
	}
	for i := range items {
		if items[i].CpID != "" && enabled[items[i].CpID] {
			items[i].AdbEnabled = true
		}
	}
}

// enrichRoot 批量标记「已 root」（isRooted），供列表/卡片显示 + 前端选对的 root 开关动作。
// best-effort：中台未配置/查询失败时静默保留 false。
func (s *serviceImpl) enrichRoot(items []CloudPhone) {
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
	rooted, err := s.ops.RootEnabledMap(ctx, cpIDs)
	if err != nil {
		return
	}
	for i := range items {
		if items[i].CpID != "" && rooted[items[i].CpID] {
			items[i].Rooted = true
		}
	}
}

// enrichParallel 并行拉取三个中台 enrichment（状态/ADB/Root），串行 apply 到 items。
// 各 goroutine 仅写各自局部变量，wg.Wait() 后统一写 items，无数据竞争。
func (s *serviceImpl) enrichParallel(items []CloudPhone) {
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

	var (
		statuses     map[string]string
		statusFailed bool
		adbMap       map[string]bool
		rootMap      map[string]bool
	)

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		ctx, cancel := opCtx()
		defer cancel()
		m, err := s.ops.Statuses(ctx, cpIDs)
		if err != nil {
			statusFailed = true
			return
		}
		statuses = m
	}()
	go func() {
		defer wg.Done()
		ctx, cancel := opCtx()
		defer cancel()
		m, err := s.ops.AdbEnabledMap(ctx, cpIDs)
		if err != nil {
			return
		}
		adbMap = m
	}()
	go func() {
		defer wg.Done()
		ctx, cancel := opCtx()
		defer cancel()
		m, err := s.ops.RootEnabledMap(ctx, cpIDs)
		if err != nil {
			return
		}
		rootMap = m
	}()
	wg.Wait()

	for i := range items {
		cp := &items[i]
		if cp.CpID == "" {
			continue // 未开通：保留本地态
		}
		if statusFailed {
			cp.Status = StatusUnknown
		} else if raw, ok := statuses[cp.CpID]; !ok || raw == "" {
			cp.Status = StatusUnknown
		} else {
			cp.Status = mapMidplatStatus(raw)
		}
		if adbMap != nil && adbMap[cp.CpID] {
			cp.AdbEnabled = true
		}
		if rootMap != nil && rootMap[cp.CpID] {
			cp.Rooted = true
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

// GetByIDDisplay 详情/单台用户读：状态走中台实时（查不到→UNKNOWN）。内部逻辑仍用 GetByID（本地态）。
func (s *serviceImpl) GetByIDDisplay(userID, id int) (*CloudPhone, error) {
	item, err := s.GetByID(userID, id)
	if err != nil {
		return nil, err
	}
	one := []CloudPhone{*item}
	s.resolveLiveStatuses(one)
	return &one[0], nil
}

// liveStatus 查单台云手机的中台实时展示态；中台不可用/查不到 → UNKNOWN。供操作门禁用。
func (s *serviceImpl) liveStatus(ctx context.Context, cpID string) string {
	if s.ops == nil || cpID == "" {
		return StatusUnknown
	}
	statuses, err := s.ops.Statuses(ctx, []string{cpID})
	if err != nil {
		return StatusUnknown
	}
	raw, ok := statuses[cpID]
	if !ok || raw == "" {
		return StatusUnknown
	}
	return mapMidplatStatus(raw)
}

// Create 创建一台云手机。
//   - 已配置中台：调中台异步创建 → 落 CREATING + 创建任务，worker 轮询收敛到 CREATED/CREATE_FAILED。
//   - 未配置中台（本地/测试降级）：仅落本地档案，直接置 CREATED，不触达中台、不建任务。
func (s *serviceImpl) Create(userID int, req *CloudPhoneCreate) (*CloudPhone, error) {
	// 席位前置校验（新模型）：当前非回收实例数 < 未过期 seat 容量才允许创建，保证不超额。
	if err := s.checkSeatAvailable(userID); err != nil {
		return nil, err
	}
	item := CloudPhone{
		UserID:  uint(userID),
		Name:    req.Name,
		ImageID: req.ImageID,
		ProxyID: req.ProxyID,
		Remark:  req.Remark,
	}
	if s.ops == nil {
		item.Status = StatusCreated
		if err := s.repo.create(&item); err != nil {
			return nil, err
		}
		return &item, nil
	}

	ctx, cancel := opCtx()
	defer cancel()
	res, err := s.ops.Create(ctx, CreateArgs{ImageID: req.ImageID})
	if err != nil {
		return nil, apperr.Internal("创建云手机失败：" + err.Error())
	}
	item.Status = StatusCreating
	item.CpID = res.CpID
	item.VmID = res.VmID
	if res.ImageID != "" {
		item.ImageID = res.ImageID
	}
	if err := s.repo.create(&item); err != nil {
		return nil, err
	}
	// 创建后触发席位 reconcile：物化新实例的席位占用（best-effort，巡检兜底）。
	_ = s.reconcileSeats(ctx, userID)
	// 入库创建任务：needStart=false，worker 轮询中台直到 cpId = STOPPED（已创建未开机）→ CREATED；超时 → CREATE_FAILED。
	_ = s.repo.createTask(&CpTask{
		UserID:        uint(userID),
		CloudPhoneID:  item.ID,
		CpID:          res.CpID,
		Type:          TaskTypeCreate,
		ExpectedState: MidplatStopped,
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
	return s.GetByIDDisplay(userID, id)
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
	// 门禁按中台实时态判（与 UI 显示同源）：仅 STOPPED/CREATED/CREATE_FAILED 可销毁。
	if p.CpID != "" && s.ops != nil {
		ctx, cancel := opCtx()
		live := s.liveStatus(ctx, p.CpID)
		cancel()
		switch live {
		case StatusStopped, StatusCreated, StatusCreateFailed:
			// 可销毁
		case StatusUnknown:
			return apperr.Validation("状态未知，请稍后重试")
		default:
			return apperr.Validation("当前状态不可销毁，请先停止")
		}
	}
	if p.CpID == "" || s.ops == nil {
		if err := s.repo.delete(userID, id); err != nil {
			return err
		}
		if p.CpID != "" {
			_ = billing.ReleaseInstanceOccupancy(userID, []string{p.CpID})
		}
		// 删除后触发 reconcile：席位释放后池中可能有溢出实例可被重新覆盖。
		ctx, cancel := opCtx()
		_ = s.reconcileSeats(ctx, userID)
		cancel()
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
	s.resolveLiveStatuses(items)
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

	// 门禁按中台实时态判（与 UI 显示同源），UNKNOWN 一律拒绝。
	live := s.liveStatus(ctx, p.CpID)
	if live == StatusUnknown {
		return apperr.Validation("状态未知，请稍后重试")
	}

	if operation == "开机" {
		if live != StatusCreated && live != StatusStopped {
			return apperr.Validation("当前状态不可开机")
		}
		// 开机前置校验①：必须已绑定代理（proxy_id>0）。未绑代理一律禁止开机（业务硬约束）。
		if p.ProxyID == 0 {
			return apperr.Validation("未绑定代理，无法开机，请先绑定代理")
		}
		// 开机前置校验②（新模型 §3.3）：必须有空闲包月名额或临时时长>0，否则禁止开机。
		canBoot, err := billing.CanBoot(userID)
		if err != nil {
			return err
		}
		if !canBoot {
			return apperr.Forbidden("没有可用的包月开机名额或临时开机时长，无法开机，请购买包月开机数或临时时长")
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
	if live != StatusRunning {
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
	if err := s.repo.delete(userID, id); err != nil {
		return err
	}
	if p.CpID != "" {
		_ = billing.ReleaseInstanceOccupancy(userID, []string{p.CpID})
	}
	_ = s.reconcileSeats(ctx, userID)
	return nil
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

// FileDelete 删除云手机上的一个或多个文件（并行透传中台 file-delete，返回首个错误）。
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
	eg, ctx := errgroup.WithContext(ctx)
	for _, path := range paths {
		if path == "" {
			continue
		}
		path := path
		eg.Go(func() error {
			return s.ops.FileDelete(ctx, p.VmID, p.CpID, path)
		})
	}
	return eg.Wait()
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

// --- ADB Token 接管（spec v3.25.9 §3.5）。enable 签发 token，disable 吊销，§2.6 取连接信息 ---

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

// AdbEnable 开启 ADB（签发 Token）。token 有效期由中台后台配置固定（实测 +60 天，validTime 不生效）。
// 「续期」复用本方法：再次 enable 即签发全新 token（旧 token 失效）。
//
// 连接地址 / token 以 enable 响应为准（§3.5.1 的 adb_address / login_code 最可靠）；
// 过期时间 enable 不返回，best-effort 再查 §2.6 补上。
func (s *serviceImpl) AdbEnable(userID, id int) (*AdbConnInfo, error) {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	ct, err := s.ops.AdbEnableToken(ctx, p.CpID)
	if err != nil {
		return nil, err
	}
	if ct == nil || ct.LoginCode == "" {
		return nil, apperr.Internal("开启 ADB 失败：中台未返回登录码")
	}
	out := &AdbConnInfo{
		Enabled:    true,
		AdbAddress: ct.AdbAddress,
		AdbToken:   ct.LoginCode,
	}
	// 过期时间 / 状态 best-effort 从 §2.6 补全（失败不影响开启结果）。
	if info, ierr := s.ops.AdbInfo(ctx, p.CpID); ierr == nil && info != nil {
		out.AdbTokenExpiredAt = info.AdbTokenExpiredAt
		out.Status = info.Status
		if out.AdbAddress == "" {
			out.AdbAddress = info.AdbAddress
		}
	}
	return out, nil
}

// RuntimeInfo 远控真实开机时长（spec 2026-06-17）：服务端按中台运行日志算开机秒数，前端只读累加。
type RuntimeInfo struct {
	Running       bool   `json:"running"`
	PowerOnAt     string `json:"power_on_at,omitempty"` // RFC3339，仅 running 时给
	UptimeSeconds int64  `json:"uptime_seconds"`        // 服务端算 now-powerOnAt，clamp ≥0
}

// Runtime 取某台云手机当前运行会话的真实开机时长：实时查中台运行日志最新一条，
// 运行中则由服务端算 now-powerOnAt（规避客户端时区/时钟偏差）；无日志/最新已关机/解析失败 → running=false。
func (s *serviceImpl) Runtime(userID, id int) (*RuntimeInfo, error) {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	page, err := s.ops.RunLogs(ctx, p.CpID, 1, 1)
	if err != nil {
		return nil, err
	}
	if page == nil || len(page.Data) == 0 {
		return &RuntimeInfo{Running: false}, nil
	}
	e := page.Data[0]
	on, ok := parseRunLogTime(e.PowerOnTime)
	if !ok {
		return &RuntimeInfo{Running: false}, nil
	}
	if _, ended := parseRunLogTime(e.PowerOffTime); ended {
		return &RuntimeInfo{Running: false}, nil // 最新会话已关机
	}
	up := int64(time.Since(on).Seconds())
	if up < 0 {
		up = 0
	}
	return &RuntimeInfo{Running: true, PowerOnAt: on.Format(time.RFC3339), UptimeSeconds: up}, nil
}

// RunLogs 分页查询某台云手机的运行会话日志（spec §2.9）。size 限定 1..100。
func (s *serviceImpl) RunLogs(userID, id, page, size int) (*midplat.RunLogPage, error) {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.RunLogs(ctx, p.CpID, page, size)
}

// AdbDisable 关闭 ADB（吊销 Token，中台幂等）。
func (s *serviceImpl) AdbDisable(userID, id int) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()
	return s.ops.AdbDisableToken(ctx, p.CpID)
}

// OwnedCpIDs 返回某用户名下已开通的全部 cpId（跨模块归属校验门面用）。
func (s *serviceImpl) OwnedCpIDs(userID int) ([]string, error) {
	return s.repo.ownedCpIDs(userID)
}

// OwnsCpID 校验某 cpId 是否属于该用户且已开通。
func (s *serviceImpl) OwnsCpID(userID int, cpID string) (bool, error) {
	if cpID == "" {
		return false, nil
	}
	ids, err := s.repo.ownedCpIDs(userID)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		if id == cpID {
			return true, nil
		}
	}
	return false, nil
}

// Root 开启 / 关闭云手机 root 权限（§3.4.1 update-root）。
//   - 前置：设备须 NORMAL（已开机）；开启要求当前未 root，关闭要求当前已 root。
//   - 同步生效（~8s），不重启、不丢运行态；不建异步任务。
func (s *serviceImpl) Root(userID, id int, enable bool) error {
	p, err := s.resolveCp(userID, id)
	if err != nil {
		return err
	}
	ctx, cancel := opCtx()
	defer cancel()

	// 门禁按中台实时态判（与 UI 同源），UNKNOWN 一律拒绝；root 仅在已开机时可切换。
	live := s.liveStatus(ctx, p.CpID)
	if live == StatusUnknown {
		return apperr.Validation("状态未知，请稍后重试")
	}
	if live != StatusRunning {
		return apperr.Validation("请先开机后再切换 Root")
	}

	// 前置校验：避免对已是目标态的设备误调（中台会报「未 root/已 root」）。best-effort，查不到则放行交由中台判。
	if rooted, rerr := s.ops.RootEnabledMap(ctx, []string{p.CpID}); rerr == nil {
		if cur, ok := rooted[p.CpID]; ok {
			if enable && cur {
				return apperr.Validation("该云手机已开启 Root")
			}
			if !enable && !cur {
				return apperr.Validation("该云手机未开启 Root")
			}
		}
	}

	return s.ops.Root(ctx, p.VmID, p.CpID, enable)
}

// --- 异步任务收敛（worker tick）---

// runDueTasks 处理所有未收敛任务：批量查中台实时状态，把云手机从过渡态收敛到稳定态。
//   - 创建（needStart=false）：中台 STOPPED → CREATED；开机：中台 NORMAL → RUNNING；失败态/超时 → CREATE_FAILED / STOPPED（timeout）。
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
	// 创建后不自动开机（needStart=false）→ 收敛信号为「已创建且关机」STOPPED；关机同理。
	if taskType == TaskTypeStop || taskType == TaskTypeCreate {
		return MidplatStopped
	}
	return MidplatReady
}

// settleDestroy 销毁任务收敛：删本地档案 + 释放席位占用 + reconcile + 标记任务终态。
func (s *serviceImpl) settleDestroy(t CpTask, taskStatus, lastErr string) {
	if err := s.repo.delete(int(t.UserID), int(t.CloudPhoneID)); err == nil {
		if t.CpID != "" {
			_ = billing.ReleaseInstanceOccupancy(int(t.UserID), []string{t.CpID})
		}
		ctx, cancel := opCtx()
		_ = s.reconcileSeats(ctx, int(t.UserID))
		cancel()
	}
	_ = s.repo.updateTask(t.ID, taskStatus, lastErr)
}

// settleTask 收敛单个任务：先改云手机状态，再标记任务终态。
func (s *serviceImpl) settleTask(t CpTask, phoneStatus, taskStatus, lastErr string) {
	_ = s.repo.setStatus(t.CloudPhoneID, phoneStatus)
	_ = s.repo.updateTask(t.ID, taskStatus, lastErr)
}

// taskSuccessStatus 成功收敛态：关机 → STOPPED；创建（needStart=false，不自动开机）→ CREATED；开机 → RUNNING。
func taskSuccessStatus(taskType string) string {
	switch taskType {
	case TaskTypeStop:
		return StatusStopped
	case TaskTypeCreate:
		return StatusCreated // 创建完成且未开机 → CREATED（可开机/可销毁）
	default:
		return StatusRunning
	}
}

// taskFailureStatus 失败/超时收敛态：创建 → CREATE_FAILED；开机/关机 → STOPPED。
func taskFailureStatus(taskType string) string {
	if taskType == TaskTypeCreate {
		return StatusCreateFailed
	}
	return StatusStopped
}
