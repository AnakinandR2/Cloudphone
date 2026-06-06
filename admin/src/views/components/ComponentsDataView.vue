<script setup lang="ts">
import type { ColumnDef, ColumnFiltersState, ExpandedState, RowSelectionState, SortingState, VisibilityState } from '@tanstack/vue-table'
import {

  getCoreRowModel,
  getExpandedRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useVueTable,
} from '@tanstack/vue-table'
import {
  ArrowDown,
  ArrowUp,
  ChevronDown,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  ChevronsUpDown,
  Download,
  Eye,
  MoreHorizontal,
  Pencil,
  Plus,
  Search,
  SlidersHorizontal,
  Trash2,
  X,
} from 'lucide-vue-next'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
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
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { valueUpdater } from '@/lib/table'

interface Member {
  id: number
  name: string
  email: string
  role: 'admin' | 'editor' | 'viewer'
  status: 'active' | 'pending' | 'offline'
  score: number
  createdAt: string
  avatar: number
  bio: string
}

const { t } = useI18n()

const ROLES: Member['role'][] = ['admin', 'editor', 'viewer']
const STATUSES: Member['status'][] = ['active', 'pending', 'offline']
const FIRST = ['Alice', 'Bob', 'Carol', 'David', 'Eve', 'Frank', 'Grace', 'Henry', 'Ivy', 'Jack', 'Kate', 'Leo']
const LAST = ['Chen', 'Liu', 'Wang', 'Zhao', 'Sun', 'Zhou', 'Wu', 'Zheng']

const members = ref<Member[]>(
  Array.from({ length: 23 }, (_, i) => {
    const name = `${FIRST[i % FIRST.length]} ${LAST[i % LAST.length]}`
    return {
      id: i + 1,
      name,
      email: `${name.toLowerCase().replace(' ', '.')}@glory.io`,
      role: ROLES[i % ROLES.length],
      status: STATUSES[i % STATUSES.length],
      score: 40 + ((i * 37) % 60),
      createdAt: `2026-0${(i % 9) + 1}-${String((i * 7) % 28 + 1).padStart(2, '0')}`,
      avatar: (i % 60) + 1,
      bio: `${name} — ${ROLES[i % ROLES.length]} on the Glory Phone team, focused on shipping delightful experiences.`,
    }
  }),
)

const statusStyle: Record<Member['status'], string> = {
  active: 'border-transparent bg-emerald-500/15 text-emerald-600 dark:text-emerald-400',
  pending: 'border-transparent bg-amber-500/15 text-amber-600 dark:text-amber-400',
  offline: 'border-transparent bg-slate-500/15 text-slate-600 dark:text-slate-400',
}

// ── 列定义（渲染在模板里按 column.id 分发，这里只声明行为）──
const columns: ColumnDef<Member>[] = [
  { id: 'select', enableSorting: false, enableHiding: false },
  { id: 'expander', enableSorting: false, enableHiding: false },
  { accessorKey: 'name', id: 'name', meta: { label: 'table.name' } },
  { accessorKey: 'role', id: 'role', meta: { label: 'table.role' } },
  { accessorKey: 'status', id: 'status', meta: { label: 'table.status' }, filterFn: 'equals' },
  { accessorKey: 'score', id: 'score', meta: { label: 'comp.score' } },
  { accessorKey: 'createdAt', id: 'createdAt', meta: { label: 'table.createdAt' } },
  { id: 'actions', enableSorting: false, enableHiding: false },
]

const sorting = ref<SortingState>([])
const columnFilters = ref<ColumnFiltersState>([])
const columnVisibility = ref<VisibilityState>({})
const rowSelection = ref<RowSelectionState>({})
const expanded = ref<ExpandedState>({})
const globalFilter = ref('')

