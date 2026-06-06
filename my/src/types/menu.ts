/** 菜单项（一级图标栏下的子菜单，可多级嵌套） */
export interface MenuRecord {
  path?: string
  meta?: {
    title?: string
    icon?: string
    auth?: string | string[]
    /** false 表示不在菜单显示（仅作为路由存在） */
    menu?: boolean
    badge?: string | number
    badgeVariant?: 'default' | 'secondary' | 'destructive'
    link?: string
  }
  children?: MenuRecord[]
}

/** 主导航（一级图标栏的一个分组） */
export interface MenuMainRecord {
  meta?: {
    title?: string
    icon?: string
    auth?: string | string[]
  }
  children: MenuRecord[]
}
