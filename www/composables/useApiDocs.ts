// API 文档组合式：取内容中台已发布的 API 文档列表（数据经本站 /_content 代理）。
import type { ApiDocSummary } from '~/types/content'

export function useApiDocs() {
  const { data, pending, error, refresh } = useFetch<ApiDocSummary[]>('/_content/api-docs', {
    default: () => [],
  })
  const docs = computed(() => data.value ?? [])
  return { docs, pending, error, refresh }
}
