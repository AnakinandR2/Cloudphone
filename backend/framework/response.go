package framework

import (
	"net/http"

	"manager-backend/framework/apperr"

	"github.com/gin-gonic/gin"
)

// Response 统一响应格式
type Response struct {
	Code    int         `json:"code" example:"0"`
	Message string      `json:"message" example:"成功"`
	Data    interface{} `json:"data,omitempty"`
}

// PageResponse 分页响应
type PageResponse struct {
	List  interface{} `json:"list"`
	Total int64       `json:"total"`
}

func OK(c *gin.Context) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: "成功"})
}

func OKWithData(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: "成功", Data: data})
}

func OKWithPage(c *gin.Context, list interface{}, total int64) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "成功",
		Data:    PageResponse{List: list, Total: total},
	})
}

func Fail(c *gin.Context, httpCode int, message string) {
	c.JSON(httpCode, Response{Code: httpCode, Message: message})
}

func FailWithCode(c *gin.Context, httpCode int, bizCode int, message string) {
	c.JSON(httpCode, Response{Code: bizCode, Message: message})
}

// FailErr 按领域错误（apperr）的类别映射 HTTP 状态码并返回统一响应。
// 非 apperr 错误一律按 500 处理。Code 字段沿用 HTTP 状态码，与 Fail 保持一致。
func FailErr(c *gin.Context, err error) {
	httpCode := apperr.HTTPStatus(apperr.KindOf(err))
	c.JSON(httpCode, Response{Code: httpCode, Message: err.Error()})
}
