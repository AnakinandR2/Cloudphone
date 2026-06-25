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

/** 脚本（商店/用户，仅 Lua） */
export interface AutomationScript {
  id: number
  store: boolean
  scriptId: number
  name: string
  description: string
  version: string
  luaContent: string
  fileName: string
  status: string // enabled / disabled
  createTime: string
  updateTime: string
}

/** 治理视图脚本（附上传者 ID） */
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
