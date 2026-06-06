export interface Proxy {
  id: number
  user_id: number
  name: string
  protocol: string
  host: string
  port: number
  username: string
  region: string
  status: string
  latency: number
  egress_ip: string
  last_checked_at: string
  // 测试代理后自动识别的出口归属信息
  country: string
  city: string
  asn: string
  asn_name: string
  company: string
  conn_type: string
  remark: string
  created_at: string
  updated_at: string
}

// 即时探测结果（添加/编辑表单测试用，不落库）
export interface ProbeOutcome {
  status: string
  latency: number
  egress_ip: string
  country: string
  city: string
  asn: string
  asn_name: string
  company: string
  conn_type: string
  message: string
}

export interface ProxyCreate {
  name: string
  protocol: string
  host: string
  port: number
  username: string
  password: string
  region: string
  remark: string
}

export interface ProxyUpdate {
  name?: string
  protocol?: string
  host?: string
  port?: number
  username?: string
  password?: string
  region?: string
  remark?: string
}

export interface ProxyListParams {
  page: number
  size: number
  kw?: string
}

export interface ProxyListResult {
  list: Proxy[]
  total: number
}
