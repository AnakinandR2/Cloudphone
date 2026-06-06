package app

import (
	"manager-backend/framework/apperr"
	"manager-backend/framework/midplat"
)

// AppService 模块内服务实例，由 module.Init 注入后装配。
var AppService *serviceImpl

type serviceImpl struct {
	repo repository
	ops  midplatPort
}

func newService(repo repository, ops midplatPort) *serviceImpl {
	return &serviceImpl{repo: repo, ops: ops}
}

// refreshStatus 用中台应用库按 md5 把一批本地应用的「创建中 → 正常」状态刷新并回写变化。
// 中台暂不可用时原样返回，不改状态。
func (s *serviceImpl) refreshStatus(apps []CustomerApp) []CustomerApp {
	if len(apps) == 0 || s.ops == nil {
		return apps
	}
	ctx, cancel := opCtx()
	defer cancel()
	resp, err := s.ops.ListApps(ctx, midplat.ListAppsRequest{Page: 1, PageSize: 500})
	if err != nil || resp == nil {
		return apps
	}
	byMD5 := make(map[string]midplat.AppInfo, len(resp.List))
	for _, a := range resp.List {
		byMD5[a.AppMD5] = a
	}
	for i := range apps {
		mid, ok := byMD5[apps[i].AppMD5]
		if !ok {
			// 不在中台列表 = 仍在创建中（中台异步处理）。
			if apps[i].Status != StatusCreating {
				apps[i].Status = StatusCreating
				_ = s.repo.update(&apps[i])
			}
			continue
		}
		// 命中中台 = 已就绪，转正常并回填可能变化的字段。
		changed := apps[i].Status != StatusNormal
		apps[i].Status = StatusNormal
		if mid.ID != 0 && apps[i].CpAppID != mid.ID {
			apps[i].CpAppID = mid.ID
			changed = true
		}
		if mid.IconPath != "" && apps[i].IconPath != mid.IconPath {
			apps[i].IconPath = mid.IconPath
			changed = true
		}
		if mid.Version != "" && apps[i].Version != mid.Version {
			apps[i].Version = mid.Version
			changed = true
		}
		if changed {
			_ = s.repo.update(&apps[i])
		}
	}
	return apps
}

// List 返回当前用户上传的应用（本地绑定），并按 md5 刷新「创建中 → 正常」状态。
func (s *serviceImpl) List(userID int) ([]CustomerApp, error) {
	apps, err := s.repo.listByUser(userID)
	if err != nil {
		return nil, err
	}
	return s.refreshStatus(apps), nil
}

// StoreList 返回应用商店应用（admin 上传，面向全部用户），并刷新创建状态。
// 同时服务于 my 的「应用市场」标签页与 admin 的应用商店管理页。
func (s *serviceImpl) StoreList() ([]CustomerApp, error) {
	apps, err := s.repo.listStore()
	if err != nil {
		return nil, err
	}
	return s.refreshStatus(apps), nil
}

