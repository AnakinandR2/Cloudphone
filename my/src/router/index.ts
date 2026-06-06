import type { RouteRecordRaw } from 'vue-router'
import { createRouter, createWebHistory } from 'vue-router'

import { setupGuards } from './guards'
import { asyncRoutes, constantRoutes, flattenRoutes, standaloneRoutes } from './routes'

// 布局路由：所有业务页面作为其 children
const layoutRoute: RouteRecordRaw = {
  path: '/',
  component: () => import('@/layouts/DefaultLayout.vue'),
  // 不再硬编码 redirect 到 /dashboard：根路径 '/' 由守卫按权限解析到首个可访问页
  children: flattenRoutes(asyncRoutes),
}

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [layoutRoute, ...standaloneRoutes, ...constantRoutes],
})

setupGuards(router)

export default router
