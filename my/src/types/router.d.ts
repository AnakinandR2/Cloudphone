import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    title?: string
    icon?: string
    /** 访问该路由所需权限码（任一满足即可）；空表示不限 */
    auth?: string | string[]
    /** false 表示不在菜单中显示 */
    menu?: boolean
    /** 白名单：无需登录即可访问 */
    whiteList?: boolean
    badge?: string | number
    badgeVariant?: 'default' | 'secondary' | 'destructive'
    link?: string
  }
}
