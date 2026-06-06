// Package note 是 note 业务模块的公开入口。
//
// 模块实现位于 modules/note/internal，受 Go internal 机制保护；公开包仅用于在 main 中
// blank-import 以触发自注册。如需对外发布契约（供其他模块依赖），在此再导出。
package note

import _ "manager-backend/modules/note/internal"
