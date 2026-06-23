package app

import (
	"context"
	"errors"
	"io"
	"testing"

	"manager-backend/framework"
	"manager-backend/framework/apperr"
	"manager-backend/framework/midplat"
)

// recordingPort 是可定制返回值/错误并记录调用的中台桩，用于 service 层副作用断言。
type recordingPort struct {
	listResp    *midplat.ListAppsResponse
	listErr     error
	created     *midplat.CreatedApp
	createErr   error
	batchErr    error
	batchCalled bool
	gotBatchIDs []int64
}

func (f *recordingPort) ListApps(context.Context, midplat.ListAppsRequest) (*midplat.ListAppsResponse, error) {
	return f.listResp, f.listErr
}
func (f *recordingPort) BatchDeleteApps(_ context.Context, ids []int64) error {
	f.batchCalled = true
	f.gotBatchIDs = ids
	return f.batchErr
}
func (f *recordingPort) UploadAppFromFile(context.Context, string, midplat.UploadAppOptions) (*midplat.CreatedApp, error) {
	return f.created, f.createErr
}
func (f *recordingPort) InitiateAppUpload(context.Context, midplat.InitiateUploadRequest) (*midplat.InitiateUploadResponse, error) {
	return &midplat.InitiateUploadResponse{}, nil
}
func (f *recordingPort) UploadPart(context.Context, int64, int, string, io.Reader, string) (*midplat.UploadPartResponse, error) {
	return &midplat.UploadPartResponse{}, nil
}
func (f *recordingPort) CompleteAppUpload(context.Context, int64) (string, error) { return "", nil }
func (f *recordingPort) GetAppInfoFromFile(context.Context, int64) (*midplat.ParsedAppInfo, error) {
	return &midplat.ParsedAppInfo{}, nil
}
func (f *recordingPort) CreateAppFromUploadedFile(_ context.Context, req midplat.CreateFromUploadedFileRequest) (*midplat.CreatedApp, error) {
	return f.created, f.createErr
}
func (f *recordingPort) QueryUploadStatus(context.Context, int64) (string, error) {
	return midplat.UploadStatusOSSSuccess, nil
}

func newRepoSvc(ops midplatPort) *serviceImpl { return newService(newRepository(framework.DB), ops) }

// ── firstNonEmpty ────────────────────────────────────────────────

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "  ", "x", "y"); got != "x" {
		t.Errorf("firstNonEmpty = %q, want x", got)
	}
	if got := firstNonEmpty("", "   "); got != "" {
		t.Errorf("firstNonEmpty all-blank = %q, want empty", got)
	}
	if got := firstNonEmpty("first"); got != "first" {
		t.Errorf("firstNonEmpty single = %q", got)
	}
}

// ── refreshStatus ────────────────────────────────────────────────

func TestRefreshStatus_NilOps_ReturnsAsIs(t *testing.T) {
	svc := newService(newRepository(framework.DB), nil)
	in := []CustomerApp{{AppMD5: "x", Status: StatusCreating}}
	out := svc.refreshStatus(in)
	if len(out) != 1 || out[0].Status != StatusCreating {
		t.Errorf("nil ops should return unchanged, got %+v", out)
	}
}

func TestRefreshStatus_Empty_ReturnsAsIs(t *testing.T) {
	svc := newRepoSvc(&recordingPort{})
	out := svc.refreshStatus(nil)
	if len(out) != 0 {
		t.Errorf("empty in → empty out, got %+v", out)
	}
}

func TestRefreshStatus_MidplatError_ReturnsAsIs(t *testing.T) {
	svc := newRepoSvc(&recordingPort{listErr: errors.New("midplat down")})
	in := []CustomerApp{{AppMD5: "y", Status: StatusCreating}}
	out := svc.refreshStatus(in)
	if out[0].Status != StatusCreating {
		t.Errorf("midplat error should leave status untouched, got %q", out[0].Status)
	}
}

