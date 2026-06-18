package automation

import (
	"net/http"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// ---- 运营：商店脚本管理 ----

func AdminStoreList(c *gin.Context) {
	list, err := Service.ListStoreScripts()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

func AdminStoreCreate(c *gin.Context) {
	var b scriptBody
	if err := c.ShouldBindJSON(&b); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	rec, err := Service.AdminCreateStoreScript(b.toInput())
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, rec)
}

func AdminStoreUpdate(c *gin.Context) {
	id, ok := paramUint(c, "id")
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "ID 非法")
		return
	}
	var b scriptBody
	if err := c.ShouldBindJSON(&b); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}
	rec, err := Service.AdminUpdateStoreScript(id, b.toInput())
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, rec)
}

func AdminStoreToggle(c *gin.Context) {
	id, ok := paramUint(c, "id")
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "ID 非法")
		return
	}
	var b struct {
		Enabled bool `json:"enabled"`
	}
	_ = c.ShouldBindJSON(&b)
	if err := Service.AdminToggleStoreScript(id, b.Enabled); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

func AdminStoreDelete(c *gin.Context) {
	id, ok := paramUint(c, "id")
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "ID 非法")
		return
	}
	if err := Service.AdminDeleteStoreScript(id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// ---- 运营：用户脚本治理 ----

func AdminUserScriptList(c *gin.Context) {
	list, err := Service.AdminListUserScripts()
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, list)
}

func AdminUserScriptToggle(c *gin.Context) {
	id, ok := paramUint(c, "id")
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "ID 非法")
		return
	}
	var b struct {
		Enabled bool `json:"enabled"`
	}
	_ = c.ShouldBindJSON(&b)
	if err := Service.AdminToggleUserScript(id, b.Enabled); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

func AdminUserScriptDelete(c *gin.Context) {
	id, ok := paramUint(c, "id")
	if !ok {
		framework.Fail(c, http.StatusBadRequest, "ID 非法")
		return
	}
	if err := Service.AdminDeleteUserScript(id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}
