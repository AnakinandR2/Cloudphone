package app

import (
	"context"
	"io"
	"testing"

	"manager-backend/framework"
	"manager-backend/framework/midplat"
)

// fakeUploadPort 是只为分片流水线测试准备的中台桩：CreateAppFromUploadedFile 可定制返回，
// 其余方法返回零值即可（CreateFromUpload 的本地落库逻辑才是被测点）。
type fakeUploadPort struct {
	created  *midplat.CreatedApp
	gotCreis midplat.CreateFromUploadedFileRequest
}

func (f *fakeUploadPort) ListApps(context.Context, midplat.ListAppsRequest) (*midplat.ListAppsResponse, error) {
	return &midplat.ListAppsResponse{}, nil
}
func (f *fakeUploadPort) BatchDeleteApps(context.Context, []int64) error { return nil }
func (f *fakeUploadPort) UploadAppFromFile(context.Context, string, midplat.UploadAppOptions) (*midplat.CreatedApp, error) {
	return &midplat.CreatedApp{}, nil
}
func (f *fakeUploadPort) InitiateAppUpload(context.Context, midplat.InitiateUploadRequest) (*midplat.InitiateUploadResponse, error) {
	return &midplat.InitiateUploadResponse{}, nil
}
func (f *fakeUploadPort) UploadPart(context.Context, int64, int, string, io.Reader, string) (*midplat.UploadPartResponse, error) {
	return &midplat.UploadPartResponse{}, nil
}
func (f *fakeUploadPort) CompleteAppUpload(context.Context, int64) (string, error) { return "", nil }
func (f *fakeUploadPort) GetAppInfoFromFile(context.Context, int64) (*midplat.ParsedAppInfo, error) {
	return &midplat.ParsedAppInfo{}, nil
}
func (f *fakeUploadPort) CreateAppFromUploadedFile(_ context.Context, req midplat.CreateFromUploadedFileRequest) (*midplat.CreatedApp, error) {
	f.gotCreis = req
	return f.created, nil
}
func (f *fakeUploadPort) QueryUploadStatus(context.Context, int64) (string, error) {
	return midplat.UploadStatusOSSSuccess, nil
}

func TestCreateFromUpload_PersistsLocalRecord_PreferMidplatFields(t *testing.T) {
	port := &fakeUploadPort{created: &midplat.CreatedApp{
		ID: 42, AppName: "微信", PackageName: "com.tencent.mm", Version: "8.0", MD5: "abc123", FileSize: "256MB", IconPath: "/icon.png",
	}}
	svc := newService(newRepository(framework.DB), port)

	rec, err := svc.CreateFromUpload(7, midplat.CreateFromUploadedFileRequest{
		UploadID: 100, AppName: "fallback", MD5: "reqmd5",
	})
	if err != nil {
		t.Fatalf("CreateFromUpload error: %v", err)
	}
	if rec.ID == 0 {
		t.Fatalf("expected local record persisted with an ID")
	}
	if rec.UserID != 7 || rec.Store {
		t.Errorf("owner wrong: UserID=%d Store=%v", rec.UserID, rec.Store)
	}
	if rec.Status != StatusCreating {
		t.Errorf("status = %q, want CREATING", rec.Status)
	}
	// 中台返回值优先。
	if rec.CpAppID != 42 || rec.AppName != "微信" || rec.PackageName != "com.tencent.mm" ||
		rec.Version != "8.0" || rec.AppMD5 != "abc123" || rec.FileSize != "256MB" || rec.IconPath != "/icon.png" {
		t.Errorf("midplat fields not preferred: %+v", rec)
	}
	// 落库后能按属主查回。
	got, err := svc.repo.getByIDs(7, []int{int(rec.ID)})
	if err != nil || len(got) != 1 {
		t.Fatalf("record not found by owner: err=%v n=%d", err, len(got))
	}
}

func TestCreateFromUpload_FallsBackToRequestFields(t *testing.T) {
	// 中台只回主键（秒传 / 精简响应），其余字段空 → 回退用前端确认面板传来的元信息。
	port := &fakeUploadPort{created: &midplat.CreatedApp{ID: 9}}
	svc := newService(newRepository(framework.DB), port)

	rec, err := svc.CreateFromUpload(3, midplat.CreateFromUploadedFileRequest{
		UploadID: 0, // 秒传命中：uploadId 为 0
		AppName:  "抖音", PackageName: "com.ss.android", Version: "1.2.3", MD5: "md5xyz", FileSize: "19.89 MB", IconPath: "/dy.png",
	})
	if err != nil {
		t.Fatalf("CreateFromUpload error: %v", err)
	}
	if rec.CpAppID != 9 {
		t.Errorf("CpAppID = %d, want 9", rec.CpAppID)
	}
	if rec.AppName != "抖音" || rec.PackageName != "com.ss.android" || rec.Version != "1.2.3" ||
		rec.AppMD5 != "md5xyz" || rec.FileSize != "19.89 MB" || rec.IconPath != "/dy.png" {
		t.Errorf("request fallback fields wrong: %+v", rec)
	}
}
