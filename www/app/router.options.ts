import type { RouterConfig } from '@nuxt/schema'

// 自定义锚点滚动：用 getElementById（按原始 id 查找，不经 CSS 选择器解析）解析 hash，
// 避免含「/」等特殊字符的 hash（如 Scalar 的 #description/introduction）被 querySelector
// 当成非法选择器而告警；找不到对应元素则不滚动，交给组件自身处理（Scalar 自管滚动）。
export default <RouterConfig>{
  scrollBehavior(to, _from, savedPosition) {
    if (savedPosition) return savedPosition
    if (to.hash) {
      const id = decodeURIComponent(to.hash.slice(1))
      const el = typeof document !== 'undefined' ? document.getElementById(id) : null
      return el ? { el, top: 80, behavior: 'smooth' } : false
    }
    return { left: 0, top: 0 }
  },
}
