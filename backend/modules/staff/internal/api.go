package staff

import (
	"net/http"
	"strconv"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
)

// Login 用户登录
// @Summary 用户登录
// @Description 用户登录接口，返回 JWT Token
// @Tags 认证
// @Accept json
// @Produce json
// @Param body body LoginRequest true "登录信息"
// @Success 200 {object} framework.Response{data=LoginResponse}
// @Failure 400 {object} framework.Response
// @Failure 401 {object} framework.Response
// @Router /auth/login [post]
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	u, err := Service.GetStaffByUsername(req.Account)
	if err != nil {
		framework.Fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	if !u.IsActive {
		framework.Fail(c, http.StatusUnauthorized, "账号已被禁用")
		return
	}

	if !Service.VerifyPassword(u.HashedPassword, req.Password) {
		framework.Fail(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	token, err := GenerateTokenWithVersion(u.ID, u.Username, u.TokenVersion, framework.AppConfig.JWTSecret, framework.AppConfig.JWTExpireHours)
	if err != nil {
		framework.Fail(c, http.StatusInternalServerError, "生成Token失败")
		return
	}
	SetJWTCookie(c, token)

	framework.OKWithData(c, LoginResponse{
		Account:     u.Username,
		Token:       token,
		Avatar:      u.Avatar,
		IsSuperuser: u.IsSuperuser,
	})
}

// GetCurrentStaff 获取当前登录用户信息
// @Summary 获取当前用户信息
// @Description 根据 Token 获取当前登录用户的信息
// @Tags 认证
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} framework.Response{data=Staff}
// @Failure 401 {object} framework.Response
// @Router /auth/me [get]
func GetCurrentStaff(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}

	profile, err := Service.GetStaffProfile(userID.(int))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, profile)
}

// ChangePassword 修改密码
// @Summary 修改密码
// @Description 修改当前用户的密码
// @Tags 认证
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body ChangePasswordRequest true "密码信息"
// @Success 200 {object} framework.Response
// @Failure 400 {object} framework.Response
// @Failure 401 {object} framework.Response
// @Router /auth/change-password [post]
func ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}

	if err := Service.UpdatePassword(userID.(int), req.OldPassword, req.NewPassword); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}

// SSOLogin SSO登录
// @Summary SSO登录
// @Description 使用小西通行证SSO Token登录
// @Tags 认证
// @Accept json
// @Produce json
// @Param body body SSOLoginRequest true "SSO Token"
// @Success 200 {object} framework.Response{data=LoginResponse}
// @Failure 400 {object} framework.Response
// @Failure 401 {object} framework.Response
// @Router /auth/sso-login [post]
func SSOLogin(c *gin.Context) {
	var req SSOLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	u, err := SSOService.LoginWithSSO(req.Token)
	if err != nil {
		framework.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}

	token, err := GenerateTokenWithVersion(u.ID, u.Username, u.TokenVersion, framework.AppConfig.JWTSecret, framework.AppConfig.JWTExpireHours)
	if err != nil {
		framework.Fail(c, http.StatusInternalServerError, "生成Token失败")
		return
	}
	SetJWTCookie(c, token)

	framework.OKWithData(c, LoginResponse{
		Account:     u.Username,
		Token:       token,
		Avatar:      u.Avatar,
		IsSuperuser: u.IsSuperuser,
	})
}

// GetStaffList 获取用户列表
// @Summary 获取用户列表
// @Description 获取用户列表（分页）
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(10)
// @Param username query string false "用户名（模糊搜索）"
// @Param order query string false "排序字段"
// @Param sort query string false "排序方式（ascending/descending）"
// @Success 200 {object} framework.Response{data=framework.PageResponse}
// @Failure 500 {object} framework.Response
// @Router /user/list [get]
func GetStaffList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	username := c.Query("username")
	order := c.Query("order")
	sort := c.Query("sort")

	list, total, err := Service.GetStaffListWithRoles(page, size, username, order, sort)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithPage(c, list, total)
}

// GetStaff 获取用户详情
// @Summary 获取用户详情
// @Description 根据ID获取用户详情
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "用户ID"
// @Success 200 {object} framework.Response{data=Staff}
// @Failure 400 {object} framework.Response
// @Failure 404 {object} framework.Response
// @Router /user/{id} [get]
func GetStaff(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的用户ID")
		return
	}

	u, err := Service.GetStaffByID(id)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, u)
}

// CreateStaff 创建用户
// @Summary 创建用户
// @Description 创建新用户
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param body body StaffCreate true "用户信息"
// @Success 200 {object} framework.Response{data=Staff}
// @Failure 400 {object} framework.Response
// @Router /user/create [post]
func CreateStaff(c *gin.Context) {
	var req StaffCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	u, err := Service.CreateStaffWithForm(&req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, u)
}

// UpdateStaff 更新用户
// @Summary 更新用户
// @Description 更新用户信息
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "用户ID"
// @Param body body StaffUpdate true "用户信息"
// @Success 200 {object} framework.Response{data=Staff}
// @Failure 400 {object} framework.Response
// @Router /user/update/{id} [put]
func UpdateStaff(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的用户ID")
		return
	}

	var req StaffUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	u, err := Service.UpdateStaffWithForm(id, &req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, u)
}

// DeleteStaff 删除用户
// @Summary 删除用户
// @Description 删除用户
// @Tags 用户管理
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path int true "用户ID"
// @Success 200 {object} framework.Response
// @Failure 400 {object} framework.Response
// @Router /user/delete/{id} [delete]
func DeleteStaff(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		framework.Fail(c, http.StatusBadRequest, "无效的用户ID")
		return
	}

	if err = Service.DeleteStaff(id); err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OK(c)
}
