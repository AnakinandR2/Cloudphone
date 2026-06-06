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

interface ExampleRec {
  id: number
  title: string
  content: string
  created_at: string
  updated_at: string
}

const items: ExampleRec[] = Array.from({ length: 38 }).map((_, i) => ({
  id: 100 - i,
  title: `示例项目 ${100 - i}`,
  content: `这是第 ${100 - i} 条示例数据的内容，用于演示列表分页、搜索与增删改查。`,
  created_at: `2026-0${(i % 9) + 1}-1${i % 9} 10:30:00`,
  updated_at: `2026-0${(i % 9) + 1}-1${i % 9} 10:30:00`,
}))
let seq = 1000

export default defineFakeRoute([
  {
    url: '/v1/example/list',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 10
      const title = (query.title as string) || ''
      let list = items
      if (title) list = list.filter(it => it.title.includes(title))
      const total = list.length
      const start = (page - 1) * size
      return ok({ list: list.slice(start, start + size), total })
    },
  },
  {
    url: '/v1/example/:id',
    method: 'get',
    response: ({ params }) => {
      const it = items.find(x => x.id === Number(params.id))
      return it ? ok(it) : fail('记录不存在')
    },
  },
  {
    url: '/v1/example/create',
    method: 'post',
    response: ({ body }) => {
      const rec: ExampleRec = {
        id: ++seq,
        title: body.title,
        content: body.content || '',
        created_at: now(),
        updated_at: now(),
      }
      items.unshift(rec)
      return ok(rec)
    },
  },
  {
    url: '/v1/example/update/:id',
    method: 'put',
    response: ({ params, body }) => {
      const it = items.find(x => x.id === Number(params.id))
      if (!it) return fail('记录不存在')
      it.title = body.title ?? it.title
      it.content = body.content ?? it.content
      it.updated_at = now()
      return ok(it)
    },
  },
  {
    url: '/v1/example/delete/:id',
    method: 'delete',
    response: ({ params }) => {
      const idx = items.findIndex(x => x.id === Number(params.id))
      if (idx === -1) return fail('记录不存在')
      items.splice(idx, 1)
      return ok(null)
    },
  },
])
