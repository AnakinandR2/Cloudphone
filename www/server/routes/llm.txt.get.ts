// GET /llm.txt —— 回源内容中台 web-files；?lang= 指定语言（缺省走中台回退）。
export default defineEventHandler((event) => {
  const q = getQuery(event)
  return proxyWebFile(event, '/llm.txt', q.lang ? langToApi(String(q.lang)) : undefined)
})
