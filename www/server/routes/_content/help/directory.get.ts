// GET /api/help/directory —— 帮助文档目录树代理。
// query: lang
import type { PubDirectoryNode } from '~/types/content'

export default defineEventHandler(async (event) => {
  const q = getQuery(event)
  return await contentFetch<PubDirectoryNode[]>('/directory', {
    space: 'help',
    lang: langToApi(q.lang ? String(q.lang) : undefined),
  })
})
