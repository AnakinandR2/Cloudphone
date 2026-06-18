package openapi

import (
	"os"
	"strings"
	"testing"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	tdb, _ := framework.SetupTestDB(m)
	// 建所有已注册模块的表（openapi 的 /apps、/scripts 委托 app/automation，需其表与服务装配）。
	if err := framework.RunSetup(framework.DB); err != nil {
		panic(err)
	}
	for _, mod := range framework.GlobalModule.GetAll() {
		if err := mod.Init(framework.DB); err != nil {
			panic(err)
		}
	}
	code := m.Run()
	tdb.Teardown()
	os.Exit(code)
}

const userA = 8001

// 创建返回完整明文，库内只存哈希；鉴权能解析属主。
func TestCreateAndAuthenticate(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM api_keys") })

	res, err := Service.Create(userA, "默认密钥")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(res.FullKey, "gp_live_"))
	assert.Contains(t, res.Masked, "••••")
	assert.Equal(t, "active", res.Status)

	// 库内只存 sha256（不含明文）。
	k, err := Service.repo.findByHash(hashKey(res.FullKey))
	require.NoError(t, err)
	assert.Len(t, k.KeyHash, 64)
	assert.NotContains(t, k.KeyHash, res.FullKey)
	assert.Len(t, k.KeyLast4, 4)
	assert.True(t, strings.HasSuffix(res.FullKey, k.KeyLast4))

	// 鉴权：正确明文 → 属主；伪造/错前缀 → 失败。
	uid, ok := Service.Authenticate(res.FullKey)
	assert.True(t, ok)
	assert.Equal(t, userA, uid)
	_, ok = Service.Authenticate("gp_live_deadbeef")
	assert.False(t, ok)
	_, ok = Service.Authenticate("not-a-key")
	assert.False(t, ok)

	// 名称必填。
	_, err = Service.Create(userA, "  ")
	require.Error(t, err)
}

// 可重复查看：reveal 解密返回与创建一致的明文；旧密钥/越权报错。
func TestReveal(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM api_keys") })

	res, err := Service.Create(userA, "可查看")
	require.NoError(t, err)

	full, err := Service.Reveal(userA, res.ID)
	require.NoError(t, err)
	assert.Equal(t, res.FullKey, full, "reveal 应返回与创建一致的完整明文")

	// 越权 reveal → NotFound。
	_, err = Service.Reveal(userA+1, res.ID)
	require.Error(t, err)

	// 旧密钥（无 cipher）→ 明确报错。
	legacy := &APIKey{UserID: userA, Name: "旧", KeyPrefix: "gp_live_dead", KeyLast4: "beef", KeyHash: "legacyhash"}
	require.NoError(t, framework.DB.Create(legacy).Error)
	_, err = Service.Reveal(userA, legacy.ID)
	require.Error(t, err)
}

// 撤销后立即失效，列表状态为 revoked。
func TestRevoke(t *testing.T) {
	t.Cleanup(func() { framework.DB.Exec("DELETE FROM api_keys") })

	res, err := Service.Create(userA, "待撤销")
	require.NoError(t, err)
	require.NoError(t, Service.Revoke(userA, res.ID))

	_, ok := Service.Authenticate(res.FullKey)
	assert.False(t, ok, "撤销后鉴权应失败")

	list, err := Service.List(userA)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "revoked", list[0].Status)

	// 重复撤销幂等；撤销别人的 → NotFound。
	require.NoError(t, Service.Revoke(userA, res.ID))
	require.Error(t, Service.Revoke(userA+1, res.ID))
}