func TestRefreshStatus_HitMarksNormalAndBackfills(t *testing.T) {
	// 落一条「创建中」记录，中台列表命中其 md5 → 转 NORMAL 并回填 icon/version/cpAppID。
	rec := &CustomerApp{UserID: 91000, AppMD5: "refresh-hit", Status: StatusCreating}
	if err := framework.DB.Create(rec).Error; err != nil {
		t.Fatal(err)
	}
	port := &recordingPort{listResp: &midplat.ListAppsResponse{List: []midplat.AppInfo{
		{ID: 555, AppMD5: "refresh-hit", IconPath: "/new.png", Version: "9.9"},
	}}}
	svc := newRepoSvc(port)

	out := svc.refreshStatus([]CustomerApp{*rec})
	if out[0].Status != StatusNormal {
		t.Errorf("status = %q, want NORMAL", out[0].Status)
	}
	if out[0].CpAppID != 555 || out[0].IconPath != "/new.png" || out[0].Version != "9.9" {
		t.Errorf("backfill wrong: %+v", out[0])
	}
	// 变化已写回 DB。
	var reloaded CustomerApp
	framework.DB.First(&reloaded, rec.ID)
	if reloaded.Status != StatusNormal || reloaded.CpAppID != 555 {
		t.Errorf("not persisted: %+v", reloaded)
	}
}

func TestRefreshStatus_MissMarksCreating(t *testing.T) {
	// 已是 NORMAL 但中台列表不含其 md5 → 退回 CREATING 并写回。
	rec := &CustomerApp{UserID: 91001, AppMD5: "refresh-miss", Status: StatusNormal}
	if err := framework.DB.Create(rec).Error; err != nil {
		t.Fatal(err)
	}
	port := &recordingPort{listResp: &midplat.ListAppsResponse{List: []midplat.AppInfo{
		{ID: 1, AppMD5: "some-other-md5"},
	}}}
	svc := newRepoSvc(port)

	out := svc.refreshStatus([]CustomerApp{*rec})
	if out[0].Status != StatusCreating {
		t.Errorf("miss should mark CREATING, got %q", out[0].Status)
	}
	var reloaded CustomerApp
	framework.DB.First(&reloaded, rec.ID)
	if reloaded.Status != StatusCreating {
		t.Errorf("creating not persisted: %+v", reloaded)
	}
}

// ── List / StoreList ─────────────────────────────────────────────

func TestList_FiltersByOwnerAndRefreshes(t *testing.T) {
	const owner = 91010
	framework.DB.Create(&CustomerApp{UserID: owner, AppMD5: "list-a", Status: StatusCreating})
	framework.DB.Create(&CustomerApp{UserID: 91011, AppMD5: "list-b"})
	port := &recordingPort{listResp: &midplat.ListAppsResponse{List: []midplat.AppInfo{
		{ID: 7, AppMD5: "list-a"},
	}}}
	svc := newRepoSvc(port)

	list, err := svc.List(owner)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].AppMD5 != "list-a" {
		t.Fatalf("owner filter wrong: %+v", list)
	}
	if list[0].Status != StatusNormal {
		t.Errorf("refresh not applied: %q", list[0].Status)
	}
}

func TestStoreList_OnlyStoreApps(t *testing.T) {
	framework.DB.Create(&CustomerApp{UserID: 0, Store: true, AppMD5: "store-list-a", Status: StatusCreating})
	framework.DB.Create(&CustomerApp{UserID: 91020, AppMD5: "store-list-user"})
	port := &recordingPort{listResp: &midplat.ListAppsResponse{}}
	svc := newRepoSvc(port)

	list, err := svc.StoreList()
	if err != nil {
		t.Fatalf("StoreList: %v", err)
	}
	for _, a := range list {
		if !a.Store {
			t.Errorf("non-store app leaked: %+v", a)
		}
	}
}

// ── Upload / StoreUpload ─────────────────────────────────────────

func TestUpload_NilOps_Internal(t *testing.T) {
	svc := newService(newRepository(framework.DB), nil)
	_, err := svc.Upload(1, "/tmp/x.apk", midplat.UploadAppOptions{})
	if apperr.KindOf(err) != apperr.KindInternal {
		t.Errorf("nil ops → Internal, got %v", err)
	}
}

func TestUpload_PersistsBinding(t *testing.T) {
	port := &recordingPort{created: &midplat.CreatedApp{
		ID: 808, MD5: "up-md5", AppName: "QQ", PackageName: "com.tencent.qq", Version: "1.0", FileSize: "10MB", IconPath: "/qq.png",
	}}
	svc := newRepoSvc(port)
	rec, err := svc.Upload(91030, "/tmp/x.apk", midplat.UploadAppOptions{})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if rec.ID == 0 || rec.UserID != 91030 || rec.Store {
		t.Errorf("binding wrong: %+v", rec)
	}
	if rec.CpAppID != 808 || rec.AppMD5 != "up-md5" || rec.Status != StatusCreating {
		t.Errorf("fields wrong: %+v", rec)
	}
}

