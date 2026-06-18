// GET /robots.txt —— 回源内容中台 web-files（全局文件）。
export default defineEventHandler((event) => proxyWebFile(event, '/robots.txt'))
