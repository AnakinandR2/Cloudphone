import type { AdminScript, AutomationScript, ScriptInput } from '@/types/automation'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

// 运营侧自动化脚本：商店脚本管理 + 用户脚本治理（需 staff 权限 script:view / script:manage）
export default {
  // ---- 商店脚本 ----
  storeList: () => api.get<unknown, R<AutomationScript[]>>('admin/automation/store'),
  storeCreate: (body: ScriptInput) => api.post<unknown, R<AutomationScript>>('admin/automation/store', body),
  storeUpdate: (id: number, body: ScriptInput) => api.put<unknown, R<AutomationScript>>(`admin/automation/store/${id}`, body),
  storeToggle: (id: number, enabled: boolean) => api.post<unknown, R<null>>(`admin/automation/store/${id}/toggle`, { enabled }),
  storeDelete: (id: number) => api.delete<unknown, R<null>>(`admin/automation/store/${id}`),

  // ---- 用户脚本治理 ----
  userScripts: () => api.get<unknown, R<AdminScript[]>>('admin/automation/user-scripts'),
  userToggle: (id: number, enabled: boolean) => api.post<unknown, R<null>>(`admin/automation/user-scripts/${id}/toggle`, { enabled }),
  userDelete: (id: number) => api.delete<unknown, R<null>>(`admin/automation/user-scripts/${id}`),
}
