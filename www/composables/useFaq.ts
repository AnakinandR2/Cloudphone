// FAQ 组合式：按分类分组的问答，数据来自内容中台 faq 空间。
import type { FaqGroup } from '~/types/content'

export function useFaq() {
  const { locale } = useI18n()
  const { data, pending, error, refresh } = useFetch<FaqGroup[]>('/_content/faq', {
    query: computed(() => ({ lang: locale.value })),
    default: () => [],
  })
  const groups = computed(() => data.value ?? [])
  // 扁平化（首页板块取前 N 条用）
  const flat = computed(() => groups.value.flatMap((g) => g.items))
  return { groups, flat, pending, error, refresh }
}