const table = useVueTable({
  get data() {
    return members.value
  },
  columns,
  getRowId: row => String(row.id),
  getCoreRowModel: getCoreRowModel(),
  getSortedRowModel: getSortedRowModel(),
  getFilteredRowModel: getFilteredRowModel(),
  getPaginationRowModel: getPaginationRowModel(),
  getExpandedRowModel: getExpandedRowModel(),
  onSortingChange: u => valueUpdater(u, sorting),
  onColumnFiltersChange: u => valueUpdater(u, columnFilters),
  onColumnVisibilityChange: u => valueUpdater(u, columnVisibility),
  onRowSelectionChange: u => valueUpdater(u, rowSelection),
  onExpandedChange: u => valueUpdater(u, expanded),
  onGlobalFilterChange: u => valueUpdater(u, globalFilter),
  initialState: { pagination: { pageSize: 20 } },
  state: {
    get sorting() { return sorting.value },
    get columnFilters() { return columnFilters.value },
    get columnVisibility() { return columnVisibility.value },
    get rowSelection() { return rowSelection.value },
    get expanded() { return expanded.value },
    get globalFilter() { return globalFilter.value },
  },
})

const visibleCols = computed(() => table.getVisibleLeafColumns().length)
const selectedRows = computed(() => table.getFilteredSelectedRowModel().rows)

// 状态筛选下拉
const statusFilter = computed<string>({
  get: () => (table.getColumn('status')?.getFilterValue() as string) ?? 'all',
  set: v => table.getColumn('status')?.setFilterValue(v === 'all' ? undefined : v),
})

const headerCheck = computed<boolean | 'indeterminate'>(() =>
  table.getIsAllPageRowsSelected()
    ? true
    : table.getIsSomePageRowsSelected()
      ? 'indeterminate'
      : false,
)

function colLabel(id: string) {
  const meta = columns.find(c => c.id === id)?.meta
  return meta?.label ? t(meta.label) : id
}

// ── 批量操作 ──
const batchOpen = ref(false)
function batchDelete() {
  const ids = new Set(selectedRows.value.map(r => r.original.id))
  members.value = members.value.filter(m => !ids.has(m.id))
  table.resetRowSelection()
  toast.success(t('comp.batchDeleted', { n: ids.size }))
  batchOpen.value = false
}
function exportSelected() {
  const rows = selectedRows.value.map(r => r.original)
  if (!rows.length) return
  const cols = ['id', 'name', 'email', 'role', 'status', 'score', 'createdAt'] as const
  const csv = [
    cols.join(','),
    ...rows.map(m => cols.map(c => JSON.stringify(String(m[c] ?? ''))).join(',')),
  ].join('\n')
  const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }))
  const a = document.createElement('a')
  a.href = url
  a.download = 'members.csv'
  a.click()
  URL.revokeObjectURL(url)
  toast.success(t('comp.exported', { n: rows.length }))
}

// ── 单行删除 ──
const delOpen = ref(false)
const pending = ref<Member | null>(null)
function askDelete(m: Member) {
  pending.value = m
  delOpen.value = true
}
function confirmDelete() {
  if (pending.value) {
    members.value = members.value.filter(x => x.id !== pending.value!.id)
    toast.success(t('comp.deleted', { name: pending.value.name }))
  }
  delOpen.value = false
}

