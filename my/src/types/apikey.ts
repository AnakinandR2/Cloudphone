/** API 密钥（安全视图，不含明文） */
export interface ApiKey {
  id: number
  name: string
  masked: string // gp_live_3f2a••••a17c
  status: string // active / revoked
  lastUsedAt: string | null
  createdAt: string
}

/** 创建密钥返回：安全视图 + 完整明文（仅此一次） */
export interface ApiKeyCreated extends ApiKey {
  fullKey: string
}
