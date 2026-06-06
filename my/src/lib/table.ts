import type { Updater } from '@tanstack/vue-table'
import type { Ref } from 'vue'

/** 把 TanStack 的 Updater（值或函数）应用到 Vue ref 上 */
export function valueUpdater<T>(updaterOrValue: Updater<T>, ref: Ref<T>) {
  ref.value
    = typeof updaterOrValue === 'function'
      ? (updaterOrValue as (old: T) => T)(ref.value)
      : updaterOrValue
}

// 为列定义的 meta 增加 label 字段（用于“列显示”下拉菜单中文名）
declare module '@tanstack/vue-table' {
  // TData/TValue 为模块增强所需的同名类型参数（须与上游签名一致），此处用不到
  // eslint-disable-next-line unused-imports/no-unused-vars
  interface ColumnMeta<TData, TValue> {
    /** “列显示”下拉里的名称（i18n key） */
    label?: string
    /** 表头额外 class */
    headClass?: string
    /** 单元格额外 class */
    cellClass?: string
  }
}
