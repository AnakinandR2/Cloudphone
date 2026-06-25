/** 参数类型（覆盖全 JSON 类型 + enum 下拉） */
export type ParamType = 'string' | 'number' | 'boolean' | 'enum' | 'array' | 'object'

/** 单个参数定义（脚本 params_schema 数组项） */
export interface ParamSpec {
  key: string
  label?: string
  type: ParamType
  required?: boolean
  default?: unknown
  description?: string
  options?: string[] // 仅 enum：候选值
}

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
  params?: Record<string, unknown> // 计划级共用参数（无逐台）
}

/** 一次性运行入参 */
export interface RunTaskInput {
  scriptId: number
  cpIds: string[]
  taskName?: string
  params?: Record<string, unknown> // 共用参数
  perPhoneParams?: Record<string, Record<string, unknown>> // 逐台覆盖：cpId → 覆盖值
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
