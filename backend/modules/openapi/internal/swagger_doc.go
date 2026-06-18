// swagger_doc.go 仅承载「开放 API」独立 Swagger 文档的总信息（general info）。
// 用 `swag init -g modules/openapi/internal/swagger_doc.go --instanceName openapi` 生成，
// 见 CLAUDE.md「开放 API 独立 Swagger」。这里不含任何运行时代码。
//
// @title        Gloryphone 开放 API
// @version      1.0
// @description  用 API 密钥（Authorization: Bearer <key>）调用的云手机开放接口。密钥在「我的 · API 与 MCP」里签发。
// @BasePath     /api/open/v1
// @securityDefinitions.apikey ApiKey
// @in header
// @name Authorization
// @description Authorization: Bearer gp_live_xxxxxxxx
package openapi
