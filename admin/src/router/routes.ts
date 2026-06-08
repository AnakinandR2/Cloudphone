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
    auth?: string | string[]
    menu?: boolean
  }
  children?: AppRoute[]
}

/** 一级图标栏的分组（双层菜单的第一层） */
export interface AppMainRoute {
  meta: { title: string, icon: string, auth?: string | string[] }
  children: AppRoute[]
}

/**
 * 动态路由（登录后按权限过滤）。
 * 顶层每一项 = 左侧图标栏的一个分组；其 children 渲染为二级（可多级）子菜单。
 */
export const asyncRoutes: AppMainRoute[] = [
  {
    meta: { title: 'menu.workbench', icon: 'LayoutDashboard' },
    children: [
      {
        path: '/dashboard',
        name: 'dashboard',
        component: () => import('@/views/dashboard/DashboardView.vue'),
        meta: { title: 'menu.dashboard', icon: 'Gauge', auth: 'dashboard:view' },
      },
    ],
  },
  {
    meta: { title: 'menu.system', icon: 'Settings' },
    children: [
      {
        path: '/system/staff',
        name: 'staff',
        component: () => import('@/views/system/staff/StaffView.vue'),
        meta: { title: 'menu.staff', icon: 'Users', auth: 'staff:view' },
      },
      {
        path: '/system/roles',
        name: 'roles',
        component: () => import('@/views/system/roles/RolesView.vue'),
        meta: { title: 'menu.roles', icon: 'ShieldCheck', auth: 'role:view' },
      },
      {
        path: '/system/access-log',
        name: 'accessLog',
        component: () => import('@/views/system/access-log/AccessLogView.vue'),
        meta: { title: 'menu.accessLog', icon: 'ScrollText', auth: 'access_log:view' },
      },
    ],
  },
  {
    meta: { title: 'menu.ops', icon: 'Activity' },
    children: [
      {
        path: '/ops/users',
        name: 'users',
        component: () => import('@/views/ops/users/UsersView.vue'),
        meta: { title: 'menu.users', icon: 'UserRound', auth: 'user:view' },
      },
      {
        path: '/ops/proxies',
        name: 'opsProxies',
        component: () => import('@/views/ops/ProxyPoolView.vue'),
        meta: { title: 'menu.opsProxies', icon: 'Network', auth: 'proxy:view' },
      },
      {
        path: '/ops/instances',
        name: 'opsInstances',
        component: () => import('@/views/ops/InstanceView.vue'),
        meta: { title: 'menu.opsInstances', icon: 'Smartphone', auth: 'phone:view' },
      },
      {
        path: '/ops/apps',
        name: 'opsApps',
        component: () => import('@/views/ops/AppsView.vue'),
        meta: { title: 'menu.opsApps', icon: 'AppWindow', auth: 'app:view' },
      },
    ],
  },
  {
    meta: { title: 'menu.cloudphone', icon: 'Smartphone' },
    children: [
      {
        path: '/cloudphone/phone-specs',
        name: 'cpPhoneSpecs',
        component: () => import('@/views/cloudphone/SpecsView.vue'),
        meta: { title: 'menu.cpPhoneSpecs', icon: 'Cpu', auth: 'cloudphone:view' },
      },
      {
        path: '/cloudphone/vm-specs',
        name: 'cpVmSpecs',
        component: () => import('@/views/cloudphone/SpecsView.vue'),
        meta: { title: 'menu.cpVmSpecs', icon: 'Server', auth: 'cloudphone:view' },
      },
      {
        path: '/cloudphone/vms',
        name: 'cpVms',
        component: () => import('@/views/cloudphone/VmsView.vue'),
        meta: { title: 'menu.cpVms', icon: 'Server', auth: 'cloudphone:view' },
      },
      {
        path: '/cloudphone/images',
        name: 'cpImages',
        component: () => import('@/views/cloudphone/ImagesView.vue'),
        meta: { title: 'menu.cpImages', icon: 'Disc', auth: 'cloudphone:view' },
      },
      {
        path: '/cloudphone/apps',
        name: 'cpApps',
        component: () => import('@/views/cloudphone/AppsView.vue'),
        meta: { title: 'menu.cpApps', icon: 'AppWindow', auth: 'app:view' },
      },
      {
        // 素材管理（占位，功能开发中）
        path: '/cloudphone/materials',
        name: 'cpMaterials',
        component: () => import('@/views/demo/nested/BlankView.vue'),
        meta: { title: 'menu.cpMaterials', icon: 'Images' },
      },
    ],
  },
  {
    // 费用运营
    meta: { title: 'menu.billing', icon: 'CreditCard' },
    children: [
      {
        path: '/billing/pricing',
        name: 'billingPricing',
        component: () => import('@/views/billing/PricingView.vue'),
        meta: { title: 'menu.billingPricing', icon: 'Tags', auth: 'billing:view' },
      },
      {
        path: '/billing/discounts',
        name: 'billingDiscounts',
        component: () => import('@/views/billing/DiscountsView.vue'),
        meta: { title: 'menu.billingDiscounts', icon: 'Percent', auth: 'billing:view' },
      },
      {
        path: '/billing/orders',
        name: 'billingOrders',
        component: () => import('@/views/billing/OrdersView.vue'),
        meta: { title: 'menu.billingOrders', icon: 'ReceiptText', auth: 'billing:view' },
      },
      {
        path: '/billing/accounts',
        name: 'billingAccounts',
        component: () => import('@/views/billing/AccountsView.vue'),
        meta: { title: 'menu.billingAccounts', icon: 'Wallet', auth: 'billing:view' },
      },
      {
        path: '/billing/trials',
        name: 'billingTrials',
        component: () => import('@/views/billing/TrialsView.vue'),
        meta: { title: 'menu.billingTrials', icon: 'Gift', auth: 'billing:view' },
      },
    ],
  },
  {
    // 自动化（占位分组，子项均为开发中占位页）
    meta: { title: 'menu.automation', icon: 'Bot' },
    children: [
      {
        path: '/automation/script-templates',
        name: 'autoScriptTemplates',
        component: () => import('@/views/demo/nested/BlankView.vue'),
        meta: { title: 'menu.autoScriptTemplates', icon: 'FileCode' },
      },
      {
        path: '/automation/tasks',
        name: 'autoTasks',
        component: () => import('@/views/demo/nested/BlankView.vue'),
        meta: { title: 'menu.autoTasks', icon: 'ListChecks' },
      },
      {
        path: '/automation/api-mcp',
        name: 'autoApiMcp',
        component: () => import('@/views/demo/nested/BlankView.vue'),
        meta: { title: 'menu.autoApiMcp', icon: 'Plug' },
      },
    ],
  },
  {
    meta: { title: 'menu.demo', icon: 'Boxes' },
    children: [
      {
        path: '/demo/example',
        name: 'example',
        component: () => import('@/views/demo/example/ExampleView.vue'),
        meta: { title: 'menu.example', icon: 'FileText', auth: 'example:view' },
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
      if (node.children) {
        walk(node.children)
      }
    })
  }
  mains.forEach(m => walk(m.children))
  return result
}

/** 常量路由（无需登录或与权限无关） */
export const constantRoutes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: { title: 'login.title', whiteList: true },
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('@/views/error/NotFoundView.vue'),
    meta: { title: 'notFound.desc', whiteList: true },
  },
]
