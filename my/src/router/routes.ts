import type { RouteRecordRaw } from 'vue-router'

/** 路由树节点：既是路由定义，也是菜单来源 */
export interface AppRoute {
  path?: string
  name?: string
  component?: () => Promise<unknown>
  redirect?: string
  meta?: {
    title?: string
    icon?: string
    /** false 表示不在菜单显示（仅作路由） */
    menu?: boolean
    /** 隐藏/详情页面包屑链（title 为 i18n key，有 path 则可点击） */
    breadcrumb?: { title: string, path?: string }[]
  }
  children?: AppRoute[]
}

/** 一级图标栏的分组（双层菜单的第一层） */
export interface AppMainRoute {
  meta: { title: string, icon: string }
  children: AppRoute[]
}

/**
 * 业务路由。顶层每一项 = 左侧图标栏的一个分组；
 * 其 children 渲染为二级（可多级）子菜单。前台用户无权限体系，菜单全部可见。
 */
export const asyncRoutes: AppMainRoute[] = [
  {
    meta: { title: 'menu.cloud', icon: 'Smartphone' },
    children: [
      {
        path: '/phone',
        name: 'phone',
        component: () => import('@/views/phone/PhoneView.vue'),
        meta: { title: 'menu.phone', icon: 'Smartphone' },
      },
      {
        path: '/proxy',
        name: 'proxy',
        component: () => import('@/views/proxy/ProxyView.vue'),
        meta: { title: 'menu.proxy', icon: 'Network' },
      },
      {
        // 应用/素材（我的应用 + 应用市场 + 我的素材，共享容量条）
        path: '/assets',
        name: 'assets',
        component: () => import('@/views/assets/AssetsView.vue'),
        meta: { title: 'menu.assets', icon: 'FolderArchive' },
      },
      {
        // 旧入口重定向到合并后的「应用/素材」页（不在菜单显示）
        path: '/phone/apps',
        name: 'phoneApps',
        redirect: '/assets',
        meta: { menu: false },
      },
      {
        path: '/library',
        name: 'library',
        redirect: '/assets',
        meta: { menu: false },
      },
    ],
  },
  {
    // 费用（购买 / 用量 / 订单 / 费用日志）
    meta: { title: 'menu.billing', icon: 'Wallet' },
    children: [
      {
        path: '/billing',
        name: 'billing',
        component: () => import('@/views/billing/BillingPurchaseView.vue'),
        meta: { title: 'menu.billingPurchase', icon: 'ShoppingCart' },
      },
      {
        // 费用日志（聚合到开机会话的运行计费记录，设计 §5.5 / 契约 §1.6）
        path: '/billing/runtime-log',
        name: 'billingRuntimeLog',
        component: () => import('@/views/billing/BillingRuntimeLogView.vue'),
        meta: { title: 'menu.billingLedger', icon: 'ScrollText' },
      },
      {
        path: '/billing/trials',
        name: 'billingTrials',
        component: () => import('@/views/billing/BillingTrialsView.vue'),
        meta: { title: 'menu.billingTrials', icon: 'Gift' },
      },
    ],
  },
  {
    // 自动化（占位分组，子项均为开发中占位页）
    meta: { title: 'menu.automation', icon: 'Bot' },
    children: [
      {
        path: '/automation/scripts',
        name: 'automationScripts',
        component: () => import('@/views/automation/ScriptView.vue'),
        meta: { title: 'menu.scriptManage', icon: 'FileCode' },
      },
      {
        path: '/automation/schedules',
        name: 'automationSchedules',
        component: () => import('@/views/automation/TaskScheduleView.vue'),
        meta: { title: 'menu.taskSchedule', icon: 'CalendarClock' },
      },
      {
        path: '/automation/task-logs',
        name: 'automationTaskLogs',
        component: () => import('@/views/automation/TaskLogView.vue'),
        meta: { title: 'menu.taskLogs', icon: 'ListChecks' },
      },
      {
        // API & MCP（原型，暂无具体功能）
        path: '/automation/api-mcp',
        name: 'automationApiMcp',
        component: () => import('@/views/automation/ApiMcpView.vue'),
        meta: { title: 'menu.apiMcp', icon: 'Plug' },
      },
    ],
  },
  {
    meta: { title: 'menu.demo', icon: 'Boxes' },
    children: [
      {
        path: '/demo/note',
        name: 'note',
        component: () => import('@/views/demo/note/NoteView.vue'),
        meta: { title: 'menu.note', icon: 'FileText' },
      },
      {
        // 多级菜单示例（菜单中间节点，无 component）
        name: 'nested',
        meta: { title: 'menu.nested', icon: 'Layers' },
        children: [
          {
            path: '/demo/nested/menu1',
            name: 'nestedMenu1',
            component: () => import('@/views/demo/nested/BlankView.vue'),
            meta: { title: 'menu.nestedMenu1' },
          },
          {
            name: 'nestedMenu2',
            meta: { title: 'menu.nestedMenu2' },
            children: [
              {
                path: '/demo/nested/menu2/page1',
                name: 'nestedMenu2Page1',
                component: () => import('@/views/demo/nested/BlankView.vue'),
                meta: { title: 'menu.nestedMenu2Page1' },
              },
              {
                path: '/demo/nested/menu2/page2',
                name: 'nestedMenu2Page2',
                component: () => import('@/views/demo/nested/BlankView.vue'),
                meta: { title: 'menu.nestedMenu2Page2' },
              },
            ],
          },
        ],
      },
    ],
  },
  {
    meta: { title: 'menu.components', icon: 'Blocks' },
    children: [
      {
        path: '/components/basic',
        name: 'componentsBasic',
        component: () => import('@/views/components/ComponentsBasicView.vue'),
        meta: { title: 'menu.componentsBasic', icon: 'Component' },
      },
      {
        path: '/components/feedback',
        name: 'componentsFeedback',
        component: () => import('@/views/components/ComponentsFeedbackView.vue'),
        meta: { title: 'menu.componentsFeedback', icon: 'Sparkles' },
      },
      {
        path: '/components/form',
        name: 'componentsForm',
        component: () => import('@/views/components/ComponentsFormView.vue'),
        meta: { title: 'menu.componentsForm', icon: 'FileText' },
      },
      {
        path: '/components/data',
        name: 'componentsData',
        component: () => import('@/views/components/ComponentsDataView.vue'),
        meta: { title: 'menu.componentsData', icon: 'Boxes' },
      },
    ],
  },
]

