/** 参数类型。table = 普通 Lua table（值为 Lua table 字面量文本，支持数组与 {[k]=v} 两种写法） */
export type ParamType = 'string' | 'number' | 'boolean' | 'table'

/** 单个参数定义（脚本顶部注释里的一项） */
export interface ParamSpec {
  key: string
  type: ParamType
  required?: boolean
  default?: unknown // table 为 Lua 字面量文本
  description?: string
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