func TestUpload_MidplatError_Propagates(t *testing.T) {
	port := &recordingPort{createErr: apperr.Internal("boom")}
	svc := newRepoSvc(port)
	if _, err := svc.Upload(1, "/tmp/x.apk", midplat.UploadAppOptions{}); err == nil {
		t.Fatal("expected upload error to propagate")
	}
}

func TestStoreUpload_NilOps_Internal(t *testing.T) {
	svc := newService(newRepository(framework.DB), nil)
	if _, err := svc.StoreUpload("/tmp/x.apk", midplat.UploadAppOptions{}); apperr.KindOf(err) != apperr.KindInternal {
		t.Errorf("nil ops → Internal, got %v", err)
	}
}

func TestStoreUpload_PersistsStoreBinding(t *testing.T) {
	port := &recordingPort{created: &midplat.CreatedApp{ID: 909, MD5: "store-up-md5", AppName: "Store"}}
	svc := newRepoSvc(port)
	rec, err := svc.StoreUpload("/tmp/s.apk", midplat.UploadAppOptions{})
	if err != nil {
		t.Fatalf("StoreUpload: %v", err)
	}
	if !rec.Store || rec.UserID != 0 || rec.CpAppID != 909 || rec.Status != StatusCreating {
		t.Errorf("store binding wrong: %+v", rec)
	}
}

// ── BatchDelete (owner) ──────────────────────────────────────────

func TestBatchDelete_EmptyIDs_BadRequest(t *testing.T) {
	svc := newRepoSvc(&recordingPort{})
	if err := svc.BatchDelete(1, nil); apperr.KindOf(err) != apperr.KindBadRequest {
		t.Errorf("empty ids → BadRequest, got %v", err)
	}
}

func TestBatchDelete_NotFound(t *testing.T) {
	svc := newRepoSvc(&recordingPort{})
	if err := svc.BatchDelete(91040, []int{99999999}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Errorf("no owned → NotFound, got %v", err)
	}
}

func TestBatchDelete_OwnerScoped_DeletesAndSyncsMidplat(t *testing.T) {
	const owner = 91050
	rec := &CustomerApp{UserID: owner, CpAppID: 7001, AppMD5: "bd-own"}
	framework.DB.Create(rec)
	other := &CustomerApp{UserID: 91051, CpAppID: 7002, AppMD5: "bd-other"}
	framework.DB.Create(other)

	port := &recordingPort{}
	svc := newRepoSvc(port)
	// 即便把他人 id 也传进来，也只删自己的。
	if err := svc.BatchDelete(owner, []int{int(rec.ID), int(other.ID)}); err != nil {
		t.Fatalf("BatchDelete: %v", err)
	}
	if !port.batchCalled || len(port.gotBatchIDs) != 1 || port.gotBatchIDs[0] != 7001 {
		t.Errorf("midplat batch delete should only include owned cpID, got %+v", port.gotBatchIDs)
	}
	var cnt int64
	framework.DB.Model(&CustomerApp{}).Where("id = ?", rec.ID).Count(&cnt)
	if cnt != 0 {
		t.Errorf("owned record not deleted")
	}
	framework.DB.Model(&CustomerApp{}).Where("id = ?", other.ID).Count(&cnt)
	if cnt != 1 {
		t.Errorf("other user's record wrongly deleted")
	}
}

func TestBatchDelete_MidplatError_AbortsBeforeLocalDelete(t *testing.T) {
	const owner = 91060
	rec := &CustomerApp{UserID: owner, CpAppID: 7100, AppMD5: "bd-mperr"}
	framework.DB.Create(rec)
	svc := newRepoSvc(&recordingPort{batchErr: errors.New("midplat reject")})
	if err := svc.BatchDelete(owner, []int{int(rec.ID)}); err == nil {
		t.Fatal("expected midplat error")
	}
	var cnt int64
	framework.DB.Model(&CustomerApp{}).Where("id = ?", rec.ID).Count(&cnt)
	if cnt != 1 {
		t.Errorf("local record should remain when midplat delete failed")
	}
}

func TestBatchDelete_NilOps_StillDeletesLocal(t *testing.T) {
	const owner = 91070
	rec := &CustomerApp{UserID: owner, CpAppID: 7200, AppMD5: "bd-nilops"}
	framework.DB.Create(rec)
	svc := newService(newRepository(framework.DB), nil)
	if err := svc.BatchDelete(owner, []int{int(rec.ID)}); err != nil {
		t.Fatalf("BatchDelete nil ops: %v", err)
	}
	var cnt int64
	framework.DB.Model(&CustomerApp{}).Where("id = ?", rec.ID).Count(&cnt)
	if cnt != 0 {
		t.Errorf("local record not deleted when ops nil")
	}
}

