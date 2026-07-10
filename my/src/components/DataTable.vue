<script setup lang="ts" generic="TData">
import type { ColumnDef, ExpandedState, VisibilityState } from '@tanstack/vue-table'
import {

  FlexRender,
  getCoreRowModel,
  getExpandedRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  useVueTable,
} from '@tanstack/vue-table'
import { ChevronLeft, ChevronRight, SlidersHorizontal } from 'lucide-vue-next'
import { computed, nextTick, onMounted, onUnmounted, ref, useSlots, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import FilterField from '@/components/FilterField.vue'
import FilterSearchInput from '@/components/FilterSearchInput.vue'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableEmpty,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { resolveCellClass, resolveHeadClass, valueUpdater } from '@/lib/table'
import { cn } from '@/lib/utils'

const props = withDefaults(
  defineProps<{
    columns: ColumnDef<TData, unknown>[]
    data: TData[]
    pageSize?: number
    pageSizes?: number[]
    search?: boolean
    searchLabel?: string
    searchPlaceholder?: string
    expandable?: boolean
    loading?: boolean
    getRowId?: (row: TData) => string
    /** 填满父级：工具栏/分页固定，仅表格区滚动 */
    fixedLayout?: boolean
    /** 横向滚动时钉住操作列（id=actions 或 meta.pin=right） */
    pinActionsColumn?: boolean
  }>(),
  { pageSize: 20, search: true, searchPlaceholder: '', expandable: false, loading: false, fixedLayout: false, pinActionsColumn: false },
)

// 每页条数可选项：默认 20，可选 50 / 100 / 200
const pageSizeOptions = computed(() => props.pageSizes ?? [20, 50, 100, 200])

const slots = useSlots()
const { t } = useI18n()

// 全局搜索值可由父级 v-model:search-value 接管（用于与 URL query 同步）
const globalFilter = defineModel<string>('searchValue', { default: '' })
const columnVisibility = ref<VisibilityState>({})
const expanded = ref<ExpandedState>({})
const pagination = ref({ pageIndex: 0, pageSize: props.pageSize })

const table = useVueTable({
  get data() {
    return props.data
  },
  get columns() {
    return props.columns
  },
  ...(props.getRowId ? { getRowId: props.getRowId } : {}),
  getCoreRowModel: getCoreRowModel(),
  getFilteredRowModel: getFilteredRowModel(),
  getPaginationRowModel: getPaginationRowModel(),
  getExpandedRowModel: getExpandedRowModel(),
  onColumnVisibilityChange: u => valueUpdater(u, columnVisibility),
  onExpandedChange: u => valueUpdater(u, expanded),
  onGlobalFilterChange: u => valueUpdater(u, globalFilter),
  onPaginationChange: u => valueUpdater(u, pagination),
  enableSorting: false,
  autoResetPageIndex: false,
  initialState: { pagination: { pageSize: props.pageSize } },
  state: {
    get columnVisibility() {
      return columnVisibility.value
    },
    get expanded() {
      return expanded.value
    },
    get globalFilter() {
      return globalFilter.value
    },
    get pagination() {
      return pagination.value
    },
  },
})

// 搜索词变化时回到第 1 页；数据静默刷新（轮询）不重置页码。
watch(globalFilter, () => {
  table.setPageIndex(0)
})

watch(() => props.data.length, () => {
  const maxPage = Math.max(0, table.getPageCount() - 1)
  if (pagination.value.pageIndex > maxPage)
    table.setPageIndex(maxPage)
})

const leafCount = computed(() => table.getVisibleLeafColumns().length)
const pageIndex = computed(() => table.getState().pagination.pageIndex)
const pageCount = computed(() => table.getPageCount())

function colLabel(id: string) {
  const meta = props.columns.find(c => (c as { id?: string }).id === id)?.meta as
    | { label?: string }
    | undefined
  return meta?.label ? t(meta.label) : id
}

function hasSlot(name: string) {
  return !!slots[name]
}

const tableClass = computed(() => cn(
  (props.fixedLayout || props.pinActionsColumn) && '[&_[data-slot=table-container]]:!overflow-visible',
  (props.fixedLayout || props.pinActionsColumn) && '[&_[data-slot=table]]:table-fixed',
  props.pinActionsColumn && '[&_[data-slot=table]]:w-max [&_[data-slot=table]]:min-w-full',
))

const columnClassOptions = computed(() => ({
  // 开启即钉住；不可滚动时 sticky 与自然布局一致，无副作用
  pinActions: props.pinActionsColumn,
  compact: props.fixedLayout,
}))

function headClass(columnId: string, meta: unknown) {
  return resolveHeadClass(columnId, meta as Parameters<typeof resolveHeadClass>[1], columnClassOptions.value)
}

function cellClass(columnId: string, meta: unknown) {
  return resolveCellClass(columnId, meta as Parameters<typeof resolveCellClass>[1], columnClassOptions.value)
}

const scrollRef = ref<HTMLElement | null>(null)
const tableBoxRef = ref<HTMLElement | null>(null)
const tableScrollableX = ref(false)
const shadowLeft = ref(0)
const tableHeight = ref(0)
const shadowReady = ref(false)
let pinMetricsObserver: ResizeObserver | null = null

function updatePinMetrics() {
  nextTick(() => {
    const root = scrollRef.value
    const box = tableBoxRef.value
    if (!root || !box || !props.pinActionsColumn) {
      tableScrollableX.value = false
      shadowReady.value = false
      return
    }

    // 用 tableBox 自然宽度判断是否需要横向滚动（不依赖钉住类名）
    const scrollable = box.offsetWidth > root.clientWidth + 1
    tableScrollableX.value = scrollable

    if (!scrollable) {
      shadowReady.value = false
      return
    }

    const head = root.querySelector('.pinned-col-right-head') as HTMLElement | null
    if (!head) {
      shadowReady.value = false
      // 钉住类名刚挂上，下一帧再量
      requestAnimationFrame(updatePinMetrics)
      return
    }
    const headRect = head.getBoundingClientRect()
    const boxRect = box.getBoundingClientRect()
    shadowLeft.value = headRect.left - boxRect.left
    tableHeight.value = box.offsetHeight
    shadowReady.value = shadowLeft.value > 0 && tableHeight.value > 0
  })
}

onMounted(() => {
  updatePinMetrics()
  pinMetricsObserver = new ResizeObserver(updatePinMetrics)
  if (scrollRef.value)
    pinMetricsObserver.observe(scrollRef.value)
  if (tableBoxRef.value)
    pinMetricsObserver.observe(tableBoxRef.value)
  scrollRef.value?.addEventListener('scroll', updatePinMetrics, { passive: true })
  window.addEventListener('resize', updatePinMetrics)
  // 表格渲染完成后再量一次
  requestAnimationFrame(updatePinMetrics)
})

onUnmounted(() => {
  pinMetricsObserver?.disconnect()
  scrollRef.value?.removeEventListener('scroll', updatePinMetrics)
  window.removeEventListener('resize', updatePinMetrics)
})

watch(
  () => [props.data.length, props.loading, props.pinActionsColumn, columnVisibility.value, expanded.value, tableScrollableX.value] as const,
  updatePinMetrics,
)
</script>

<template>
  <div :class="cn('w-full min-w-0', fixedLayout && 'flex flex-col')">
    <!-- 工具栏 -->
    <div :class="cn('flex min-w-0 flex-wrap items-center gap-3 py-[3px] pb-4', fixedLayout && 'shrink-0')">
      <FilterField
        v-if="search"
        :label="searchLabel || t('common.search')"
      >
        <FilterSearchInput
          v-model="globalFilter"
          :placeholder="searchPlaceholder || t('common.search')"
        />
      </FilterField>
      <slot name="filters" :table="table" />

      <div class="ml-auto flex items-center gap-2">
        <slot name="leading-actions" :table="table" />
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button variant="outline" size="sm" class="h-10">
              <SlidersHorizontal class="size-4" /> {{ t('table.columns') }}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuCheckboxItem
              v-for="column in table.getAllColumns().filter((c) => c.getCanHide())"
              :key="column.id"
              :model-value="column.getIsVisible()"
              @update:model-value="(v) => column.toggleVisibility(!!v)"
            >
              {{ colLabel(column.id) }}
            </DropdownMenuCheckboxItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <slot name="actions" :table="table" />
      </div>
    </div>

    <div
      ref="scrollRef"
      :class="cn(
        'relative rounded-md border overflow-auto',
        fixedLayout && 'max-h-[min(calc(100svh-16rem),2000px)]',
      )"
    >
      <div ref="tableBoxRef" class="relative min-w-full w-max">
      <Table :class="tableClass">
        <TableHeader :class="fixedLayout && 'sticky top-0 z-10'">
          <TableRow v-for="hg in table.getHeaderGroups()" :key="hg.id">
            <TableHead v-for="header in hg.headers" :key="header.id" :class="headClass(header.column.id, header.column.columnDef.meta)">
              <FlexRender
                v-if="!header.isPlaceholder"
                :render="header.column.columnDef.header"
                :props="header.getContext()"
              />
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <!-- 加载骨架屏 -->
          <template v-if="loading">
            <TableRow v-for="r in 6" :key="`sk-${r}`" class="hover:bg-transparent">
              <TableCell v-for="c in leafCount" :key="c">
                <Skeleton class="h-4" :class="c === 1 ? 'w-8' : 'w-full max-w-[140px]'" />
              </TableCell>
            </TableRow>
          </template>
          <template v-else-if="table.getRowModel().rows.length">
            <template v-for="row in table.getRowModel().rows" :key="row.id">
              <TableRow class="group" :data-state="row.getIsExpanded() ? 'selected' : undefined">
                <TableCell
                  v-for="cell in row.getVisibleCells()"
                  :key="cell.id"
                  :class="cellClass(cell.column.id, cell.column.columnDef.meta)"
                >
                  <!-- 展开按钮列 -->
                  <Button
                    v-if="cell.column.id === 'expander'"
                    variant="ghost"
                    size="icon"
                    class="size-7"
                    @click="row.toggleExpanded()"
                  >
                    <ChevronRight
                      class="size-4 transition-transform"
                      :class="row.getIsExpanded() && 'rotate-90'"
                    />
                  </Button>
                  <!-- 自定义单元格插槽 cell-<id> -->
                  <slot
                    v-else-if="hasSlot(`cell-${cell.column.id}`)"
                    :name="`cell-${cell.column.id}`"
                    :row="row.original"
                    :value="cell.getValue()"
                  />
                  <!-- 列定义里的 cell 渲染函数 -->
                  <FlexRender
                    v-else-if="cell.column.columnDef.cell"
                    :render="cell.column.columnDef.cell"
                    :props="cell.getContext()"
                  />
                  <!-- 默认：直接展示取值 -->
                  <template v-else>
                    {{ cell.getValue() ?? '' }}
                  </template>
                </TableCell>
              </TableRow>
              <!-- 展开行 -->
              <TableRow v-if="expandable && row.getIsExpanded()" :key="`${row.id}-expanded`" class="hover:bg-transparent">
                <TableCell :colspan="leafCount" class="!h-auto bg-muted/30 p-0">
                  <slot name="expanded" :row="row.original" />
                </TableCell>
              </TableRow>
            </template>
          </template>
          <TableEmpty v-else :colspan="leafCount">
            {{ t('common.empty') }}
          </TableEmpty>
        </TableBody>
      </Table>
      <div
        v-if="pinActionsColumn && tableScrollableX && shadowReady"
        class="pinned-col-shadow"
        :style="{
          left: `${shadowLeft}px`,
          height: `${tableHeight}px`,
        }"
        aria-hidden="true"
      />
      </div>
    </div>

    <!-- 页脚：计数 + 分页 -->
    <div class="flex flex-wrap items-center justify-end gap-3 py-3 text-sm">
      <div class="text-muted-foreground mr-auto">
        {{ t('common.total', { n: table.getFilteredRowModel().rows.length }) }}
      </div>
      <div class="flex items-center gap-2">
        <Select
          :model-value="String(table.getState().pagination.pageSize)"
          @update:model-value="(v) => table.setPageSize(Number(v))"
        >
          <SelectTrigger size="sm" class="h-8 w-[110px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="s in pageSizeOptions" :key="s" :value="String(s)">
              {{ s }} {{ t('pagination.perPage') }}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div class="text-muted-foreground tabular-nums">
        {{ pageIndex + 1 }} / {{ Math.max(1, pageCount) }}
      </div>
      <div class="flex gap-1">
        <Button
          variant="outline"
          size="icon"
          class="size-8"
          :disabled="!table.getCanPreviousPage()"
          @click="table.previousPage()"
        >
          <ChevronLeft class="size-4" />
        </Button>
        <Button
          variant="outline"
          size="icon"
          class="size-8"
          :disabled="!table.getCanNextPage()"
          @click="table.nextPage()"
        >
          <ChevronRight class="size-4" />
        </Button>
      </div>
    </div>
  </div>
</template>
