package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// SafeOrder 是防 ORDER BY 注入的白名单守卫：仅放行白名单字段，方向只认 "descending"。
func TestSafeOrder(t *testing.T) {
	allowed := map[string]bool{"created_at": true, "name": true}
	def := "id DESC"

	t.Run("空字段回退默认", func(t *testing.T) {
		assert.Equal(t, def, SafeOrder("", "ascending", allowed, def))
	})

	t.Run("非白名单字段回退默认（防注入）", func(t *testing.T) {
		assert.Equal(t, def, SafeOrder("password", "ascending", allowed, def))
		assert.Equal(t, def, SafeOrder("name; DROP TABLE users", "descending", allowed, def))
	})

	t.Run("合法字段默认升序", func(t *testing.T) {
		assert.Equal(t, "name ASC", SafeOrder("name", "ascending", allowed, def))
	})

	t.Run("descending 大小写不敏感", func(t *testing.T) {
		assert.Equal(t, "created_at DESC", SafeOrder("created_at", "descending", allowed, def))
		assert.Equal(t, "created_at DESC", SafeOrder("created_at", "DESCENDING", allowed, def))
	})

	t.Run("方向缺失或非全词 descending 视为升序", func(t *testing.T) {
		assert.Equal(t, "name ASC", SafeOrder("name", "", allowed, def))
		assert.Equal(t, "name ASC", SafeOrder("name", "desc", allowed, def))
	})
}
