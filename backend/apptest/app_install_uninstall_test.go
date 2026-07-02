package apptest

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"manager-backend/framework"
	"manager-backend/modules/app"
	"manager-backend/modules/billing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// app_install_uninstall_test.go 覆盖按 URL 安装矩阵 + 卸载（对应计划文档
// 《2026-07-02-验收测试-planB-接口层全覆盖.md》任务 15）：
// TC-12-060（单台安装，user 来源）、TC-12-061（单台安装，market 来源）、
// TC-12-066（未知来源拒绝）、TC-12-081（卸载 appIds/packageNames）、TC-12-082（卸载越权）。
//
// 承接 Plan A 已覆盖的 modules/phone/internal/install_by_url_test.go（多台映射/属主前置/
// 未就绪透传/空入参）与 modules/phone/internal/ops_test.go 的 TestOpInvalidArgs（appIds/
// packageNames 都为空 → 400 的内部单元覆盖，用注入 fakePort 绕过中台）。
//
// 环境约束（务必先读，决定了本文件的断言边界）：apptest 的 phone 模块在 TestMain 装配时
// newMidplatPort() 读取的是 apptest 进程 cwd（backend/apptest/）下的 .env——该目录不存在
// .env 文件，所以 s.ops 在整个 apptest 测试运行期间恒为 nil，且 PhoneService 是进程级单例，
// 无法在单个测试内重新装配。resolveCp()（modules/phone/internal/service.go）的校验顺序是
// 「属主 → CpID 非空 → ops 非 nil」，因此：
//   - 任何经 HTTP 建的手机 CpID 恒为空串（未配置中台走本地降级分支），装载/卸载会先在
//     「云手机尚未开通」(422) 处短路，永远到不了 ops==nil 的「中台未配置」(500) 分支，
//     更到不了 app.ResolveInstallSpecs 的来源校验分支。
//   - 因此 TC-12-060/061/066 涉及 ResolveInstallSpecs 具体载荷构造（presigned URL / 永久
//     公有 URL / 未知来源报错）的断言改为直接调用 app 模块的公开门面 app.ResolveInstallSpecs
//     （非 internal，是模块对外的合法接口，phone 模块自身也是通过它拿安装载荷）。这与
//     TC 文档「ResolveInstallSpecs xxx 分支」的表述完全对应，且不依赖不可控的真实中台网络。
//   - TC-12-081/082 中「属主先于其他校验」的部分可经真实 HTTP + 真实 JWT 鉴权验证（这是
//     Plan A 的 internal 单测覆盖不到的：internal 用 ctxFor 直接注入 userID，绕过了真实
//     AuthMiddleware 和真实跨用户边界）；而「appIds 与 packageNames 至少传一个」的 400 在
//     apptest 环境下同样会被同一 422 短路，已由 Plan A 的 TestOpInvalidArgs 覆盖，此处只做
//     「同一位置短路」的一致性核对，不重复造断言。

// appIUReadyMarketApp 用 admin 权限经市场上传接口造一个 ready 的市场应用，返回其 market id
// 与 S3Key（供断言 DownloadURL = PublicBaseURL + "/" + S3Key）。调用前需已 withFakeS3(公有桶,...)。
func appIUReadyMarketApp(t *testing.T, r *gin.Engine, adminTok, username, pkg, version, name string) (id uint, s3Key string) {
	t.Helper()
	tok, cleanup := marketStaffWithPerms(t, r, adminTok, username, []string{"app:manage"})
	t.Cleanup(cleanup)

	pkgBytes := minimalXAPK(t, pkg, version, name)
	w := marketMultipartUpload(t, r, tok, name+".xapk", pkgBytes)
	require.Equal(t, http.StatusOK, w.Code, "市场上传失败: %s", w.Body.String())
	dto := decode(t, w).Data.(map[string]interface{})
	require.Equal(t, "ready", dto["parse_status"], "最小可解析 xapk 应落 ready: %v", dto)
	id = uint(dto["id"].(float64))
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM app_market WHERE id = ?", id) })

	require.NoError(t, framework.DB.Table("app_market").
		Where("id = ?", id).Select("s3_key").Scan(&s3Key).Error)
	require.NotEmpty(t, s3Key)
	return id, s3Key
}

