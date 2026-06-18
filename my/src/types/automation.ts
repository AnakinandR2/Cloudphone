/** 脚本（我的脚本 / 商店脚本，仅 Lua） */
export interface AutomationScript {
  id: number
  store: boolean
  scriptId: number // 中台 scriptId
  name: string
  description: string
  version: string
  luaContent: string
  fileName: string
  status: string // enabled / disabled
  createTime: string
  updateTime: string
}

/** 治理视图脚本（附上传者） */
export interface AdminScript extends AutomationScript {
  uploaderId: number
}

/** 新建/编辑脚本入参 */
export interface ScriptInput {
  name: string
  description: string
  luaContent: string
  fileName?: string
}

/** 周期计划 */
export interface AutomationPlan {
  id: number
  planId: number
  planUid: string
  scriptId: number // 本地脚本 id
  scriptName: string
  name: string
  frequency: string // INTERVAL / DAILY
  intervalValue: number
  executionTime: string
  startTime: string
  endTime: string
  cpIds: string[]
  status: string // NOT_STARTED / ENABLING / PAUSED / FINISHED
  createTime: string
  updateTime: string
}

/** 新建周期计划入参 */
export interface PlanInput {
  scriptId: number
  name: string
  frequency: 'INTERVAL' | 'DAILY'
  intervalValue?: number
  executionTime?: string
  startTime?: string
  endTime?: string
  cpIds: string[]
}

/** 任务索引（任务日志列表行） */
export interface AutomationTask {
  id: number
  midTaskId: number
  taskNo: string
  scriptId: number
  scriptName: string
  planId: number // 0 = 一次性
  cpId: string
  taskName: string
  trigger: string // manual / plan
  status: string // 英文枚举
  runStart: string
  runEnd: string
  createTime: string
}

/** 任务详情/报告 */
export interface TaskReportDetail {
  midTaskId: number
  taskNo: string
  status: string
  statusDesc: string
  terminal: boolean
  runDurationMs: number
  runLog: string
  result: string
  screenshotUrl: string
}
