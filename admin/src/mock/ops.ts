import { defineFakeRoute } from 'vite-plugin-fake-server/client'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}
function fail(message: string) {
  return { code: 1, message, data: null }
}

interface ProxyRec {
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

interface CloudPhoneRec {
  id: number
  user_id: number
  cp_id: string
  name: string
  status: 'CREATED' | 'STARTING' | 'RUNNING' | 'STOPPED'
  region: string
  vm_id: string
  image_id: string
  proxy_id: number
  remark: string
  created_at: string
  updated_at: string
}

const proxies: ProxyRec[] = [
  { id: 1, user_id: 1001, name: '美西节点-01', protocol: 'socks5', host: '23.105.12.34', port: 1080, username: 'proxy_us1', region: 'us-west', status: 'ok', latency: 86, egress_ip: '23.105.12.34', last_checked_at: '2026-06-02T08:12:31', remark: '主力出口', created_at: '2026-04-10T09:12:31', updated_at: '2026-06-02T08:12:31' },
  { id: 2, user_id: 1001, name: '美西节点-02', protocol: 'socks5', host: '23.105.12.35', port: 1080, username: 'proxy_us2', region: 'us-west', status: 'fail', latency: 0, egress_ip: '', last_checked_at: '2026-06-02T07:55:10', remark: '近期超时', created_at: '2026-04-10T09:15:02', updated_at: '2026-06-02T07:55:10' },
  { id: 3, user_id: 1002, name: '日本东京-01', protocol: 'socks5', host: '45.32.88.120', port: 1080, username: 'proxy_jp1', region: 'jp-tokyo', status: 'ok', latency: 142, egress_ip: '45.32.88.120', last_checked_at: '2026-06-02T08:01:44', remark: '', created_at: '2026-05-01T14:05:07', updated_at: '2026-06-02T08:01:44' },
  { id: 4, user_id: 1003, name: '新加坡-01', protocol: 'socks5', host: '139.180.55.7', port: 1080, username: 'proxy_sg1', region: 'sg', status: 'unknown', latency: 0, egress_ip: '', last_checked_at: '', remark: '新建未检测', created_at: '2026-05-28T11:40:55', updated_at: '2026-05-28T11:40:55' },
  { id: 5, user_id: 1002, name: '德国法兰克福-01', protocol: 'socks5', host: '88.198.20.99', port: 1080, username: 'proxy_de1', region: 'eu-de', status: 'ok', latency: 210, egress_ip: '88.198.20.99', last_checked_at: '2026-06-02T06:30:00', remark: '欧洲备用', created_at: '2026-05-12T18:00:21', updated_at: '2026-06-02T06:30:00' },
]
let proxySeq = 100

const phones: CloudPhoneRec[] = [
  { id: 1, user_id: 1001, cp_id: 'cp-bj-0001', name: '运营机-01', status: 'RUNNING', region: 'cn-north-1', vm_id: 'vm-bj-0001', image_id: 'img-android13-001', proxy_id: 1, remark: '日常运营', created_at: '2026-05-01T08:19:57', updated_at: '2026-06-02T08:19:57' },
  { id: 2, user_id: 1001, cp_id: 'cp-bj-0002', name: '运营机-02', status: 'STOPPED', region: 'cn-north-1', vm_id: 'vm-bj-0001', image_id: 'img-android13-001', proxy_id: 0, remark: '', created_at: '2026-05-02T10:31:02', updated_at: '2026-06-01T22:10:02' },
  { id: 3, user_id: 1002, cp_id: 'cp-sh-0001', name: '测试机-01', status: 'STARTING', region: 'cn-east-2', vm_id: 'vm-sh-0001', image_id: 'img-android12-002', proxy_id: 3, remark: '启动中', created_at: '2026-05-05T15:42:33', updated_at: '2026-06-02T09:00:01' },
  { id: 4, user_id: 1003, cp_id: '', name: '待分配实例', status: 'CREATED', region: 'cn-south-1', vm_id: 'vm-gz-0001', image_id: 'img-android14-004', proxy_id: 0, remark: '刚创建未就绪', created_at: '2026-06-01T18:00:21', updated_at: '2026-06-01T18:00:21' },
  { id: 5, user_id: 1002, cp_id: 'cp-sh-0002', name: '测试机-02', status: 'RUNNING', region: 'cn-east-2', vm_id: 'vm-sh-0002', image_id: 'img-android12-002', proxy_id: 5, remark: '', created_at: '2026-05-06T09:11:08', updated_at: '2026-06-02T07:11:08' },
]
let phoneSeq = 100

function paginate<T>(list: T[], page: number, size: number) {
  const total = list.length
  const start = (page - 1) * size
  return { list: list.slice(start, start + size), total }
}

export default defineFakeRoute([
  {
    url: '/v1/admin/proxies/list',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 20
      const kw = ((query.kw as string) || '').trim()
      const status = (query.status as string) || ''
      let list = proxies
      if (kw) {
        list = list.filter(p =>
          p.name.includes(kw)
          || p.host.includes(kw)
          || p.egress_ip.includes(kw)
          || p.region.includes(kw)
          || p.username.includes(kw),
        )
      }
      if (status) list = list.filter(p => p.status === status)
      return ok(paginate(list, page, size))
    },
  },
  {
    url: '/v1/admin/proxies/:id',
    method: 'get',
    response: ({ params }) => {
      const rec = proxies.find(p => p.id === Number(params.id))
      return rec ? ok(rec) : fail('代理不存在')
    },
  },
  {
    url: '/v1/admin/proxies/delete/:id',
    method: 'delete',
    response: ({ params }) => {
      const idx = proxies.findIndex(p => p.id === Number(params.id))
      if (idx === -1) return fail('代理不存在')
      proxies.splice(idx, 1)
      void proxySeq
      return ok(null)
    },
  },
  {
    url: '/v1/admin/phones/list',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 20
      const kw = ((query.kw as string) || '').trim()
      const status = (query.status as string) || ''
      const userId = query.userId ? Number(query.userId) : 0
      let list = phones
      if (kw) {
        list = list.filter(p =>
          p.name.includes(kw)
          || p.cp_id.includes(kw)
          || p.region.includes(kw)
          || p.vm_id.includes(kw),
        )
      }
      if (status) list = list.filter(p => p.status === status)
      if (userId) list = list.filter(p => p.user_id === userId)
      return ok(paginate(list, page, size))
    },
  },
  {
    url: '/v1/admin/phones/:id',
    method: 'get',
    response: ({ params }) => {
      const rec = phones.find(p => p.id === Number(params.id))
      return rec ? ok(rec) : fail('实例不存在')
    },
  },
  {
    url: '/v1/admin/phones/delete/:id',
    method: 'delete',
    response: ({ params }) => {
      const idx = phones.findIndex(p => p.id === Number(params.id))
      if (idx === -1) return fail('实例不存在')
      phones.splice(idx, 1)
      void phoneSeq
      return ok(null)
    },
  },
])
