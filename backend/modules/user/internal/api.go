package user

import (
	"net/http"

	"manager-backend/framework"
	"manager-backend/framework/auth"

	"github.com/gin-gonic/gin"
)

// issueToken 为前台用户签发 user 身份域令牌（写入当前令牌版本，使用前台密钥，见 jwtSecret）。
func issueToken(c *User) (string, error) {
	return auth.Generate(c.ID, c.Phone, auth.ScopeUser, c.TokenVersion,
		jwtSecret(), framework.AppConfig.JWTExpireHours)
}

// Register 前台用户注册
// @Summary 前台用户注册
// @Description 注册前台用户并直接返回登录令牌
// @Tags 前台用户
// @Accept json
// @Produce json
// @Param body body RegisterRequest true "注册信息"
// @Success 200 {object} framework.Response{data=LoginResponse}
// @Failure 400 {object} framework.Response
// @Failure 409 {object} framework.Response
// @Router /user/auth/register [post]
func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	cust, err := Service.Register(&req)
	if err != nil {
		framework.FailErr(c, err)
		return
	}

	token, err := issueToken(cust)
	if err != nil {
		framework.Fail(c, http.StatusInternalServerError, "生成Token失败")
		return
	}
	setCookie(c, token)
	framework.OKWithData(c, LoginResponse{
		ID: cust.ID, Phone: cust.Phone, Nickname: cust.Nickname, Avatar: cust.Avatar, Token: token,
	})
}

// Login 前台用户登录
// @Summary 前台用户登录
// @Description 手机号+密码登录，返回 user 身份域 JWT
// @Tags 前台用户
// @Accept json
// @Produce json
// @Param body body LoginRequest true "登录信息"
// @Success 200 {object} framework.Response{data=LoginResponse}
// @Failure 400 {object} framework.Response
// @Failure 401 {object} framework.Response
// @Router /user/auth/login [post]
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		framework.Fail(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	cust, err := Service.Authenticate(req.Phone, req.Password)
	if err != nil {
		framework.FailErr(c, err)
		return
	}

	token, err := issueToken(cust)
	if err != nil {
		framework.Fail(c, http.StatusInternalServerError, "生成Token失败")
		return
	}
	setCookie(c, token)
	framework.OKWithData(c, LoginResponse{
		ID: cust.ID, Phone: cust.Phone, Nickname: cust.Nickname, Avatar: cust.Avatar, Token: token,
	})
}

// Logout 前台登出
// @Summary 前台用户登出
// @Description 递增令牌版本（令该用户已签发的所有令牌立即失效）并清除前台 Cookie。
// @Tags 前台用户
// @Produce json
// @Security Bearer
// @Success 200 {object} framework.Response
// @Router /user/auth/logout [post]
func Logout(c *gin.Context) {
	if userID, ok := c.Get("userID"); ok {
		if err := Service.Logout(userID.(int)); err != nil {
			framework.FailErr(c, err)
			return
		}
	}
	clearCookie(c)
	framework.OK(c)
}

// GetProfile 获取当前前台用户信息
// @Summary 获取当前前台用户信息
// @Tags 前台用户
// @Produce json
// @Security Bearer
// @Success 200 {object} framework.Response{data=User}
// @Failure 401 {object} framework.Response
// @Router /user/me [get]
func GetProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		framework.Fail(c, http.StatusUnauthorized, "未授权")
		return
	}

	cust, err := Service.GetByID(userID.(int))
	if err != nil {
		framework.FailErr(c, err)
		return
	}
	framework.OKWithData(c, cust)
}
