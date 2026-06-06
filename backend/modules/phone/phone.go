// Package phone 是「云手机实例」业务模块的公开入口与契约。
//
// 业务层维护云手机实例的归属与档案（业务库自有记录，含 owner/状态/绑定代理等），
// 按属主隔离：每台归属一个前台用户。前台用户增删改查自己的云手机；后台运营查看全量并可强制删除。
// 实例的实际开通/生命周期（调用云手机中台异步创建等）属后续阶段，本模块先承载档案与列表。
// 模块实现位于 modules/phone/internal，受 Go internal 机制保护。
package phone

import _ "manager-backend/modules/phone/internal"
