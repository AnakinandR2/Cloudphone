/** 菜单模式：double=双层(图标栏+子菜单)，single=单层(只有子菜单) */
export type MenuMode = 'double' | 'single'

/** 主题色预设（对应 index.css 中 [data-theme] 预设） */
export type ThemeColor
  = | 'indigo'
    | 'violet'
    | 'blue'
    | 'cyan'
    | 'teal'
    | 'emerald'
    | 'rose'
    | 'orange'
    | 'amber'
    | 'pink'
    | 'neutral'

/** 明暗模式 */
export type ColorScheme = 'light' | 'dark' | 'system'

/** 页面进入动效（对应 index.css 中的 transition name） */
export type PageTransition = 'none' | 'fade' | 'slide' | 'slide-up' | 'zoom' | 'blur'

/** 界面语言 */
export type Locale = 'zh-CN' | 'en'

export interface AppSettings {
  /** 界面语言 */
  locale: Locale
  /** 菜单模式 */
  menuMode: MenuMode
  /** 主题色 */
  themeColor: ThemeColor
  /** 明暗模式 */
  colorScheme: ColorScheme
  /** 页面进入动效 */
  pageTransition: PageTransition
  /** 路由切换时显示顶部加载进度条（模拟载入） */
  progressBar: boolean
  /** 圆角大小（rem） */
  radius: number
}
