// Package proxy 是「SOCKS5 代理」业务模块的公开入口与契约。
//
// 代理池是业务层自有资源（云手机中台不存代理），按属主隔离：每条归属一个前台用户。
// 前台用户增删改查自己的代理；后台运营可查看全量代理池并删除。
// 模块实现位于 modules/proxy/internal，受 Go internal 机制保护。
package proxy

import _ "manager-backend/modules/proxy/internal"
