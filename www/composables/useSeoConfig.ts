// 全站 SEO 配置注入：按当前路由 path 调中台 seo/resolve，把返回的 metas 注入 <head>。
// 在 app.vue 全站调用，SSR 阶段解析，保证爬虫首屏拿到正确 head。
// 非破坏式覆盖：只注入配置显式定义的 key，用高优先级压过页面同名默认 tag；未命中则保留页面自身值。
import type { SeoMeta, SeoResolveResult } from '~/types/content'

/** 把 seo/resolve 的 metas 转成 useHead 输入（纯函数，便于单测）。 */
export function seoMetasToHead(metas: SeoMeta[]): {
  title?: string
  meta: Record<string, string>[]
} {
  const head: { title?: string; meta: Record<string, string>[] } = { meta: [] }
  for (const m of metas || []) {
    if (!m || !m.key) continue
    if (m.key === '__title__') head.title = m.value
    else head.meta.push({ [m.attr || 'name']: m.key, content: m.value })
  }
  return head
}

export function useSeoConfig() {
  const route = useRoute()
  const { data } = useFetch<SeoResolveResult>('/_content/seo', {
    query: computed(() => ({ path: route.path })),
    default: () => ({ matched: false, metas: [] }),
  })
  useHead(
    computed(() => {
      const { title, meta } = seoMetasToHead(data.value?.metas ?? [])
      return { ...(title ? { title } : {}), meta }
    }),
    // 高优先级，压过页面自身设置的同名 title/meta（中台统一管控）。
    { tagPriority: 1 },
  )
}