// ── AdminBatchDelete ─────────────────────────────────────────────

func TestAdminBatchDelete_EmptyAndNotFound(t *testing.T) {
	svc := newRepoSvc(&recordingPort{})
	if err := svc.AdminBatchDelete(nil); apperr.KindOf(err) != apperr.KindBadRequest {
		t.Errorf("empty → BadRequest, got %v", err)
	}
	if err := svc.AdminBatchDelete([]int{99999998}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Errorf("missing → NotFound, got %v", err)
	}
}

func TestAdminBatchDelete_DeletesUserAppsNotStore(t *testing.T) {
	userApp := &CustomerApp{UserID: 91080, CpAppID: 8001, AppMD5: "abd-user"}
	framework.DB.Create(userApp)
	storeApp := &CustomerApp{UserID: 0, Store: true, CpAppID: 8002, AppMD5: "abd-store"}
	framework.DB.Create(storeApp)

	port := &recordingPort{}
	svc := newRepoSvc(port)
	// 传入用户应用 + 商店应用 id；getAllByIDs 会过滤掉商店应用 → 只删用户应用。
	if err := svc.AdminBatchDelete([]int{int(userApp.ID), int(storeApp.ID)}); err != nil {
		t.Fatalf("AdminBatchDelete: %v", err)
	}
	if len(port.gotBatchIDs) != 1 || port.gotBatchIDs[0] != 8001 {
		t.Errorf("should only delete user app cpID, got %+v", port.gotBatchIDs)
	}
	var cnt int64
	framework.DB.Model(&CustomerApp{}).Where("id = ?", storeApp.ID).Count(&cnt)
	if cnt != 1 {
		t.Errorf("store app wrongly deleted by AdminBatchDelete")
	}
}

// ── StoreDelete ──────────────────────────────────────────────────

func TestStoreDelete_EmptyAndNotFound(t *testing.T) {
	svc := newRepoSvc(&recordingPort{})
	if err := svc.StoreDelete(nil); apperr.KindOf(err) != apperr.KindBadRequest {
		t.Errorf("empty → BadRequest, got %v", err)
	}
	if err := svc.StoreDelete([]int{99999997}); apperr.KindOf(err) != apperr.KindNotFound {
		t.Errorf("missing → NotFound, got %v", err)
	}
}

func TestStoreDelete_OnlyStoreApps(t *testing.T) {
	storeApp := &CustomerApp{UserID: 0, Store: true, CpAppID: 8101, AppMD5: "sd-store"}
	framework.DB.Create(storeApp)
	userApp := &CustomerApp{UserID: 91090, CpAppID: 8102, AppMD5: "sd-user"}
	framework.DB.Create(userApp)

	port := &recordingPort{}
	svc := newRepoSvc(port)
	if err := svc.StoreDelete([]int{int(storeApp.ID), int(userApp.ID)}); err != nil {
		t.Fatalf("StoreDelete: %v", err)
	}
	if len(port.gotBatchIDs) != 1 || port.gotBatchIDs[0] != 8101 {
		t.Errorf("should only delete store app cpID, got %+v", port.gotBatchIDs)
	}
	var cnt int64
	framework.DB.Model(&CustomerApp{}).Where("id = ?", userApp.ID).Count(&cnt)
	if cnt != 1 {
		t.Errorf("user app wrongly deleted by StoreDelete")
	}
}

// ── AdminList ────────────────────────────────────────────────────

func TestAdminList_MarksStatusFromMidplat(t *testing.T) {
	known := &CustomerApp{UserID: 91100, AppMD5: "al-known", Status: StatusCreating}
	framework.DB.Create(known)
	unknown := &CustomerApp{UserID: 91100, AppMD5: "al-unknown", Status: StatusNormal}
	framework.DB.Create(unknown)

	port := &recordingPort{listResp: &midplat.ListAppsResponse{List: []midplat.AppInfo{
		{ID: 1, AppMD5: "al-known"},
	}}}
	svc := newRepoSvc(port)
	list, err := svc.AdminList()
	if err != nil {
		t.Fatalf("AdminList: %v", err)
	}
	var gotKnown, gotUnknown *AdminApp
	for i := range list {
		if list[i].AppMD5 == "al-known" {
			gotKnown = &list[i]
		}
		if list[i].AppMD5 == "al-unknown" {
			gotUnknown = &list[i]
		}
	}
	if gotKnown == nil || gotKnown.Status != StatusNormal {
		t.Errorf("known md5 should be NORMAL, got %+v", gotKnown)
	}
	if gotUnknown == nil || gotUnknown.Status != StatusCreating {
		t.Errorf("unknown md5 should be CREATING, got %+v", gotUnknown)
	}
}

