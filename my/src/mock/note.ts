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

interface NoteRec {
  id: number
  user_id: number
  title: string
  content: string
  created_at: string
  updated_at: string
}

// 内存笔记表（演示用，归属 user 1）
const notes: NoteRec[] = Array.from({ length: 8 }).map((_, i) => ({
  id: 8 - i,
  user_id: 1,
  title: `我的笔记 ${8 - i}`,
  content: `这是第 ${8 - i} 条笔记的内容，演示前台用户的笔记增删改查。`,
  created_at: `2026-0${(i % 9) + 1}-1${i % 9} 10:30:00`,
  updated_at: `2026-0${(i % 9) + 1}-1${i % 9} 10:30:00`,
}))
let seq = 1000

export default defineFakeRoute([
  {
    url: '/v1/note/list',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 10
      const title = (query.title as string) || ''
      let list = notes
      if (title) list = list.filter(n => n.title.includes(title))
      const total = list.length
      const start = (page - 1) * size
      return ok({ list: list.slice(start, start + size), total })
    },
  },
  {
    url: '/v1/note/:id',
    method: 'get',
    response: ({ params }) => {
      const n = notes.find(x => x.id === Number(params.id))
      return n ? ok(n) : fail('笔记不存在')
    },
  },
  {
    url: '/v1/note/create',
    method: 'post',
    response: ({ body }) => {
      if (!body?.title) return fail('请填写标题')
      const rec: NoteRec = {
        id: ++seq,
        user_id: 1,
        title: body.title,
        content: body.content || '',
        created_at: now(),
        updated_at: now(),
      }
      notes.unshift(rec)
      return ok(rec)
    },
  },
  {
    url: '/v1/note/update/:id',
    method: 'put',
    response: ({ params, body }) => {
      const n = notes.find(x => x.id === Number(params.id))
      if (!n) return fail('笔记不存在')
      if (body?.title !== undefined) n.title = body.title
      if (body?.content !== undefined) n.content = body.content
      n.updated_at = now()
      return ok(n)
    },
  },
  {
    url: '/v1/note/delete/:id',
    method: 'delete',
    response: ({ params }) => {
      const idx = notes.findIndex(x => x.id === Number(params.id))
      if (idx === -1) return fail('笔记不存在')
      notes.splice(idx, 1)
      return ok(null)
    },
  },
])
