/** 云手机实例实体（管理侧只读 + 删除） */
export interface Tag {
  name: string
  color: string
}

export interface CloudPhone {
  id: number
  user_id: number
  cp_id: string
  name: string
  status: 'CREATING' | 'CREATE_FAILED' | 'CREATED' | 'STARTING' | 'RUNNING' | 'STOPPING' | 'STOPPED' | 'DESTROYING'
  region: string
  vm_id: string
  image_id: string
  proxy_id: number
  remark: string
  tags?: Tag[]
  created_at: string
  updated_at: string
}

export interface CloudPhoneListParams {
  page: number
  size: number
  kw?: string
  status?: string
  tag?: string
  userId?: number
}

export interface CloudPhoneListResult {
  list: CloudPhone[]
  total: number
}
