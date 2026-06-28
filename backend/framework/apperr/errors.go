// Package apperr 定义领域错误类型及其到 HTTP 状态码的映射。
// 业务层（service）返回这些类型化错误，由 framework.FailErr 在 HTTP 边界统一翻译，
// 避免 handler 硬编码状态码、避免 service 直接依赖 gin。
package apperr

import (
	"errors"
	"net/http"
)

// Kind 领域错误类别
type Kind int

const (
	KindInternal        Kind = iota // 未归类的内部错误 → 500
	KindBadRequest                  // 一般性请求错误 → 400
	KindNotFound                    // 资源不存在 → 404
	KindConflict                    // 唯一性/状态冲突 → 409
	KindValidation                  // 入参/业务校验失败 → 422
	KindUnauthorized                // 未认证 → 401
	KindForbidden                   // 已认证但无权限 → 403
	KindUnavailable                 // 依赖未配置/暂不可用 → 503
	KindPayloadTooLarge             // 请求体超限 → 413
	KindUpstream                    // 上游依赖（如 S3）失败 → 502
)

// Error 携带类别与可直接展示给调用方的消息。
type Error struct {
	Kind Kind
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

// New 构造指定类别的领域错误。
func New(kind Kind, msg string) *Error { return &Error{Kind: kind, Msg: msg} }

// 各类别快捷构造器
func BadRequest(msg string) *Error   { return &Error{Kind: KindBadRequest, Msg: msg} }
func NotFound(msg string) *Error     { return &Error{Kind: KindNotFound, Msg: msg} }
func Conflict(msg string) *Error     { return &Error{Kind: KindConflict, Msg: msg} }
func Validation(msg string) *Error   { return &Error{Kind: KindValidation, Msg: msg} }
func Unauthorized(msg string) *Error { return &Error{Kind: KindUnauthorized, Msg: msg} }
func Forbidden(msg string) *Error    { return &Error{Kind: KindForbidden, Msg: msg} }
func Internal(msg string) *Error     { return &Error{Kind: KindInternal, Msg: msg} }

// PayloadTooLarge 请求体超限（如上传文件超过限额）→ 413。
func PayloadTooLarge(msg string) *Error { return &Error{Kind: KindPayloadTooLarge, Msg: msg} }

// Upstream 上游依赖（如对象存储/中台）失败 → 502。
func Upstream(msg string) *Error { return &Error{Kind: KindUpstream, Msg: msg} }

// KindOf 提取错误类别；非领域错误一律视为内部错误。
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindInternal
}

// HTTPStatus 将类别映射为 HTTP 状态码。
func HTTPStatus(k Kind) int {
	switch k {
	case KindBadRequest:
		return http.StatusBadRequest
	case KindNotFound:
		return http.StatusNotFound
	case KindConflict:
		return http.StatusConflict
	case KindValidation:
		return http.StatusUnprocessableEntity
	case KindUnauthorized:
		return http.StatusUnauthorized
	case KindForbidden:
		return http.StatusForbidden
	case KindUnavailable:
		return http.StatusServiceUnavailable
	case KindPayloadTooLarge:
		return http.StatusRequestEntityTooLarge
	case KindUpstream:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}
