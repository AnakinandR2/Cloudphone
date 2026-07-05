// Package proxy 是「SOCKS5 代理」业务模块的公开入口与契约。
//
// 代理池是业务层自有资源（云手机中台不存代理），按属主隔离：每条归属一个前台用户。
// 前台用户增删改查自己的代理；后台运营可查看全量代理池并删除。
// 模块实现位于 modules/proxy/internal，受 Go internal 机制保护；公开包用于 blank-import
// 触发自注册，并对外导出少量门面（供 openapi 等读写代理）。
package proxy

import (
	"context"

	proxyinternal "manager-backend/modules/proxy/internal"

	"gorm.io/gorm"
)

// Proxy 是代理记录的公开别名（供 openapi 等引用返回类型；Password 字段 json:"-" 永不外泄）。
type Proxy = proxyinternal.Proxy

// ProxyInput 是创建/编辑代理的入参（门面统一形态）。
type ProxyInput struct {
	Name     string
	Protocol string
	Host     string
	Port     int
	Username string
	Password string
	Region   string
	Remark   string
}

// List 返回某用户的全部代理。
func List(userID int) ([]Proxy, error) {
	return proxyinternal.ProxyService.ListOptions(userID)
}

// Create 新增一条代理。
func Create(userID int, in ProxyInput) (*Proxy, error) {
	return proxyinternal.ProxyService.Create(userID, &proxyinternal.ProxyCreate{
		Name: in.Name, Protocol: in.Protocol, Host: in.Host, Port: in.Port,
		Username: in.Username, Password: in.Password, Region: in.Region, Remark: in.Remark,
	})
}

// Update 编辑一条代理（留空字段不更新，沿用 service 语义）。
func Update(userID, id int, in ProxyInput) (*Proxy, error) {
	return proxyinternal.ProxyService.Update(userID, id, &proxyinternal.ProxyUpdate{
		Name: in.Name, Protocol: in.Protocol, Host: in.Host, Port: in.Port,
		Username: in.Username, Password: in.Password, Region: in.Region, Remark: in.Remark,
	})
}

// Delete 删除一条代理。
func Delete(userID, id int) error {
	return proxyinternal.ProxyService.Delete(userID, id)
}

// GetByID 返回某用户名下指定 id 的代理；不存在或非属主一律 apperr.NotFound（隐藏他人资源）。
// 供跨模块（如云手机绑定代理）做属主 + 存在性校验，杜绝 BOLA/IDOR。
func GetByID(userID, id int) (*Proxy, error) {
	return proxyinternal.ProxyService.GetByID(userID, id)
}

// ProbeOwned 对本人某代理做一次连通性探测（不落库）。供云手机「开机前短超时代理探测」复用：
// 调用方用带较短超时的 ctx 控制耗时；返回 nil=连通、err=不可用。
func ProbeOwned(ctx context.Context, userID, id int) error {
	return proxyinternal.ProxyService.ProbeOwned(ctx, userID, id)
}

// InitForTest 测试用：供其他模块装配 proxy（建表 + 装配服务）。
func InitForTest(db *gorm.DB) error { return proxyinternal.InitForTest(db) }
