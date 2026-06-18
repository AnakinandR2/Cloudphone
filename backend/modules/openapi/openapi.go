// Package openapi 提供「API 密钥 + 开放 REST API」。
//
// 前台用户签发/撤销 API 密钥（库内只存 sha256，明文仅创建时返回一次）；开放 API 在
// /api/v1/open/v1/* 下用 Bearer 密钥鉴权，处理器经各业务模块门面委托（openapi → phone/automation）。
// 模块实现位于 modules/openapi/internal，受 Go internal 机制保护；本包仅用于 blank-import 触发自注册。
package openapi

import (
	"manager-backend/framework/apperr"
	openapiinternal "manager-backend/modules/openapi/internal"
)

// Authenticate 校验明文 API 密钥并返回属主 userID（供 mcp 等通道复用同一套密钥鉴权）。
// 失败返回 401 风格的 apperr，便于上层统一处理。
func Authenticate(rawKey string) (int, error) {
	uid, ok := openapiinternal.Service.Authenticate(rawKey)
	if !ok {
		return 0, apperr.Unauthorized("API 密钥无效或已撤销")
	}
	return uid, nil
}
