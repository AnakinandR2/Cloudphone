import type { Router } from 'vue-router'
import { toast } from 'vue-sonner'

import { useAuth } from '@/composables/useAuth'
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

    // 已登录但还没拉过权限 → 拉用户信息与权限码
    if (userStore.permissions.length === 0) {
      try {
        await userStore.getInfo()
      }
      catch {
        userStore.logout()
        return { name: 'login', query: { redirect: to.fullPath } }
      }
    }

    const menuStore = useMenuStore()
    const { auth } = useAuth()

    // 已登录访问登录页 / 根路径 → 解析到首个可访问页
    if (to.name === 'login' || to.path === '/') {
      const home = menuStore.firstPath
      if (home) {
        return { path: home }
      }
      // 当前账号没有任何可访问页面：登出并提示，避免 404 死循环
      userStore.logout()
      toast.error(t('login.noAccess'))
      return { name: 'login' }
    }

    // 路由级权限校验
    if (to.meta.auth && !auth(to.meta.auth)) {
      return { name: 'not-found' }
    }

    // 同步一级菜单高亮
    menuStore.syncActiveByPath(to.path)
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
