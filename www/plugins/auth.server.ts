// 服务端启动时拉一次当前用户。文件名 .server 表示仅在 SSR 阶段执行；
// 拉到的 user 经 useState 自动序列化到 payload，客户端 hydrate 无闪烁。
export default defineNuxtPlugin(async () => {
  await fetchAuthUserOnServer()
})