// TestResolveInstallSpecs_UserSourcePresignedURL 覆盖 TC-12-060：user 来源（素材库私有桶
// 应用）解析安装载荷，DownloadURL 必须是带时效的 presigned GET（AWS SigV4 查询串），
// 而非公有桶永久 URL；PackageName/Version/AppName/FileSize 与 finalize 结果一致。
func TestResolveInstallSpecs_UserSourcePresignedURL(t *testing.T) {
	const phone = "13915000001"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	pubCli, _ := newFakeS3(t, "gp-pub-iu-060", "https://cdn.iu060.test/")
	libCli, _ := newFakeS3(t, "gp-lib-iu-060", "")
	withFakeS3(t, pubCli, libCli)

	const wantPkg, wantVer, wantName = "com.gp.iu060", "6.0.0", "IU060App"
	content := minimalXAPK(t, wantPkg, wantVer, wantName)
	fileID := appUploadReadyApp(t, r, token, wantPkg, wantVer, wantName)
	require.Greater(t, fileID, uint(0))

	specs, err := app.ResolveInstallSpecs(int(uid), []app.AppRef{{Source: "user", ID: fileID}})
	require.NoError(t, err, "己方 ready 用户应用解析安装载荷不应报错")
	require.Len(t, specs, 1)
	spec := specs[0]

	assert.Equal(t, wantPkg, spec.PackageName)
	assert.Equal(t, wantVer, spec.Version)
	assert.Equal(t, wantName, spec.AppName)
	assert.Equal(t, fmt.Sprintf("%d", len(content)), spec.FileSize, "FileSize 应为真实字节数的十进制字符串")
	assert.NotEmpty(t, spec.MD5)

	// presigned GET：必须带 AWS SigV4 签名查询参数（时效性 URL），且不能是市场分支的
	// PublicBaseURL 永久拼接形态（否则就退化成了公有桶永久链接，绕过了私有桶的属主/时效控制）。
	assert.Contains(t, spec.DownloadURL, "X-Amz-Signature=", "user 来源应是带签名时效的 presigned GET")
	assert.False(t, strings.HasPrefix(spec.DownloadURL, "https://cdn.iu060.test/"),
		"user 来源不应退化为公有桶永久 URL: %s", spec.DownloadURL)
}

// TestResolveInstallSpecs_MarketSourcePublicPermanentURL 覆盖 TC-12-061：market 来源（应用
// 市场公有桶应用）解析安装载荷，DownloadURL 必须精确等于 framework.S3.PublicURL(S3Key)
// （= PublicBaseURL + "/" + S3Key，永久可达，非 presigned）。
func TestResolveInstallSpecs_MarketSourcePublicPermanentURL(t *testing.T) {
	const phone = "13915000002"
	r := setupRouter()
	registerUser(t, r, phone) // 仅需已注册用户产生 uid，走公开门面本身不需要 token
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })
	adminTok := adminToken(t, r)

	const publicBase = "https://cdn.iu061.test"
	pubCli, _ := newFakeS3(t, "gp-pub-iu-061", publicBase+"/")
	withFakeS3(t, pubCli, nil)

	marketID, s3Key := appIUReadyMarketApp(t, r, adminTok, "iu061_market_admin",
		"com.gp.iu061", "6.1.0", "IU061Market")

	specs, err := app.ResolveInstallSpecs(int(uid), []app.AppRef{{Source: "market", ID: marketID}})
	require.NoError(t, err, "己方访问 ready 市场应用解析安装载荷不应报错")
	require.Len(t, specs, 1)
	spec := specs[0]

	wantURL := pubCli.PublicURL(s3Key)
	assert.Equal(t, publicBase+"/"+s3Key, wantURL, "自检：fakeS3 PublicURL 契约应为 PublicBaseURL+\"/\"+S3Key")
	assert.Equal(t, wantURL, spec.DownloadURL, "market 来源 DownloadURL 应精确等于 PublicURL(S3Key)（永久链接）")
	assert.NotContains(t, spec.DownloadURL, "X-Amz-Signature=", "market 来源不应是 presigned（应为永久公有 URL）")
	assert.Equal(t, "com.gp.iu061", spec.PackageName)
	assert.Equal(t, "6.1.0", spec.Version)
	assert.Equal(t, "IU061Market", spec.AppName)
}

