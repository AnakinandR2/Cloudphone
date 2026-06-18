import type {
  AutomationPlan,
  AutomationScript,
  AutomationTask,
  PlanInput,
  ScriptInput,
  TaskReportDetail,
} from '@/types/automation'
import api from '../index'

interface R<T> { code: number, message: string, data: T }
interface Page<T> { list: T[], total: number }

// 自动化：脚本 / 计划 / 任务，归属当前登录前台用户。
export default {
  // ---- 脚本 ----
  listScripts: () => api.get<unknown, R<AutomationScript[]>>('automation/scripts'),
  storeScripts: () => api.get<unknown, R<AutomationScript[]>>('automation/scripts/store'),
  usableScripts: () => api.get<unknown, R<AutomationScript[]>>('automation/scripts/usable'),
  createScript: (body: ScriptInput) => api.post<unknown, R<AutomationScript>>('automation/scripts', body),
  updateScript: (id: number, body: ScriptInput) => api.put<unknown, R<AutomationScript>>(`automation/scripts/${id}`, body),
  toggleScript: (id: number, enabled: boolean) => api.post<unknown, R<null>>(`automation/scripts/${id}/toggle`, { enabled }),
  deleteScript: (id: number) => api.delete<unknown, R<null>>(`automation/scripts/${id}`),

  // ---- 计划 ----
  listPlans: () => api.get<unknown, R<AutomationPlan[]>>('automation/plans'),
  createPlan: (body: PlanInput) => api.post<unknown, R<AutomationPlan>>('automation/plans', body),
  startPlan: (id: number) => api.post<unknown, R<null>>(`automation/plans/${id}/start`),
  pausePlan: (id: number) => api.post<unknown, R<null>>(`automation/plans/${id}/pause`),
  deletePlan: (id: number) => api.delete<unknown, R<null>>(`automation/plans/${id}`),

  // ---- 任务 ----
  runTask: (scriptId: number, cpIds: string[], taskName?: string) =>
    api.post<unknown, R<AutomationTask[]>>('automation/tasks/run', { scriptId, cpIds, taskName }),
  listTasks: (params: { page: number, size: number, status?: string }) =>
    api.get<unknown, R<Page<AutomationTask>>>('automation/tasks', { params }),
  taskDetail: (midTaskId: number | string) =>
    api.get<unknown, R<TaskReportDetail>>(`automation/tasks/${midTaskId}`),
}