// StoreUpload 由 admin 整链上传 APK 到中台应用库，并落一条「应用商店」绑定（Store=true，UserID=0）。
func (s *serviceImpl) StoreUpload(path string, opts midplat.UploadAppOptions) (*CustomerApp, error) {
	if s.ops == nil {
		return nil, apperr.Internal("云手机中台未配置")
	}
	ctx, cancel := opCtxUpload()
	defer cancel()
	created, err := s.ops.UploadAppFromFile(ctx, path, opts)
	if err != nil {
		return nil, err
	}
	rec := &CustomerApp{
		UserID:      0,
		Store:       true,
		CpAppID:     created.ID,
		AppMD5:      created.MD5,
		AppName:     created.AppName,
		PackageName: created.PackageName,
		Version:     created.Version,
		FileSize:    created.FileSize,
		IconPath:    created.IconPath,
		Status:      StatusCreating,
	}
	if err := s.repo.create(rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// StoreDelete 删除应用商店应用（按本地绑定 id，仅限 Store=true），并软删对应中台应用。
func (s *serviceImpl) StoreDelete(ids []int) error {
	if len(ids) == 0 {
		return apperr.BadRequest("未选择应用")
	}
	apps, err := s.repo.getStoreByIDs(ids)
	if err != nil {
		return err
	}
	if len(apps) == 0 {
		return apperr.NotFound("应用不存在")
	}
	cpIDs := make([]int64, 0, len(apps))
	localIDs := make([]int, 0, len(apps))
	for i := range apps {
		if apps[i].CpAppID > 0 {
			cpIDs = append(cpIDs, apps[i].CpAppID)
		}
		localIDs = append(localIDs, int(apps[i].ID))
	}
	if s.ops != nil && len(cpIDs) > 0 {
		ctx, cancel := opCtx()
		defer cancel()
		if err := s.ops.BatchDeleteApps(ctx, cpIDs); err != nil {
			return err
		}
	}
	return s.repo.deleteStoreByIDs(localIDs)
}

// Upload 整链上传到中台应用库，并落一条本地绑定（初始「创建中」）。
func (s *serviceImpl) Upload(userID int, path string, opts midplat.UploadAppOptions) (*CustomerApp, error) {
	if s.ops == nil {
		return nil, apperr.Internal("云手机中台未配置")
	}
	ctx, cancel := opCtxUpload()
	defer cancel()
	created, err := s.ops.UploadAppFromFile(ctx, path, opts)
	if err != nil {
		return nil, err
	}
	rec := &CustomerApp{
		UserID:      uint(userID),
		CpAppID:     created.ID,
		AppMD5:      created.MD5,
		AppName:     created.AppName,
		PackageName: created.PackageName,
		Version:     created.Version,
		FileSize:    created.FileSize,
		IconPath:    created.IconPath,
		Status:      StatusCreating,
	}
	if err := s.repo.create(rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// AdminList 运营查看全部用户上传的应用，并用中台列表按 md5 判定状态（只读，不写回）。
func (s *serviceImpl) AdminList() ([]AdminApp, error) {
	apps, err := s.repo.listAll()
	if err != nil || len(apps) == 0 || s.ops == nil {
		return apps, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	resp, err := s.ops.ListApps(ctx, midplat.ListAppsRequest{Page: 1, PageSize: 500})
	if err != nil || resp == nil {
		return apps, nil
	}
	have := make(map[string]bool, len(resp.List))
	for _, a := range resp.List {
		have[a.AppMD5] = true
	}
	for i := range apps {
		if have[apps[i].AppMD5] {
			apps[i].Status = StatusNormal
		} else {
			apps[i].Status = StatusCreating
		}
	}
	return apps, nil
}

// AdminBatchDelete 运营删除任意用户的应用（按本地绑定 id；不做归属校验），并软删中台应用。
func (s *serviceImpl) AdminBatchDelete(ids []int) error {
	if len(ids) == 0 {
		return apperr.BadRequest("未选择应用")
	}
	apps, err := s.repo.getAllByIDs(ids)
	if err != nil {
		return err
	}
	if len(apps) == 0 {
		return apperr.NotFound("应用不存在")
	}
	cpIDs := make([]int64, 0, len(apps))
	localIDs := make([]int, 0, len(apps))
	for i := range apps {
		if apps[i].CpAppID > 0 {
			cpIDs = append(cpIDs, apps[i].CpAppID)
		}
		localIDs = append(localIDs, int(apps[i].ID))
	}
	if s.ops != nil && len(cpIDs) > 0 {
		ctx, cancel := opCtx()
		defer cancel()
		if err := s.ops.BatchDeleteApps(ctx, cpIDs); err != nil {
			return err
		}
	}
	return s.repo.deleteAllByIDs(localIDs)
}

// BatchDelete 仅删除属于当前用户的本地绑定，并把对应中台应用一并软删。
func (s *serviceImpl) BatchDelete(userID int, ids []int) error {
	if len(ids) == 0 {
		return apperr.BadRequest("未选择应用")
	}
	owned, err := s.repo.getByIDs(userID, ids)
	if err != nil {
		return err
	}
	if len(owned) == 0 {
		return apperr.NotFound("应用不存在或不属于你")
	}
	cpIDs := make([]int64, 0, len(owned))
	localIDs := make([]int, 0, len(owned))
	for i := range owned {
		if owned[i].CpAppID > 0 {
			cpIDs = append(cpIDs, owned[i].CpAppID)
		}
		localIDs = append(localIDs, int(owned[i].ID))
	}
	if s.ops != nil && len(cpIDs) > 0 {
		ctx, cancel := opCtx()
		defer cancel()
		if err := s.ops.BatchDeleteApps(ctx, cpIDs); err != nil {
			return err
		}
	}
	return s.repo.deleteByIDs(userID, localIDs)
}