// ── 新增 ──
const addOpen = ref(false)
const draft = ref({ name: '', email: '' })
function addMember() {
  if (!draft.value.name) return
  members.value.unshift({
    id: Math.max(0, ...members.value.map(m => m.id)) + 1,
    name: draft.value.name,
    email: draft.value.email || '—',
    role: 'viewer',
    status: 'pending',
    score: 50,
    createdAt: '2026-06-01',
    avatar: 60,
    bio: '—',
  })
  toast.success(t('comp.added', { name: draft.value.name }))
  draft.value = { name: '', email: '' }
  addOpen.value = false
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight">
          {{ t('comp.dataTitle') }}
        </h1>
        <p class="text-muted-foreground text-sm">
          {{ t('comp.dataDesc') }}
        </p>
      </div>
      <Dialog v-model:open="addOpen">
        <DialogTrigger as-child>
          <Button><Plus />{{ t('comp.addMember') }}</Button>
        </DialogTrigger>
        <DialogContent class="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{{ t('comp.addMember') }}</DialogTitle>
            <DialogDescription>{{ t('comp.addMemberDesc') }}</DialogDescription>
          </DialogHeader>
          <div class="space-y-4 py-2">
            <div class="space-y-2">
              <Label for="d-name">{{ t('table.name') }}</Label>
              <Input id="d-name" v-model="draft.name" :placeholder="t('comp.namePlaceholder')" />
            </div>
            <div class="space-y-2">
              <Label for="d-email">{{ t('table.email') }}</Label>
              <Input id="d-email" v-model="draft.email" type="email" placeholder="name@example.com" />
            </div>
          </div>
          <DialogFooter>
            <DialogClose as-child>
              <Button variant="outline">
                {{ t('common.cancel') }}
              </Button>
            </DialogClose>
            <Button @click="addMember">
              {{ t('common.confirm') }}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>

    <Card>
      <CardHeader>
        <CardTitle>{{ t('comp.tableTitle') }}</CardTitle>
        <CardDescription>{{ t('comp.tableDesc') }}</CardDescription>
      </CardHeader>
      <CardContent class="space-y-4">
        <!-- 工具栏：全局搜索 + 状态筛选 + 列显隐 -->
        <div class="flex flex-wrap items-center gap-2">
          <div class="relative w-full sm:w-64">
            <Search class="text-muted-foreground pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2" />
            <Input v-model="globalFilter" class="pl-9" :placeholder="t('comp.searchAll')" />
          </div>

          <Select v-model="statusFilter">
            <SelectTrigger class="w-[150px]">
              <SelectValue :placeholder="t('comp.filterStatus')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">
                {{ t('comp.allStatus') }}
              </SelectItem>
              <SelectItem v-for="s in STATUSES" :key="s" :value="s">
                {{ t(`comp.status_${s}`) }}
              </SelectItem>
            </SelectContent>
          </Select>

          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="outline" class="ml-auto">
                <SlidersHorizontal />{{ t('table.columns') }}<ChevronDown class="opacity-60" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuCheckboxItem
                v-for="column in table.getAllColumns().filter((c) => c.getCanHide())"
                :key="column.id"
                :model-value="column.getIsVisible()"
                @update:model-value="(v) => column.toggleVisibility(!!v)"
                @select="(e: Event) => e.preventDefault()"
              >
                {{ colLabel(column.id) }}
              </DropdownMenuCheckboxItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        <!-- 批量操作条（选中后出现） -->
        <Transition
          enter-active-class="transition duration-200 ease-out"
          enter-from-class="opacity-0 -translate-y-1"
          leave-active-class="transition duration-150 ease-in"
          leave-to-class="opacity-0 -translate-y-1"
        >
          <div
            v-if="selectedRows.length"
            class="bg-primary/5 border-primary/30 flex flex-wrap items-center gap-2 rounded-lg border px-3 py-2"
          >
            <span class="text-sm font-medium">
              {{ t('comp.selectedCount', { n: selectedRows.length, total: table.getFilteredRowModel().rows.length }) }}
            </span>
            <div class="ml-auto flex items-center gap-2">
              <Button size="sm" variant="outline" @click="exportSelected">
                <Download />{{ t('comp.export') }}
              </Button>
              <Button size="sm" variant="destructive" @click="batchOpen = true">
                <Trash2 />{{ t('comp.batchDelete') }}
              </Button>
              <Button size="sm" variant="ghost" @click="table.resetRowSelection()">
                <X />{{ t('comp.clearSel') }}
              </Button>
            </div>
          </div>
        </Transition>

        <!-- 表格 -->
        <div class="overflow-hidden rounded-lg border">
          <Table>
            <TableHeader>
              <TableRow
                v-for="hg in table.getHeaderGroups()"
                :key="hg.id"
                class="bg-muted/50 hover:bg-muted/50"
              >
                <TableHead
                  v-for="header in hg.headers"
                  :key="header.id"
                  :class="[
                    header.column.id === 'select' || header.column.id === 'expander' ? 'w-10' : '',
                    header.column.id === 'actions' ? 'w-12 text-right' : '',
                  ]"
                >
                  <!-- 全选复选框 -->
                  <Checkbox
                    v-if="header.column.id === 'select'"
                    :model-value="headerCheck"
                    :aria-label="t('comp.selectAll')"
                    @update:model-value="(v) => table.toggleAllPageRowsSelected(!!v)"
                  />
                  <!-- 可排序表头：点击循环 升序→降序→取消 -->
                  <button
                    v-else-if="header.column.getCanSort()"
                    type="button"
                    class="hover:text-foreground -ml-1 inline-flex items-center gap-1 rounded px-1 py-0.5 font-medium transition-colors"
                    @click="header.column.getToggleSortingHandler()?.($event)"
                  >
                    {{ colLabel(header.column.id) }}
                    <ArrowUp v-if="header.column.getIsSorted() === 'asc'" class="size-3.5" />
                    <ArrowDown v-else-if="header.column.getIsSorted() === 'desc'" class="size-3.5" />
                    <ChevronsUpDown v-else class="size-3.5 opacity-40" />
                  </button>
                  <span v-else-if="header.column.id !== 'expander'">{{ colLabel(header.column.id) }}</span>
                </TableHead>
              </TableRow>
            </TableHeader>

            <TableBody>
              <template v-if="table.getRowModel().rows.length">
                <template v-for="row in table.getRowModel().rows" :key="row.id">
                  <TableRow
                    :data-state="row.getIsSelected() ? 'selected' : undefined"
                    class="hover:bg-muted/40 data-[state=selected]:bg-primary/5 transition-colors"
                  >
                    <TableCell v-for="cell in row.getVisibleCells()" :key="cell.id">
                      <!-- 行选择 -->
                      <Checkbox
                        v-if="cell.column.id === 'select'"
                        :model-value="row.getIsSelected()"
                        :aria-label="t('comp.selectRow')"
                        @update:model-value="(v) => row.toggleSelected(!!v)"
                      />
                      <!-- 展开箭头 -->
                      <Button
                        v-else-if="cell.column.id === 'expander'"
                        variant="ghost"
                        size="icon-sm"
                        class="size-7"
                        @click="row.toggleExpanded()"
                      >
                        <ChevronRight
                          class="size-4 transition-transform duration-200"
                          :class="row.getIsExpanded() ? 'rotate-90' : ''"
                        />
                      </Button>
                      <!-- 姓名 + 头像 -->
                      <div v-else-if="cell.column.id === 'name'" class="flex items-center gap-3">
                        <Avatar class="size-9">
                          <AvatarImage :src="`https://i.pravatar.cc/120?img=${row.original.avatar}`" :alt="row.original.name" />
                          <AvatarFallback>{{ row.original.name.slice(0, 2) }}</AvatarFallback>
                        </Avatar>
                        <div class="min-w-0">
                          <p class="truncate font-medium">
                            {{ row.original.name }}
                          </p>
                          <p class="text-muted-foreground truncate text-xs">
                            {{ row.original.email }}
                          </p>
                        </div>
                      </div>
                      <!-- 角色 -->
                      <Badge v-else-if="cell.column.id === 'role'" variant="outline" class="capitalize">
                        {{ row.original.role }}
                      </Badge>
                      <!-- 状态 -->
                      <Badge
                        v-else-if="cell.column.id === 'status'"
                        class="gap-1"
                        :class="statusStyle[row.original.status]"
                      >
                        <span class="size-1.5 rounded-full bg-current opacity-70" />
                        {{ t(`comp.status_${row.original.status}`) }}
                      </Badge>
                      <!-- 评分 -->
                      <div v-else-if="cell.column.id === 'score'" class="flex items-center gap-2">
                        <div class="bg-muted h-1.5 w-16 overflow-hidden rounded-full">
                          <div class="bg-primary h-full rounded-full" :style="{ width: `${row.original.score}%` }" />
                        </div>
                        <span class="text-muted-foreground text-xs tabular-nums">{{ row.original.score }}</span>
                      </div>
                      <!-- 创建时间 -->
                      <span v-else-if="cell.column.id === 'createdAt'" class="text-muted-foreground text-sm tabular-nums">
                        {{ row.original.createdAt }}
                      </span>
                      <!-- 行操作菜单 -->
                      <div v-else-if="cell.column.id === 'actions'" class="text-right">
                        <DropdownMenu>
                          <DropdownMenuTrigger as-child>
                            <Button variant="ghost" size="icon-sm">
                              <MoreHorizontal />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end" class="w-40">
                            <DropdownMenuItem @click="row.toggleExpanded()">
                              <Eye class="size-4" />{{ t('comp.view') }}
                            </DropdownMenuItem>
                            <DropdownMenuItem><Pencil class="size-4" />{{ t('common.edit') }}</DropdownMenuItem>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                              class="text-destructive focus:text-destructive"
                              @click="askDelete(row.original)"
                            >
                              <Trash2 class="size-4" />{{ t('common.delete') }}
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </div>
                    </TableCell>
                  </TableRow>

                  <!-- 展开的详情行 -->
                  <TableRow v-if="row.getIsExpanded()" :key="`${row.id}-expanded`" class="hover:bg-transparent">
                    <TableCell :colspan="visibleCols" class="bg-muted/30">
                      <div class="grid gap-4 px-2 py-1 sm:grid-cols-3">
                        <div class="space-y-1">
                          <p class="text-muted-foreground text-xs">
                            {{ t('table.email') }}
                          </p>
                          <p class="text-sm font-medium">
                            {{ row.original.email }}
                          </p>
                        </div>
                        <div class="space-y-1">
                          <p class="text-muted-foreground text-xs">
                            {{ t('comp.score') }}
                          </p>
                          <p class="text-sm font-medium tabular-nums">
                            {{ row.original.score }} / 100
                          </p>
                        </div>
                        <div class="space-y-1">
                          <p class="text-muted-foreground text-xs">
                            {{ t('table.createdAt') }}
                          </p>
                          <p class="text-sm font-medium tabular-nums">
                            {{ row.original.createdAt }}
                          </p>
                        </div>
                        <div class="space-y-1 sm:col-span-3">
                          <p class="text-muted-foreground text-xs">
                            {{ t('comp.bio') }}
                          </p>
                          <p class="text-sm">
                            {{ row.original.bio }}
                          </p>
                        </div>
                      </div>
                    </TableCell>
                  </TableRow>
                </template>
              </template>
              <TableRow v-else class="hover:bg-transparent">
                <TableCell :colspan="visibleCols" class="text-muted-foreground h-24 text-center">
                  {{ t('common.empty') }}
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>

        <!-- 分页 -->
        <div class="flex flex-wrap items-center justify-between gap-4">
          <p class="text-muted-foreground text-sm">
            {{ t('comp.selectedCount', { n: selectedRows.length, total: table.getFilteredRowModel().rows.length }) }}
          </p>
          <div class="flex flex-wrap items-center gap-4">
            <div class="flex items-center gap-2">
              <span class="text-muted-foreground text-sm">{{ t('comp.rowsPerPage') }}</span>
              <Select
                :model-value="String(table.getState().pagination.pageSize)"
                @update:model-value="(v) => table.setPageSize(Number(v))"
              >
                <SelectTrigger class="w-[72px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="s in [20, 50, 100, 200]" :key="s" :value="String(s)">
                    {{ s }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <span class="text-muted-foreground text-sm tabular-nums">
              {{ t('comp.pageInfo', { page: table.getState().pagination.pageIndex + 1, pages: table.getPageCount() || 1 }) }}
            </span>
            <div class="flex items-center gap-1">
              <Button variant="outline" size="icon-sm" :disabled="!table.getCanPreviousPage()" @click="table.setPageIndex(0)">
                <ChevronsLeft />
              </Button>
              <Button variant="outline" size="sm" :disabled="!table.getCanPreviousPage()" @click="table.previousPage()">
                {{ t('common.prev') }}
              </Button>
              <Button variant="outline" size="sm" :disabled="!table.getCanNextPage()" @click="table.nextPage()">
                {{ t('common.next') }}
              </Button>
              <Button variant="outline" size="icon-sm" :disabled="!table.getCanNextPage()" @click="table.setPageIndex(table.getPageCount() - 1)">
                <ChevronsRight />
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- 批量删除确认 -->
    <Dialog v-model:open="batchOpen">
      <DialogContent class="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>{{ t('comp.batchDelete') }}</DialogTitle>
          <DialogDescription>{{ t('comp.batchDeleteDesc', { n: selectedRows.length }) }}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose as-child>
            <Button variant="outline">
              {{ t('common.cancel') }}
            </Button>
          </DialogClose>
          <Button variant="destructive" @click="batchDelete">
            {{ t('common.delete') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- 单行删除确认 -->
    <Dialog v-model:open="delOpen">
      <DialogContent class="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>{{ t('comp.confirmDelete') }}</DialogTitle>
          <DialogDescription>{{ t('comp.confirmDeleteDesc', { name: pending?.name }) }}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose as-child>
            <Button variant="outline">
              {{ t('common.cancel') }}
            </Button>
          </DialogClose>
          <Button variant="destructive" @click="confirmDelete">
            {{ t('common.delete') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
