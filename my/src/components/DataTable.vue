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
import { computed, ref, useSlots } from 'vue'
import { useI18n } from 'vue-i18n'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
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
import { valueUpdater } from '@/lib/table'

const props = withDefaults(
  defineProps<{
    columns: ColumnDef<TData, unknown>[]
    data: TData[]
    pageSize?: number
    pageSizes?: number[]
    search?: boolean
    searchPlaceholder?: string
    expandable?: boolean
    loading?: boolean
    getRowId?: (row: TData) => string
  }>(),
  { pageSize: 20, search: true, searchPlaceholder: '', expandable: false, loading: false },
)

// 每页条数可选项：默认 20，可选 50 / 100 / 200
const pageSizeOptions = computed(() => props.pageSizes ?? [20, 50, 100, 200])

const slots = useSlots()
const { t } = useI18n()

// 全局搜索值可由父级 v-model:search-value 接管（用于与 URL query 同步）
const globalFilter = defineModel<string>('searchValue', { default: '' })
const columnVisibility = ref<VisibilityState>({})
const expanded = ref<ExpandedState>({})

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
  enableSorting: false,
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
  },
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
</script>

<template>
  <div class="w-full">
    <!-- 工具栏 -->
    <div class="flex flex-wrap items-center gap-2 pb-4">
      <Input
        v-if="search"
        :model-value="globalFilter"
        class="h-9 max-w-xs"
        :placeholder="searchPlaceholder || t('common.search')"
        @update:model-value="table.setGlobalFilter(String($event))"
      />
      <slot name="filters" :table="table" />

      <div class="ml-auto flex items-center gap-2">
        <slot name="actions" :table="table" />
        <DropdownMenu>
          <DropdownMenuTrigger as-child>
            <Button variant="outline" size="sm" class="h-9">
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
      </div>
    </div>

    <div class="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow v-for="hg in table.getHeaderGroups()" :key="hg.id">
            <TableHead v-for="header in hg.headers" :key="header.id" :class="(header.column.columnDef.meta as any)?.headClass">
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
              <TableRow :data-state="row.getIsExpanded() ? 'selected' : undefined">
                <TableCell
                  v-for="cell in row.getVisibleCells()"
                  :key="cell.id"
                  :class="(cell.column.columnDef.meta as any)?.cellClass"
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
                <TableCell :colspan="leafCount" class="bg-muted/30 p-0">
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
