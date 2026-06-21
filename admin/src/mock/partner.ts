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

interface PartnerRec {
  id: number
  name: string
  logo_url: string
  image_url: string
  intro: string
  promo_url: string
  sort: number
  enabled: boolean
  click_count: number
  created_at: string
  updated_at: string
}

const items: PartnerRec[] = [
  {
    id: 1,
    name: '亮数代理',
    logo_url: 'https://picsum.photos/seed/partner1/80',
    image_url: 'https://picsum.photos/seed/partnerimg1/320/160',
    intro: '全球住宅 IP，覆盖 190+ 国家，按量计费，适合数据采集与多账号运营。',
    promo_url: 'https://partner1.example.com/?ref=glory',
    sort: 1,
    enabled: true,
    click_count: 128,
    created_at: '2026-05-01 10:00:00',
    updated_at: '2026-05-01 10:00:00',
  },
  {
    id: 2,
    name: '极速 SOCKS5',
    logo_url: 'https://picsum.photos/seed/partner2/80',
    image_url: 'https://picsum.photos/seed/partnerimg2/320/160',
    intro: '高匿 SOCKS5 静态住宅，低延迟、稳定不掉线，支持指纹浏览器对接。',
    promo_url: 'https://partner2.example.com/?ref=glory',
    sort: 2,
    enabled: true,
    click_count: 56,
    created_at: '2026-05-02 10:00:00',
    updated_at: '2026-05-02 10:00:00',
  },
  {
    id: 3,
    name: '停用示例',
    logo_url: '',
    image_url: '',
    intro: '已下架的合作商，仅 admin 可见，不会出现在推荐列表。',
    promo_url: 'https://partner3.example.com/?ref=glory',
    sort: 99,
    enabled: false,
    click_count: 0,
    created_at: '2026-05-03 10:00:00',
    updated_at: '2026-05-03 10:00:00',
  },
]
let seq = 1000

function sorted(list: PartnerRec[]) {
  return [...list].sort((a, b) => a.sort - b.sort || a.id - b.id)
}

export default defineFakeRoute([
  // admin：全量列表
  {
    url: '/v1/admin/partners',
    method: 'get',
    response: ({ query }) => {
      const kw = (query.kw as string) || ''
      let list = sorted(items)
      if (kw)
        list = list.filter(it => it.name.includes(kw) || it.intro.includes(kw))
      return ok({ list, total: list.length })
    },
  },
  // admin：创建
  {
    url: '/v1/admin/partners',
    method: 'post',
    response: ({ body }) => {
      const rec: PartnerRec = {
        id: ++seq,
        name: body.name,
        logo_url: body.logo_url || '',
        image_url: body.image_url || '',
        intro: body.intro || '',
        promo_url: body.promo_url,
        sort: Number(body.sort) || 0,
        enabled: body.enabled !== false,
        click_count: 0,
        created_at: now(),
        updated_at: now(),
      }
      items.push(rec)
      return ok(rec)
    },
  },
  // admin：更新
  {
    url: '/v1/admin/partners/:id',
    method: 'put',
    response: ({ params, body }) => {
      const it = items.find(x => x.id === Number(params.id))
      if (!it)
        return fail('合作商不存在')
      if (body.name !== undefined) it.name = body.name
      if (body.promo_url !== undefined) it.promo_url = body.promo_url
      if (body.logo_url !== undefined) it.logo_url = body.logo_url
      if (body.image_url !== undefined) it.image_url = body.image_url
      if (body.intro !== undefined) it.intro = body.intro
      if (body.sort !== undefined) it.sort = Number(body.sort) || 0
      if (body.enabled !== undefined) it.enabled = !!body.enabled
      it.updated_at = now()
      return ok(it)
    },
  },
  // admin：删除
  {
    url: '/v1/admin/partners/:id',
    method: 'delete',
    response: ({ params }) => {
      const idx = items.findIndex(x => x.id === Number(params.id))
      if (idx === -1)
        return fail('合作商不存在')
      items.splice(idx, 1)
      return ok(null)
    },
  },
  // admin：上传图片（mock 直接回一张随机图）
  {
    url: '/v1/admin/partners/upload',
    method: 'post',
    response: () => ok({ url: `https://picsum.photos/seed/up${++seq}/240` }),
  },
  // admin：点击明细
  {
    url: '/v1/admin/partners/:id/clicks',
    method: 'get',
    response: ({ params }) => {
      const pid = Number(params.id)
      const list = Array.from({ length: 3 }).map((_, i) => ({
        id: i + 1,
        partner_id: pid,
        user_id: i === 0 ? 0 : 1000 + i,
        ip: `203.0.113.${i + 1}`,
        user_agent: 'Mozilla/5.0',
        referer: 'https://my.example.com/proxy',
        anonymous_id: `anon-${i}`,
        session_id: '',
        utm_source: 'my',
        utm_medium: '',
        utm_campaign: '',
        utm_term: '',
        utm_content: '',
        channel: i === 0 ? 'www' : 'my',
        extra: '',
        created_at: now(),
      }))
      return ok({ list, total: list.length })
    },
  },
  // 公开：启用列表（my/www 展示）
  {
    url: '/v1/partner/list',
    method: 'get',
    response: () => ok(sorted(items.filter(it => it.enabled)).map(it => ({
      id: it.id,
      name: it.name,
      logo_url: it.logo_url,
      image_url: it.image_url,
      intro: it.intro,
      promo_url: it.promo_url,
    }))),
  },
  // 公开：点击上报
  {
    url: '/v1/partner/:id/click',
    method: 'post',
    response: ({ params }) => {
      const it = items.find(x => x.id === Number(params.id))
      if (it && it.enabled)
        it.click_count += 1
      return ok(null)
    },
  },
])
