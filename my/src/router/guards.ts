import type { Router } from 'vue-router'

import { t } from '@/locales'
import { useMenuStore } from '@/stores/menu'
import { useSettingsStore } from '@/stores/settings'
import { useUserStore } from '@/stores/user'
import { doneProgress, startProgress } from '@/utils/progress'

export function setupGuards(router: Router) {
  router.beforeEach(async (to) => {
    if (useSettingsStore().settings.progressBar) {
      startProgress()
    }
    const userStore = useUserStore()

    // 未登录：仅放行白名单，其余跳登录
    if (!userStore.isLogin) {
      if (to.meta.whiteList) {
        return true
      }
      return { name: 'login', query: { redirect: to.fullPath } }
    }

    // 已登录但还没拉过资料 → 拉一次用户信息
    if (!userStore.loaded) {
      try {
        await userStore.getInfo()
      }
      catch {
        userStore.clear()
        return { name: 'login', query: { redirect: to.fullPath } }
      }
    }

    // 已登录访问登录/注册页或根路径 → 进首页
    if (to.name === 'login' || to.name === 'register' || to.path === '/') {
      return { path: useMenuStore().firstPath || '/dashboard' }
    }

    // 同步一级菜单高亮
    useMenuStore().syncActiveByPath(to.path)
    return true
  })

  router.afterEach((to) => {
    doneProgress()
    const base = import.meta.env.VITE_APP_TITLE || '管理后台'
    const title = to.meta.title ? t(to.meta.title) : ''
    document.title = title ? `${title} · ${base}` : base
  })

  router.onError(() => doneProgress())
}
