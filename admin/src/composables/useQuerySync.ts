import { watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

/**
 * 把一个 reactive 的字符串筛选对象与 URL query 双向同步：
 * - 进入页面时用 URL 上已有的参数初始化筛选；
 * - 筛选变化时用 router.replace 写回 URL（等于默认值的项不写入，保持地址干净）；
 * 这样搜索条件可通过 URL 分享 / 刷新保留。
 *
 * @param state    reactive 的筛选对象（值均为字符串）
 * @param defaults 各字段的默认值（等于默认值时从 URL 移除）
 */
export function useQuerySync(
  state: Record<string, string>,
  defaults: Record<string, string>,
) {
  const route = useRoute()
  const router = useRouter()

  // 1) URL → state（初始化）
  for (const key of Object.keys(defaults)) {
    const v = route.query[key]
    if (typeof v === 'string') {
      state[key] = v
    }
  }

  // 2) state → URL（变化时写回）
  watch(
    () => Object.keys(defaults).map(k => state[k]).join(''),
    () => {
      const query: Record<string, string> = {}
      // 保留与筛选无关的其它 query 参数
      for (const [k, v] of Object.entries(route.query)) {
        if (!(k in defaults) && typeof v === 'string') {
          query[k] = v
        }
      }
      for (const key of Object.keys(defaults)) {
        const val = state[key]
        if (val && val !== defaults[key]) {
          query[key] = val
        }
      }
      router.replace({ query })
    },
  )
}
