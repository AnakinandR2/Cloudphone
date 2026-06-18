// Package proxy 是「SOCKS5 代理」业务模块的公开入口与契约。
//
// 代理池是业务层自有资源（云手机中台不存代理），按属主隔离：每条归属一个前台用户。
// 前台用户增删改查自己的代理；后台运营可查看全量代理池并删除。
// 模块实现位于 modules/proxy/internal，受 Go internal 机制保护；公开包用于 blank-import
// 触发自注册，并对外导出少量门面（供 openapi 等读写代理）。
package proxy

import proxyinternal "manager-backend/modules/proxy/internal"

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