func TestAdminList_MidplatErr_ReturnsRaw(t *testing.T) {
	framework.DB.Create(&CustomerApp{UserID: 91110, AppMD5: "al-err", Status: StatusCreating})
	svc := newRepoSvc(&recordingPort{listErr: errors.New("down")})
	list, err := svc.AdminList()
	if err != nil {
		t.Fatalf("AdminList should swallow midplat error: %v", err)
	}
	if len(list) == 0 {
		t.Errorf("expected raw list returned")
	}
}

// ── CreateFromUpload nil-created fallback ─────────────────────────

func TestCreateFromUpload_NilCreated_FallsBackToReq(t *testing.T) {
	port := &recordingPort{created: nil} // 中台返回 nil → 内部回退到 &CreatedApp{}
	svc := newRepoSvc(port)
	rec, err := svc.CreateFromUpload(91120, midplat.CreateFromUploadedFileRequest{
		AppName: "fromreq", PackageName: "pkg.req", Version: "0.1", MD5: "reqmd5only", FileSize: "1MB", IconPath: "/r.png",
	})
	if err != nil {
		t.Fatalf("CreateFromUpload: %v", err)
	}
	if rec.AppName != "fromreq" || rec.AppMD5 != "reqmd5only" || rec.CpAppID != 0 {
		t.Errorf("nil created fallback wrong: %+v", rec)
	}
}

func TestCreateFromUpload_NilOps_Internal(t *testing.T) {
	svc := newService(newRepository(framework.DB), nil)
	if _, err := svc.CreateFromUpload(1, midplat.CreateFromUploadedFileRequest{}); apperr.KindOf(err) != apperr.KindInternal {
		t.Errorf("nil ops → Internal, got %v", err)
	}
}

func TestCreateFromUpload_MidplatErr_Propagates(t *testing.T) {
	svc := newRepoSvc(&recordingPort{createErr: errors.New("create failed")})
	if _, err := svc.CreateFromUpload(1, midplat.CreateFromUploadedFileRequest{}); err == nil {
		t.Fatal("expected create error to propagate")
	}
}

// ── upload pipeline nil-ops guards ───────────────────────────────

func TestUploadPipeline_NilOps_AllInternal(t *testing.T) {
	svc := newService(newRepository(framework.DB), nil)
	if _, err := svc.InitiateUpload(midplat.InitiateUploadRequest{}); apperr.KindOf(err) != apperr.KindInternal {
		t.Errorf("InitiateUpload nil ops")
	}
	if _, err := svc.UploadPart(1, 1, "", nil, "f"); apperr.KindOf(err) != apperr.KindInternal {
		t.Errorf("UploadPart nil ops")
	}
	if _, err := svc.CompleteUpload(1); apperr.KindOf(err) != apperr.KindInternal {
		t.Errorf("CompleteUpload nil ops")
	}
	if _, err := svc.ParseUpload(1); apperr.KindOf(err) != apperr.KindInternal {
		t.Errorf("ParseUpload nil ops")
	}
	if _, err := svc.QueryUploadStatus(1); apperr.KindOf(err) != apperr.KindInternal {
		t.Errorf("QueryUploadStatus nil ops")
	}
}

func TestUploadPipeline_OpsOK(t *testing.T) {
	svc := newRepoSvc(&recordingPort{})
	if _, err := svc.InitiateUpload(midplat.InitiateUploadRequest{FileName: "x"}); err != nil {
		t.Errorf("InitiateUpload: %v", err)
	}
	if _, err := svc.UploadPart(1, 1, "md5", nil, "f"); err != nil {
		t.Errorf("UploadPart: %v", err)
	}
	if _, err := svc.CompleteUpload(1); err != nil {
		t.Errorf("CompleteUpload: %v", err)
	}
	if _, err := svc.ParseUpload(1); err != nil {
		t.Errorf("ParseUpload: %v", err)
	}
	if st, err := svc.QueryUploadStatus(1); err != nil || st != midplat.UploadStatusOSSSuccess {
		t.Errorf("QueryUploadStatus: st=%q err=%v", st, err)
	}
}
