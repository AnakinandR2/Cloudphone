export interface AccessLogListItem {
  id: number
  user_id: number
  username: string
  method: string
  path: string
  status_code: number
  latency_ms: number
  client_ip: string
  created_at: string
}

export interface AccessLog extends AccessLogListItem {
  user_agent: string
  request_headers: string
  request_body: string
  response_headers: string
  response_body: string
}

export interface AccessLogListParams {
  page: number
  size: number
  username?: string
  method?: string
  path?: string
  status_group?: string
  start_time?: string
  end_time?: string
}

export interface AccessLogListResult {
  // Mock 列表直接返回完整记录（含头/体），便于行展开展示
  list: AccessLog[]
  total: number
}
