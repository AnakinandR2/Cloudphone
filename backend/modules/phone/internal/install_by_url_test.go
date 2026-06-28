package phone

import (
	"errors"
	"testing"

	"manager-backend/framework"
	"manager-backend/framework/midplat"
	"manager-backend/modules/app"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withStubInstallSpecs 临时把 app 解析接缝换成桩，测试结束自动还原（绕过 app 内部 DB/S3）。
func withStubInstallSpecs(t *testing.T, fn func(int, []app.AppRef) ([]app.InstallSpec, error)) {
	t.Helper()
	prev := resolveInstallSpecs
	resolveInstallSpecs = fn
	t.Cleanup(func() { resolveInstallSpecs = prev })
}

// TestInstallByURLMapsRequestAndReturnsTasks 校验：逐台属主校验收集 cpId、app 解析载荷映射到
// 中台 InstallByURLRequest、taskInfoList 透出为 task_info_list。
func TestInstallByURLMapsRequestAndReturnsTasks(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	f := &fakePort{installByURLResp: &midplat.InstallAppResponse{
		TaskInfoList: []midplat.AppInstallTaskInfo{{TaskID: "tk-1", InstanceID: "cp-x"}},
	}}
	withFakeOps(t, f)

	id1 := provisionedPhone(t, userA, "cp-x")
	id2 := provisionedPhone(t, userA, "cp-y")

	// 桩：断言收到的 userID/refs，返回两个安装载荷。
	var gotUID int
	var gotRefs []app.AppRef
	withStubInstallSpecs(t, func(uid int, refs []app.AppRef) ([]app.InstallSpec, error) {
		gotUID, gotRefs = uid, refs
		return []app.InstallSpec{
			{AppName: "User App", DownloadURL: "https://s3/private/a.apk", MD5: "m1", PackageName: "com.a", Version: "1", FileSize: "10MB"},
			{AppName: "Market App", DownloadURL: "https://pub/b.apk", MD5: "m2", PackageName: "com.b", Version: "2"},
		}, nil
	})

	refs := []AppRefInput{{Source: "user", ID: 7}, {Source: "market", ID: 9}}
	tasks, err := PhoneService.InstallByURL(userA, []int{id1, id2}, refs)
	require.NoError(t, err)

	// 解析接缝收到正确 uid + refs（source/id 映射）。
	assert.Equal(t, userA, gotUID)
	require.Len(t, gotRefs, 2)
	assert.Equal(t, app.AppRef{Source: "user", ID: 7}, gotRefs[0])
	assert.Equal(t, app.AppRef{Source: "market", ID: 9}, gotRefs[1])

	// 下发到中台的请求：cpIds 来自属主校验收集，apps 来自解析载荷映射。
	assert.Equal(t, []string{"cp-x", "cp-y"}, f.lastInstallByURL.CpIDs)
	require.Len(t, f.lastInstallByURL.Apps, 2)
	assert.Equal(t, "https://s3/private/a.apk", f.lastInstallByURL.Apps[0].DownloadURL)
	assert.Equal(t, "com.a", f.lastInstallByURL.Apps[0].PackageName)
	assert.Equal(t, "10MB", f.lastInstallByURL.Apps[0].FileSize)
	assert.Equal(t, "com.b", f.lastInstallByURL.Apps[1].PackageName)

	// 中台 taskInfoList 透出为 task_info_list。
	require.Len(t, tasks, 1)
	assert.Equal(t, "tk-1", tasks[0].TaskID)
	assert.Equal(t, "cp-x", tasks[0].InstanceID)
}

// TestInstallByURLOwnershipFailsBeforeDispatch 非本人/未开通的手机 → 整请求前置失败，不触达中台、不解析。
func TestInstallByURLOwnershipFailsBeforeDispatch(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	f := &fakePort{}
	withFakeOps(t, f)
	id := provisionedPhone(t, userA, "cp-x")

	resolved := false
	withStubInstallSpecs(t, func(int, []app.AppRef) ([]app.InstallSpec, error) {
		resolved = true
		return []app.InstallSpec{{PackageName: "com.a"}}, nil
	})

	// userB 不拥有该手机 → NotFound，整请求失败。
	_, err := PhoneService.InstallByURL(userB, []int{id}, []AppRefInput{{Source: "user", ID: 1}})
	assert.Error(t, err)
	assert.False(t, resolved, "属主校验失败不应解析安装载荷")
	assert.Equal(t, 0, f.calls, "属主校验失败不应触达中台")
}

// TestInstallByURLResolveErrorPropagates 应用解析失败（未就绪/锁定）→ 整请求失败，不触达中台。
func TestInstallByURLResolveErrorPropagates(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	f := &fakePort{}
	withFakeOps(t, f)
	id := provisionedPhone(t, userA, "cp-x")

	withStubInstallSpecs(t, func(int, []app.AppRef) ([]app.InstallSpec, error) {
		return nil, errors.New("应用尚未就绪")
	})

	_, err := PhoneService.InstallByURL(userA, []int{id}, []AppRefInput{{Source: "user", ID: 1}})
	assert.Error(t, err)
	assert.Equal(t, 0, f.calls, "解析失败不应触达中台")
}

// TestInstallByURLValidatesInput 空 phone_ids / 空 apps → 400，不解析、不下发。
func TestInstallByURLValidatesInput(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("cloud_phones") })
	f := &fakePort{}
	withFakeOps(t, f)
	id := provisionedPhone(t, userA, "cp-x")

	_, err := PhoneService.InstallByURL(userA, nil, []AppRefInput{{Source: "user", ID: 1}})
	assert.Error(t, err)
	_, err = PhoneService.InstallByURL(userA, []int{id}, nil)
	assert.Error(t, err)
	assert.Equal(t, 0, f.calls)
}
