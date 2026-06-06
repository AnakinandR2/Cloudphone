import type { AppSettings } from '@/types/settings'

/**
 * 应用默认配置。
 *
 * 运行时可在「设置面板」中调整（仅存本地，不改此文件）；若想把某次调整
 * 固化为项目默认，点击设置面板的「复制为默认配置」，把内容粘贴回这里即可。
 */
export const defaultSettings: AppSettings = {
  locale: 'zh-CN',
  menuMode: 'single',
  themeColor: 'teal',
  colorScheme: 'light',
  pageTransition: 'zoom',
  progressBar: true,
  radius: 0.75,
}
