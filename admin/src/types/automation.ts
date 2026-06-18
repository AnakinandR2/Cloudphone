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
