/** SOCKS5 代理池实体（管理侧只读 + 删除；后端不出参 password） */
export interface Proxy {
  id: number
  user_id: number
  name: string
  protocol: string
  host: string
  port: number
  username: string
  region: string
  status: 'unknown' | 'ok' | 'fail'
  latency: number
  egress_ip: string
  last_checked_at: string
  remark: string
  created_at: string
  updated_at: string
}

export interface ProxyListParams {
  page: number
  size: number
  kw?: string
  status?: string
}

export interface ProxyListResult {
  list: Proxy[]
  total: number
}