// TestResolveInstallSpecs_UnknownSourceRejected 覆盖 TC-12-066：source 既非 user 也非
// market（如 'other'）→ ResolveInstallSpecs default 分支 400「未知应用来源」，且不产出
// 任何安装载荷（整批失败，不会把其他合法条目部分放行）。
func TestResolveInstallSpecs_UnknownSourceRejected(t *testing.T) {
	const phone = "13915000003"
	r := setupRouter()
	registerUser(t, r, phone) // 仅需已注册用户产生 uid，走公开门面本身不需要 token
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	specs, err := app.ResolveInstallSpecs(int(uid), []app.AppRef{{Source: "other", ID: 1}})
	require.Error(t, err, "未知来源应报错")
	assert.Nil(t, specs)
	assert.Contains(t, err.Error(), "未知应用来源")

	// 混合请求：一个合法占位 + 一个未知来源，整批仍应失败（不产出部分结果）。
	specs2, err2 := app.ResolveInstallSpecs(int(uid), []app.AppRef{
		{Source: "market", ID: 999999999},
		{Source: "other", ID: 1},
	})
	require.Error(t, err2)
	assert.Nil(t, specs2)
}

// appIUProvisionSeat 给某用户授予座位额度，供该用户经 HTTP 建机（checkSeatAvailable 前置校验）。
func appIUProvisionSeat(t *testing.T, uid uint) {
	t.Helper()
	require.NoError(t, billing.GrantSeatLicensesForTest(int(uid), 3))
}

// TestPhoneUninstallApp_OwnershipDeniedBeforeAnyOtherCheck 覆盖 TC-12-082：卸载越权——B 对
// A 拥有的云手机发起卸载，必须在 resolveCp 的属主校验处即被拒绝（404「云手机不存在」），
// 且这一属主校验先于「CpID 是否开通」「appIds/packageNames 是否为空」等后续校验生效——
// 用「携带看起来合法的 appIds 请求体」和「完全不带任何字段的空请求体」两种输入都应得到
// 同样的 404，证明拒绝原因是属主而非参数校验。
//
// 本测试用真实 HTTP + 真实 JWT 鉴权（A/B 两个独立注册用户），是 Plan A 的
// modules/phone/internal 单元测试覆盖不到的边界：internal 测试用 ctxFor 直接把 userID
// 注入 gin.Context，绕开了真实 AuthMiddleware 与真实跨用户越权路径。
func TestPhoneUninstallApp_OwnershipDeniedBeforeAnyOtherCheck(t *testing.T) {
	const phoneA = "13915000004"
	const phoneB = "13915000005"
	r := setupRouter()
	tokenA := registerUser(t, r, phoneA)
	tokenB := registerUser(t, r, phoneB)
	uidA := userIDByPhone(t, phoneA)
	uidB := userIDByPhone(t, phoneB)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidA) })
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uidB) })

	appIUProvisionSeat(t, uidA)

	// A 建机（中台未配置 → 本地降级分支，直接 CREATED，CpID 为空）。
	cw := doJSON(r, "POST", "/api/v1/phone/create", tokenA, map[string]interface{}{
		"name": "iu082-A机",
	})
	require.Equal(t, http.StatusOK, cw.Code, "A 建机失败: %s", cw.Body.String())
	phoneID := int(decode(t, cw).Data.(map[string]interface{})["id"].(float64))
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM cloud_phones WHERE id = ?", phoneID) })

	// B 对 A 的手机发起卸载（带看似合法的 appIds）→ 404，且文案是属主校验的「云手机不存在」，
	// 不是「appIds 与 packageNames 至少传一个」或「云手机尚未开通」这类其他分支的文案。
	w1 := doJSON(r, "POST", fmt.Sprintf("/api/v1/phone/%d/apps/uninstall", phoneID), tokenB, map[string]interface{}{
		"appIds": []int64{7},
	})
	assert.Equal(t, http.StatusNotFound, w1.Code, "B 卸载 A 的机应 404: %s", w1.Body.String())
	assert.Contains(t, decode(t, w1).Message, "云手机不存在", "越权卸载应透传属主校验文案，而非参数/开通校验文案")

	// B 对 A 的手机发起卸载（完全空请求体）→ 同样 404 + 同一文案，证明属主校验先于参数校验短路。
	w2 := doJSON(r, "POST", fmt.Sprintf("/api/v1/phone/%d/apps/uninstall", phoneID), tokenB, map[string]interface{}{})
	assert.Equal(t, http.StatusNotFound, w2.Code, "B 空体卸载 A 的机应仍 404: %s", w2.Body.String())
	assert.Contains(t, decode(t, w2).Message, "云手机不存在")

	// 对照：A 自己对同一台机卸载（appIds 非空）不应被属主校验拒绝——会在 apptest 环境下于
	// 「云手机尚未开通」(422) 处短路（本环境 CpID 恒为空，见文件头注释），但绝不能是 404，
	// 用状态码差异证明上面两次 404 确实来自属主校验而非其他共因。
	w3 := doJSON(r, "POST", fmt.Sprintf("/api/v1/phone/%d/apps/uninstall", phoneID), tokenA, map[string]interface{}{
		"appIds": []int64{7},
	})
	assert.NotEqual(t, http.StatusNotFound, w3.Code, "属主自己操作不应被当成 404 拒绝: %s", w3.Body.String())
	assert.Equal(t, http.StatusUnprocessableEntity, w3.Code, "本环境未开通(CpID空)应 422: %s", w3.Body.String())
	assert.Contains(t, decode(t, w3).Message, "尚未开通")
}

