package app

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helloAPK 读取 apkparse 包内的真实最小 APK 夹具字节。
func helloAPK(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("apkparse", "testdata", "helloworld.apk"))
	require.NoError(t, err)
	return b
}

// ── finalize ──────────────────────────────────────────────────────────────

// finalize 成功：回读 APK → 解析 → 落 ready 元数据 + 图标 URL（公有桶）。
func TestFinalizeUserApp_Ready(t *testing.T) {
	if framework.S3 == nil && framework.S3Library == nil {
		// 由测试自带的 fakeS3 提供，二者都装上，故不会 skip；保留守卫以示意图。
	}
	pub, _ := fakeS3(t, "public", "https://cdn.example.com")
	lib, store := fakeS3(t, "lib", "")
	withS3(t, pub, lib)

	const uid = 8400001
	fid := addLibFile(t, uid, "hello.apk", "apps/hello.apk", helloAPK(t), store)

	dto, err := AppService.FinalizeUserApp(uid, fid)
	require.NoError(t, err)
	require.NotNil(t, dto)
	assert.Equal(t, ParseStatusReady, dto.ParseStatus)
	assert.NotEmpty(t, dto.PackageName)
	assert.Empty(t, dto.ParseError)

	m, err := AppService.repo.getUserMeta(fid)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, ParseStatusReady, m.ParseStatus)
	assert.Len(t, m.MD5, 32)
	if m.IconURL != "" {
		assert.Contains(t, m.IconURL, "https://cdn.example.com/app-icons/")
	}
}

// finalize 失败：内容不是合法 APK → 落 failed + parse_error，仍返回 DTO（不报错）。
func TestFinalizeUserApp_Failed(t *testing.T) {
	pub, _ := fakeS3(t, "public", "https://cdn.example.com")
	lib, store := fakeS3(t, "lib", "")
	withS3(t, pub, lib)

	const uid = 8400002
	fid := addLibFile(t, uid, "broken.apk", "apps/broken.apk", []byte("not a zip"), store)

	dto, err := AppService.FinalizeUserApp(uid, fid)
	require.NoError(t, err)
	require.NotNil(t, dto)
	assert.Equal(t, ParseStatusFailed, dto.ParseStatus)
	assert.NotEmpty(t, dto.ParseError)
}

// finalize 非属主：library 门面拦截（文件不属于该用户）。
func TestFinalizeUserApp_NotOwner(t *testing.T) {
	pub, _ := fakeS3(t, "public", "https://cdn.example.com")
	lib, store := fakeS3(t, "lib", "")
	withS3(t, pub, lib)

	const owner = 8400003
	fid := addLibFile(t, owner, "hello.apk", "apps/owner.apk", helloAPK(t), store)

	_, err := AppService.FinalizeUserApp(owner+1, fid)
	require.Error(t, err)
}

// ── ResolveInstallSpecs ─────────────────────────────────────────────────────

// market ready：DownloadURL = PublicBaseURL + "/" + S3Key（无需 live S3，仅用 PublicBaseURL 拼接）。
func TestResolveInstallSpecs_MarketReady(t *testing.T) {
	pub, _ := fakeS3(t, "public", "https://cdn.example.com")
	withS3(t, pub, framework.S3Library)

	rec := &AppMarket{
		S3Key:       "app-market/abc.apk",
		AppName:     "Demo",
		PackageName: "com.demo",
		Version:     "1.2.3",
		MD5:         "0123456789abcdef0123456789abcdef",
		FileSize:    4096,
		ParseStatus: ParseStatusReady,
	}
	require.NoError(t, AppService.repo.createMarket(rec))

	specs, err := AppService.ResolveInstallSpecs(0, []AppRef{{Source: "market", ID: rec.ID}})
	require.NoError(t, err)
	require.Len(t, specs, 1)
	assert.Equal(t, "https://cdn.example.com/app-market/abc.apk", specs[0].DownloadURL)
	assert.Equal(t, "com.demo", specs[0].PackageName)
	assert.Equal(t, "1.2.3", specs[0].Version)
	assert.Equal(t, "0123456789abcdef0123456789abcdef", specs[0].MD5)
	assert.Equal(t, "4096", specs[0].FileSize)
}

