import { defineFakeRoute } from 'vite-plugin-fake-server/client'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}
function fail(message: string) {
  return { code: 1, message, data: null }
}
function now() {
  return new Date().toISOString().slice(0, 19).replace('T', ' ')
}

interface ProxyRec {
  id: number
  user_id: number
  name: string
  protocol: string
  host: string
  port: number
  username: string
  password?: string
  region: string
  status: string
  latency: number
  egress_ip: string
  last_checked_at: string
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

// 内存代理表（演示用，归属 user 1）
const proxies: ProxyRec[] = Array.from({ length: 5 }).map((_, i) => ({
  id: 5 - i,
  user_id: 1,
  name: `代理 ${5 - i}`,
  protocol: 'socks5',
  host: `10.0.0.${5 - i}`,
  port: 1080 + (5 - i),
  username: `user${5 - i}`,
  region: '',
  status: 'unknown',
  latency: 0,
  egress_ip: '',
  last_checked_at: '',
  country: '',
  city: '',
  asn: '',
  asn_name: '',
  company: '',
  conn_type: '',
  remark: '',
  created_at: `2026-0${(i % 9) + 1}-1${i % 9} 10:30:00`,
  updated_at: `2026-0${(i % 9) + 1}-1${i % 9} 10:30:00`,
}))
let seq = 2000

// 出参剔除 password
function view(p: ProxyRec) {
  const { password, ...rest } = p
  void password
  return rest
}

export default defineFakeRoute([
  {
    url: '/v1/proxy/list',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 10
      const kw = (query.kw as string) || ''
      let list = proxies
      if (kw) {
        list = list.filter(p =>
          p.name.includes(kw) || p.host.includes(kw) || p.region.includes(kw),
        )
      }
      const total = list.length
      const start = (page - 1) * size
      return ok({ list: list.slice(start, start + size).map(view), total })
    },
  },
  {
    url: '/v1/proxy/options',
    method: 'get',
    response: () => ok(proxies.filter(p => p.status !== 'fail').map(view)),
  },
  {
    url: '/v1/proxy/:id',
    method: 'get',
    response: ({ params }) => {
      const p = proxies.find(x => x.id === Number(params.id))
      return p ? ok(view(p)) : fail('代理不存在')
    },
  },
  {
    url: '/v1/proxy/create',
    method: 'post',
    response: ({ body }) => {
      if (!body?.name) return fail('请填写名称')
      if (!body?.host) return fail('请填写主机')
      if (!body?.port) return fail('请填写端口')
      const rec: ProxyRec = {
        id: ++seq,
        user_id: 1,
        name: body.name,
        protocol: body.protocol || 'socks5',
        host: body.host,
        port: Number(body.port),
        username: body.username || '',
        password: body.password || '',
        region: body.region || '',
        status: 'unknown',
        latency: 0,
        egress_ip: '',
        last_checked_at: '',
        country: '',
        city: '',
        asn: '',
        asn_name: '',
        company: '',
        conn_type: '',
        remark: body.remark || '',
        created_at: now(),
        updated_at: now(),
      }
      proxies.unshift(rec)
      return ok(view(rec))
    },
  },
  {
    url: '/v1/proxy/probe',
    method: 'post',
    response: ({ body }) => {
      if (!body?.host || !body?.port)
        return fail('请填写主机与端口')
      const okResult = String(body.host).length % 5 !== 0 // 演示：随主机长度决定成败
      if (okResult) {
        return ok({
          status: 'ok',
          latency: 60 + Math.floor(Math.random() * 180),
          egress_ip: '156.59.87.47',
          country: 'CN',
          city: 'Hong Kong',
          asn: 'AS21859',
          asn_name: 'Zenlayer Inc',
          company: 'Zenlayer IP Block @Hong Kong',
          conn_type: 'Corporate',
          message: '',
        })
      }
      return ok({
        status: 'fail',
        latency: 0,
        egress_ip: '',
        country: '',
        city: '',
        asn: '',
        asn_name: '',
        company: '',
        conn_type: '',
        message: '经代理访问失败: connect timeout',
      })
    },
  },
  {
    url: '/v1/proxy/batch',
    method: 'post',
    response: ({ body }) => {
      const items: any[] = Array.isArray(body?.proxies) ? body.proxies : []
      let created = 0
      for (const it of items) {
        if (!it?.host || !it?.port)
          continue
        proxies.unshift({
          id: ++seq,
          user_id: 1,
          name: it.name || `${it.host}:${it.port}`,
          protocol: it.protocol || 'socks5',
          host: it.host,
          port: Number(it.port),
          username: it.username || '',
          password: it.password || '',
          region: '',
          status: 'unknown',
          latency: 0,
          egress_ip: '',
          last_checked_at: '',
          country: '',
          city: '',
          asn: '',
          asn_name: '',
          company: '',
          conn_type: '',
          remark: it.remark || '',
          created_at: now(),
          updated_at: now(),
        })
        created++
      }
      return ok({ created, total: items.length })
    },
  },
  {
    url: '/v1/proxy/update/:id',
    method: 'put',
    response: ({ params, body }) => {
      const p = proxies.find(x => x.id === Number(params.id))
      if (!p) return fail('代理不存在')
      if (body?.name !== undefined && body.name !== '') p.name = body.name
      if (body?.protocol !== undefined && body.protocol !== '') p.protocol = body.protocol
      if (body?.host !== undefined && body.host !== '') p.host = body.host
      if (body?.port !== undefined && body.port) p.port = Number(body.port)
      if (body?.username !== undefined) p.username = body.username
      if (body?.password !== undefined && body.password !== '') p.password = body.password
      if (body?.region !== undefined) p.region = body.region
      if (body?.remark !== undefined) p.remark = body.remark
      p.updated_at = now()
      return ok(view(p))
    },
  },
  {
    url: '/v1/proxy/:id/test',
    method: 'post',
    response: ({ params }) => {
      const p = proxies.find(x => x.id === Number(params.id))
      if (!p) return fail('代理不存在')
      // 演示：八成成功，写回出口 IP + 延迟 + 归属信息
      const okResult = (p.id % 5) !== 0
      p.last_checked_at = now()
      p.updated_at = now()
      if (okResult) {
        p.status = 'ok'
        p.latency = 60 + Math.floor(Math.random() * 180)
        p.egress_ip = '156.59.87.47'
        p.country = 'CN'
        p.city = 'Hong Kong'
        p.region = 'CN Hong Kong'
        p.asn = 'AS21859'
        p.asn_name = 'Zenlayer Inc'
        p.company = 'Zenlayer IP Block @Hong Kong'
        p.conn_type = 'Corporate'
      }
      else {
        p.status = 'fail'
        p.latency = 0
        p.egress_ip = ''
      }
      return ok(view(p))
    },
  },
  {
    url: '/v1/proxy/delete/:id',
    method: 'delete',
    response: ({ params }) => {
      const idx = proxies.findIndex(x => x.id === Number(params.id))
      if (idx === -1) return fail('代理不存在')
      proxies.splice(idx, 1)
      return ok(null)
    },
  },
])