/** 将路由树中所有带 component 的叶子拍平，作为布局路由的 children 注册 */
export function flattenRoutes(mains: AppMainRoute[]): RouteRecordRaw[] {
  const result: RouteRecordRaw[] = []
  const walk = (nodes: AppRoute[]) => {
    nodes.forEach((node) => {
      if (node.component && node.path) {
        result.push({
          path: node.path,
          name: node.name,
          component: node.component,
          meta: node.meta,
        } as RouteRecordRaw)
      }
      else if (node.redirect && node.path) {
        // 仅重定向的路由（无 component），用于旧入口跳转到新页
        result.push({
          path: node.path,
          name: node.name,
          redirect: node.redirect,
          meta: node.meta,
        } as RouteRecordRaw)
      }
      if (node.children) {
        walk(node.children)
      }
    })
  }
  mains.forEach(m => walk(m.children))
  return result
}

/** 独立窗口路由（需登录，但不套后台布局；用 window.open 独立弹窗打开）。 */
export const standaloneRoutes: RouteRecordRaw[] = [
  {
    path: '/phone/remote/:id',
    name: 'phoneRemote',
    component: () => import('@/views/phone/RemoteControlView.vue'),
    meta: { title: 'phone.rc.title' },
  },
  {
    path: '/phone/group',
    name: 'phoneGroup',
    component: () => import('@/views/phone/GroupControlView.vue'),
    meta: { title: 'phone.group.title' },
  },
]

/** 常量路由（无需登录） */
export const constantRoutes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: { title: 'login.title', whiteList: true },
  },
  {
    path: '/register',
    name: 'register',
    component: () => import('@/views/auth/RegisterView.vue'),
    meta: { title: 'register.title', whiteList: true },
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('@/views/error/NotFoundView.vue'),
    meta: { title: 'notFound.desc', whiteList: true },
  },
]
