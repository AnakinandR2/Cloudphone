// GET /_content/api-docs/:slug/spec —— 回源该文档的原始 OpenAPI（非信封，注入 Key）。
export default defineEventHandler((event) => {
  const slug = getRouterParam(event, 'slug')
  if (!slug) {
    throw createError({ statusCode: 400, statusMessage: '缺少文档 slug' })
  }
  return proxyRaw(event, `/api-docs/${encodeURIComponent(slug)}/spec`)
})
