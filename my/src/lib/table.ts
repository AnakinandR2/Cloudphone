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
    /** 横向滚动时钉在右侧（如操作列） */
    pin?: 'right'
  }
}

/** 标准操作列 meta（配合 DataTable `pin-actions-column`） */
export const ACTIONS_COLUMN_META = {
  label: 'crud.actions',
  pin: 'right' as const,
  cellClass: 'whitespace-nowrap',
}

/** 右侧钉住列（样式见 index.css .pinned-col-right-*） */
export const PINNED_RIGHT_HEAD_CLASS = 'pinned-col-right-head w-[1%] !px-6'

export const PINNED_RIGHT_CELL_CLASS = 'pinned-col-right-cell w-[1%] !px-6'

interface ResolveColumnClassOptions {
  pinActions?: boolean
  compact?: boolean
}

export function isPinnedColumn(
  columnId: string,
  meta?: { pin?: 'right' },
  pinActions?: boolean,
) {
  return meta?.pin === 'right' || (pinActions && columnId === 'actions')
}

const COMPACT_SKIP = new Set(['select', 'expander', 'actions'])

export function resolveHeadClass(
  columnId: string,
  meta: { headClass?: string, pin?: 'right' } | undefined,
  options: ResolveColumnClassOptions = {},
) {
  const base = meta?.headClass ?? ''
  if (isPinnedColumn(columnId, meta, options.pinActions))
    return `${base} ${PINNED_RIGHT_HEAD_CLASS}`.trim()
  if (options.compact && !base.includes('w-') && !COMPACT_SKIP.has(columnId))
    return `${base} min-w-0`.trim()
  return base
}

export function resolveCellClass(
  columnId: string,
  meta: { cellClass?: string, pin?: 'right' } | undefined,
  options: ResolveColumnClassOptions = {},
) {
  const base = meta?.cellClass ?? ''
  if (isPinnedColumn(columnId, meta, options.pinActions))
    return `${base} ${PINNED_RIGHT_CELL_CLASS}`.trim()
  if (options.compact && !base.includes('w-') && !base.includes('max-w') && !COMPACT_SKIP.has(columnId))
    return `${base} min-w-0 max-w-[12rem]`.trim()
  return base
}
