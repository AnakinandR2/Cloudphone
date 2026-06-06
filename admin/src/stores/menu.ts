import type { AppMainRoute, AppRoute } from '@/router/routes'
import { defineStore } from 'pinia'

import { computed, ref } from 'vue'
import { useAuth } from '@/composables/useAuth'
import { asyncRoutes } from '@/router/routes'

/** 递归按权限过滤子菜单，并剔除 menu===false 的项 */
function filterRoutes(nodes: AppRoute[]): AppRoute[] {
  const { auth } = useAuth()
  const res: AppRoute[] = []
  nodes.forEach((node) => {
    if (node.meta?.menu === false) {
      return
    }
    if (!auth(node.meta?.auth ?? '')) {
      return
    }
    if (node.children && node.children.length > 0) {
      const children = filterRoutes(node.children)
      if (children.length > 0) {
        res.push({ ...node, children })
      }
    }
    else if (node.path) {
      res.push({ ...node })
    }
  })
  return res
}

export const useMenuStore = defineStore('menu', () => {
  const active = ref(0)

  /** 一级图标栏分组（按权限过滤后非空的分组） */
  const mainMenus = computed<AppMainRoute[]>(() => {
    const { auth } = useAuth()
    return asyncRoutes
      .filter(group => auth(group.meta.auth ?? ''))
      .map(group => ({ ...group, children: filterRoutes(group.children) }))
      .filter(group => group.children.length > 0)
  })

  /** 当前激活分组下的二级（含多级）子菜单 */
  const sidebarMenus = computed<AppRoute[]>(
    () => mainMenus.value[active.value]?.children ?? [],
  )

  function setActive(index: number) {
    active.value = index
  }

  /** 收集一个节点子树下的所有可达路径 */
  function collectPaths(node: AppRoute): string[] {
    const paths: string[] = []
    if (node.path) {
      paths.push(node.path)
    }
    node.children?.forEach(c => paths.push(...collectPaths(c)))
    return paths
  }

  /**
   * 面包屑：从一级分组 → 各级菜单 → 当前页 的标题链（title 为 i18n key）。
   * 仅「本身有对应页面（path）」的级别可点击；分组/纯容器中间节点不带 path。
   */
  function getBreadcrumb(path: string): { title: string, path?: string }[] {
    const findTrail = (
      nodes: AppRoute[],
      acc: AppRoute[],
    ): AppRoute[] | null => {
      for (const node of nodes) {
        const next = [...acc, node]
        if (node.path === path) {
          return next
        }
        if (node.children) {
          const r = findTrail(node.children, next)
          if (r) return r
        }
      }
      return null
    }
    for (const group of mainMenus.value) {
      const trail = findTrail(group.children, [])
      if (trail) {
        return [
          { title: group.meta.title },
          ...trail.map(n => ({ title: n.meta?.title ?? '', path: n.path })),
        ]
      }
    }
    return []
  }

  /** 根据当前路由路径定位它所属的一级分组（刷新/直达时同步高亮） */
  function syncActiveByPath(path: string) {
    const groups = mainMenus.value
    for (let i = 0; i < groups.length; i++) {
      const paths = groups[i].children.flatMap(collectPaths)
      if (paths.some(p => path === p || path.startsWith(`${p}/`))) {
        active.value = i
        return
      }
    }
  }

  /** 登录后第一个可访问的叶子路径（用于登录跳转兜底） */
  const firstPath = computed(() => {
    const find = (nodes: AppRoute[]): string | undefined => {
      for (const n of nodes) {
        if (n.path && (!n.children || n.children.length === 0)) {
          return n.path
        }
        if (n.children) {
          const p = find(n.children)
          if (p) return p
        }
      }
      return undefined
    }
    for (const g of mainMenus.value) {
      const p = find(g.children)
      if (p) return p
    }
    // 没有任何可访问页面时返回空串，交由调用方处理（避免死循环到无权限页）
    return ''
  })

  return {
    active,
    mainMenus,
    sidebarMenus,
    firstPath,
    setActive,
    syncActiveByPath,
    getBreadcrumb,
  }
})