// market 非就绪：整请求失败。
func TestResolveInstallSpecs_MarketNotReady(t *testing.T) {
	rec := &AppMarket{S3Key: "app-market/parsing.apk", ParseStatus: ParseStatusParsing}
	require.NoError(t, AppService.repo.createMarket(rec))
	_, err := AppService.ResolveInstallSpecs(0, []AppRef{{Source: "market", ID: rec.ID}})
	require.Error(t, err)
}

// 不存在：整请求失败。
func TestResolveInstallSpecs_Missing(t *testing.T) {
	_, err := AppService.ResolveInstallSpecs(0, []AppRef{{Source: "market", ID: 9999999}})
	require.Error(t, err)
	_, err = AppService.ResolveInstallSpecs(7, []AppRef{{Source: "user", ID: 9999999}})
	require.Error(t, err)
}

// user ready：校验 meta.ready → OpenFileWithTTL 取 presigned GET → 组 spec（md5/pkg/version 来自 meta）。
func TestResolveInstallSpecs_UserReady(t *testing.T) {
	if framework.S3Library == nil {
		// 用例自带 fakeS3 提供 S3Library；保留守卫示意。
	}
	lib, store := fakeS3(t, "lib", "")
	withS3(t, framework.S3, lib)

	const uid = 8400010
	fid := addLibFile(t, uid, "u.apk", "apps/u.apk", []byte("apkbytes"), store)
	require.NoError(t, AppService.repo.upsertUserMeta(&AppUserMeta{
		LibraryFileID: fid,
		UserID:        uid,
		AppName:       "MyApp",
		PackageName:   "com.my.app",
		Version:       "9.9",
		MD5:           "ffffffffffffffffffffffffffffffff",
		ParseStatus:   ParseStatusReady,
	}))

	specs, err := AppService.ResolveInstallSpecs(uid, []AppRef{{Source: "user", ID: fid}})
	require.NoError(t, err)
	require.Len(t, specs, 1)
	assert.Equal(t, "com.my.app", specs[0].PackageName)
	assert.Equal(t, "9.9", specs[0].Version)
	assert.Equal(t, "ffffffffffffffffffffffffffffffff", specs[0].MD5)
	assert.NotEmpty(t, specs[0].DownloadURL) // presigned GET URL
	assert.Equal(t, strconv.Itoa(len("apkbytes")), specs[0].FileSize)
}

// user 非就绪：parse_status != ready → 拒绝。
func TestResolveInstallSpecs_UserNotReady(t *testing.T) {
	lib, store := fakeS3(t, "lib", "")
	withS3(t, framework.S3, lib)

	const uid = 8400011
	fid := addLibFile(t, uid, "p.apk", "apps/p.apk", []byte("x"), store)
	require.NoError(t, AppService.repo.upsertUserMeta(&AppUserMeta{
		LibraryFileID: fid, UserID: uid, ParseStatus: ParseStatusParsing,
	}))
	_, err := AppService.ResolveInstallSpecs(uid, []AppRef{{Source: "user", ID: fid}})
	require.Error(t, err)
}

// user 非属主：meta ready 但 library OpenFileWithTTL 属主校验拒绝。
func TestResolveInstallSpecs_UserNotOwner(t *testing.T) {
	lib, store := fakeS3(t, "lib", "")
	withS3(t, framework.S3, lib)

	const owner = 8400012
	fid := addLibFile(t, owner, "o.apk", "apps/o.apk", []byte("x"), store)
	require.NoError(t, AppService.repo.upsertUserMeta(&AppUserMeta{
		LibraryFileID: fid, UserID: owner, ParseStatus: ParseStatusReady,
		MD5: "ffffffffffffffffffffffffffffffff", PackageName: "p", Version: "1",
	}))
	_, err := AppService.ResolveInstallSpecs(owner+1, []AppRef{{Source: "user", ID: fid}})
	require.Error(t, err)
}
