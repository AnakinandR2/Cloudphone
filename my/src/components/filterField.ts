/** Shared control styles for FilterField compound inputs. */
export const filterFieldWidthClass = 'min-w-0 max-w-[400px] flex-[1_1_400px]'

export const filterFieldShellClass
  = [
    'border-input bg-background hover:border-ring/50',
    // 搜索输入聚焦 / 下拉展开时显示外圈阴影；不用 focus-within，避免 Select 关面板后焦点残留导致阴影黏住
    'has-[input:focus-visible]:border-ring has-[input:focus-visible]:ring-ring/25 has-[input:focus-visible]:ring-[3px]',
    'has-[[data-state=open]]:border-ring has-[[data-state=open]]:ring-ring/25 has-[[data-state=open]]:ring-[3px]',
    'flex h-10 items-center rounded-md border shadow-xs transition-[color,box-shadow,border-color] duration-150',
  ].join(' ')

export const filterInputClass
  = 'placeholder:text-muted-foreground/50 h-full w-full min-w-0 bg-transparent px-3 text-sm outline-none'

export const filterMutedTextClass = 'text-muted-foreground/50'

export const filterSelectTriggerClass
  = 'h-full w-full rounded-none border-0 bg-transparent px-3 py-0 shadow-none focus:ring-0 focus-visible:ring-0 data-[size=default]:h-full data-[state=open]:bg-muted/30 data-[placeholder]:text-muted-foreground/50'

export const filterDateButtonClass
  = 'hover:bg-muted/30 flex h-full w-full min-w-0 cursor-pointer items-center justify-between gap-3 bg-transparent px-3 text-left text-sm outline-none transition-colors'

/** 筛选下拉/日期面板：与筛选框右对齐，间距 8px */
export const filterPopupAlign = 'end' as const
export const filterPopupSideOffset = 8

/** 去掉 SelectContent popper 默认位移，避免与 sideOffset 叠加 */
export const filterSelectContentClass
  = 'data-[side=bottom]:translate-y-0 data-[side=top]:translate-y-0 data-[side=left]:translate-x-0 data-[side=right]:translate-x-0'
