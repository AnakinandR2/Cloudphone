// Package partner 是「代理IP合作商」业务模块的公开入口。
//
// 合作商是运营全局内容（非按用户隔离）：admin 维护名称/logo/介绍/配图/推广链接/排序/启用；
// my、www 展示并引导点击推广链接，点击明细落库用于收益归因分析（外链跳转 + 点击计数）。
// 模块实现位于 modules/partner/internal，受 Go internal 机制保护；本包仅用于 blank-import
// 触发自注册（见 internal 的 init）。
package partner

import _ "manager-backend/modules/partner/internal"