// TestPhoneUninstallApp_ArgsGateOrderConsistentWithPlanA 覆盖 TC-12-081 的接口层核对：
// 卸载请求体 appIds/packageNames 的「至少传一个」校验，在 resolveCp 之后才执行（见
// modules/phone/internal/service.go UninstallApp）。在 apptest 环境下（CpID 恒为空）
// 这条校验会被同一个「云手机尚未开通」422 短路，永远走不到「appIds 与 packageNames
// 至少传一个」的 400 分支——该 400 分支已由 Plan A 的
// modules/phone/internal/ops_test.go TestOpInvalidArgs（PhoneService.UninstallApp(...,nil,nil)）
// 用注入 fakePort 真正验证过，本测试不重复造该断言，只核对「同一手机的 appIds 非空/
// packageNames 非空/两者皆空」三种请求体在本环境下确实落到同一 422 位置（回归防线：
// 若谁改动了 resolveCp 与参数校验的先后顺序，本测试能感知到状态码变化）。
func TestPhoneUninstallApp_ArgsGateOrderConsistentWithPlanA(t *testing.T) {
	const phone = "13915000006"
	r := setupRouter()
	token := registerUser(t, r, phone)
	uid := userIDByPhone(t, phone)
	t.Cleanup(func() { cleanupUserAndBillingByID(t, uid) })

	appIUProvisionSeat(t, uid)

	cw := doJSON(r, "POST", "/api/v1/phone/create", token, map[string]interface{}{
		"name": "iu081-机",
	})
	require.Equal(t, http.StatusOK, cw.Code, "建机失败: %s", cw.Body.String())
	phoneID := int(decode(t, cw).Data.(map[string]interface{})["id"].(float64))
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM cloud_phones WHERE id = ?", phoneID) })

	cases := []map[string]interface{}{
		{"appIds": []int64{1, 2}},
		{"packageNames": []string{"com.a"}},
		{}, // 两者皆空
	}
	for i, body := range cases {
		w := doJSON(r, "POST", fmt.Sprintf("/api/v1/phone/%d/apps/uninstall", phoneID), token, body)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code,
			"case[%d] body=%v 在本环境应统一落到「未开通」422（CpID 恒空）: %s", i, body, w.Body.String())
		assert.Contains(t, decode(t, w).Message, "尚未开通",
			"case[%d] 未走到 appIds/packageNames 校验分支，说明属主+开通校验先行", i)
	}
}
