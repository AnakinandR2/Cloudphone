package billing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// doPublicGet 构造一个仅挂公开路由（无任何鉴权中间件）的引擎并发起 GET，返回解包后的 envelope。
func doPublicGet(t *testing.T, path string) (int, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerBillingPublicRoutes(r)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return w.Code, body
}

func TestOpenGetPricing_NoAuth_ReturnsPricing(t *testing.T) {
	code, body := doPublicGet(t, "/api/open/v1/billing/pricing")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(0), body["code"])
	data := body["data"].(map[string]any)
	kinds := data["kinds"].(map[string]any)
	require.Contains(t, kinds, "seat")
	rt := data["runtime_pack"].(map[string]any)
	require.Contains(t, rt, "gift_minutes_per_seat_month")
}

func TestTrialMarketingFeatured_SingleSelect(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable(trialTables...) })
	repo := newTrialRepository(framework.DB)
	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "feat-a", Name: "A", Enabled: true, PerUserLimit: 1, Items: oneItem(KindSeat, 1, 7)}))
	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "feat-b", Name: "B", Enabled: true, PerUserLimit: 1, Items: oneItem(SubjectRuntimeMinute, 600, 0)}))
	a, err := repo.getPolicyByCode("feat-a")
	require.NoError(t, err)
	b, err := repo.getPolicyByCode("feat-b")
	require.NoError(t, err)

	// 标记 A → FeaturedPolicy 返回 A。
	require.NoError(t, TrialService.SetMarketingFeatured(int(a.ID), true))
	feat, err := TrialService.FeaturedPolicy()
	require.NoError(t, err)
	require.NotNil(t, feat)
	assert.Equal(t, "A", feat.Name)

	// 改标记 B → 单选：只剩 B。
	require.NoError(t, TrialService.SetMarketingFeatured(int(b.ID), true))
	feat, err = TrialService.FeaturedPolicy()
	require.NoError(t, err)
	require.NotNil(t, feat)
	assert.Equal(t, "B", feat.Name)
	var featuredCount int64
	framework.DB.Model(&TrialPolicy{}).Where("marketing_featured = ?", true).Count(&featuredCount)
	assert.Equal(t, int64(1), featuredCount)

	// 取消 B → 无展示。
	require.NoError(t, TrialService.SetMarketingFeatured(int(b.ID), false))
	feat, err = TrialService.FeaturedPolicy()
	require.NoError(t, err)
	assert.Nil(t, feat)
}

func TestTrialOverview_DisabledFeaturedHidden(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable(trialTables...) })
	repo := newTrialRepository(framework.DB)
	require.NoError(t, repo.createPolicy(&TrialPolicy{Code: "feat-c", Name: "C", Enabled: true, PerUserLimit: 1, Items: oneItem(KindSeat, 1, 7)}))
	c, err := repo.getPolicyByCode("feat-c")
	require.NoError(t, err)
	require.NoError(t, TrialService.SetMarketingFeatured(int(c.ID), true))

	// 已启用 + 标记：接口返回该策略。
	_, body := doPublicGet(t, "/api/open/v1/billing/trial-overview")
	data := body["data"].(map[string]any)
	require.NotNil(t, data["policy"])
	assert.Equal(t, "C", data["policy"].(map[string]any)["name"])

	// 禁用后：接口隐藏（policy=null）。
	disabled := false
	_, err = TrialService.UpdatePolicy(int(c.ID), &TrialPolicyUpdate{Enabled: &disabled})
	require.NoError(t, err)
	_, body = doPublicGet(t, "/api/open/v1/billing/trial-overview")
	data = body["data"].(map[string]any)
	assert.Nil(t, data["policy"])
}
