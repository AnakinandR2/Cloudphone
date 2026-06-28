package library

import (
	"encoding/json"
	"net/http"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// currentUserID 取当前登录用户 ID（user 鉴权中间件写入）。
func currentUserID(c *gin.Context) (int, bool) {
	v, ok := c.Get("userID")
	if !ok {
		return 0, false
	}
	id, ok := v.(int)
	return id, ok
}

// GetOverview 概览：用量 + 订阅 + 档位目录 + 免费额度 + 时长选项 + notice。
func GetOverview(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	ov, err := Service.Overview(uid)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, ov)
}

// QuotePackage 套餐报价（rich 预览，转调 QuotePackage）。
func QuotePackage(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}
	var body json.RawMessage
	if err := c.ShouldBindJSON(&body); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	res, err := Service.QuotePackage(uid, body)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, res)
}
