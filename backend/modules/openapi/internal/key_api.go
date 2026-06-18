package openapi

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// currentUserID 取上下文里的属主用户 ID（由 user 中间件或 key 中间件写入）。
func currentUserID(c *gin.Context) (int, bool) {
	v, ok := c.Get("userID")
	if !ok {
		return 0, false
	}
	id, ok := v.(int)
	return id, ok
}

func paramUint(c *gin.Context, key string) (uint, bool) {
	n, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil || n == 0 {
		return 0, false
	}
	return uint(n), true
}

// ===== 密钥管理（JWT 保护，供 my UI）=====

// ListKeys 列出当前用户的 API 密钥（掩码）。
func ListKeys(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	list, err := Service.List(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

// CreateKey 新建一把密钥，返回完整明文（仅此一次）。
func CreateKey(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	res, err := Service.Create(uid, body.Name)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, res)
}

// RevealKey 解密返回完整密钥（重复查看）。
func RevealKey(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := paramUint(c, "id")
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "ID 非法")
		return
	}
	full, err := Service.Reveal(uid, id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, gin.H{"fullKey": full})
}

// RevokeKey 撤销一把密钥（立即失效）。
func RevokeKey(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	id, ok := paramUint(c, "id")
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "ID 非法")
		return
	}
	if err := Service.Revoke(uid, id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}
