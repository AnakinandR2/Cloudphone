<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { CloudPhone, Tag as TagType } from '@/types/phone'
import { EllipsisVertical } from 'lucide-vue-next'
import {
  Add2 as Plus,
  AlarmClock as FileClock,
  Grid as LayoutGrid,
  Menu,
  Monitor,
  Pen as SquarePen,
  Shield,
  ShieldCheck,
  Tag,
  TerminalSquare as Terminal,
  Trash,
  Usb,
  Users,
} from 'reicon-vue'
import { computed, h, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import phoneApi from '@/api/modules/phone'
import proxyApi from '@/api/modules/proxy'
import DataTable from '@/components/DataTable.vue'
import FilterBar from '@/components/FilterBar.vue'
import FilterField from '@/components/FilterField.vue'
import FilterSearchInput from '@/components/FilterSearchInput.vue'
import {
  filterPopupAlign,
  filterPopupSideOffset,
  filterSelectContentClass,
  filterSelectTriggerClass,
} from '@/components/filterField'
import Popconfirm from '@/components/Popconfirm.vue'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button, buttonVariants } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { useQuerySync } from '@/composables/useQuerySync'
import { ACTIONS_COLUMN_META } from '@/lib/table'
import { formatDateTime } from '@/utils/date'
import { phoneStatusBadge, STATUS_BADGE_DOT } from '@/utils/statusBadge'
import { tagClass } from '@/utils/tagColor'
import AndroidIcon from '@/components/icons/AndroidIcon.vue'
import PowerSwitchIcon from '@/components/icons/PowerSwitchIcon.vue'
import TruncatedText from '@/components/TruncatedText.vue'
import AdbDrawer from './AdbDrawer.vue'
import RunLogDialog from './RunLogDialog.vue'
import TaskCreateDialog from '../automation/TaskCreateDialog.vue'
import TaskReportDialog from '../automation/TaskReportDialog.vue'
import AppManagerDialog from './AppManagerDialog.vue'
import PhoneFormDialog from './PhoneFormDialog.vue'
import PhoneTagsDialog from './PhoneTagsDialog.vue'

const { t } = useI18n()
const powerOnBtnClass =
  'bg-sidebar-primary/10 text-sidebar-primary shadow-none hover:bg-sidebar-primary/15 hover:text-sidebar-primary hover:translate-y-0 active:translate-y-0 dark:hover:bg-sidebar-primary/15'
const powerOnBtnLoadingClass = 'animate-pulse disabled:opacity-100'
const data = ref<CloudPhone[]>([])
const loading = ref(false)
const filters = reactive({ q: '' })
useQuerySync(filters, { q: '' })

// 视图模式：表格 / 卡片，记住用户选择。
const viewMode = ref<'table' | 'card'>(
  (localStorage.getItem('phone:view') as 'table' | 'card') || 'card',
)
function setView(v: 'table' | 'card') {
  viewMode.value = v
  localStorage.setItem('phone:view', v)
}

// 卡片多选 → 群控。
const selectedIds = ref<Set<number>>(new Set())
function toggleSelect(id: number, checked: boolean) {
  const next = new Set(selectedIds.value)
  if (checked)
    next.add(id)
  else next.delete(id)
  selectedIds.value = next
}
const allRowsSelected = computed(() => data.value.length > 0 && data.value.every(p => selectedIds.value.has(p.id)))
function toggleSelectAll(checked: boolean) {
  selectedIds.value = checked ? new Set(data.value.map(p => p.id)) : new Set()
}
function openGroupControl() {
  const list = [...selectedIds.value]
  if (!list.length)
    return
  const aw = window.screen?.availWidth ?? 1280
  const ah = window.screen?.availHeight ?? 800
  const w = Math.min(1600, Math.round(aw * 0.9))
  const h = Math.min(1000, Math.round(ah * 0.85))
  window.open(`${import.meta.env.BASE_URL}phone/group?ids=${list.join(',')}`, 'cp-group', `popup=yes,width=${w},height=${h},resizable=yes`)
}
// 卡片视图的客户端搜索过滤（表格视图由 DataTable 内部过滤）。
// 状态/标签过滤下推后端（list 参数）；选项用独立来源，避免被当前结果集裁剪。
const statusFilter = ref('')
const ALL_FILTER_VALUE = '__all__'
const statusFilterModel = computed({
  get: () => statusFilter.value || ALL_FILTER_VALUE,
  set: value => (statusFilter.value = value === ALL_FILTER_VALUE ? '' : value),
})
const statusOptions = ['RUNNING', 'STOPPED', 'STARTING', 'STOPPING', 'CREATING', 'CREATE_FAILED', 'DESTROYING', 'CREATED']
const tagFilter = ref('')
const tagOptions = ref<string[]>([])
const tagFilterModel = computed({
  get: () => tagFilter.value || ALL_FILTER_VALUE,
  set: value => (tagFilter.value = value === ALL_FILTER_VALUE ? '' : value),
})
async function loadTagOptions() {
  try {
    tagOptions.value = (await phoneApi.listTags()).data?.map(tg => tg.name) ?? []
  }
  catch {
    tagOptions.value = []
  }
}
// 关键词搜索仍在前端（在后端已按状态/标签过滤的结果集内进一步匹配）。
const filteredData = computed(() => {
  const q = filters.q.trim().toLowerCase()
  if (!q)
    return data.value
  return data.value.filter(p =>
    p.name.toLowerCase().includes(q)
    || (p.cp_id || '').toLowerCase().includes(q)
    || (p.tags ?? []).some(tg => tg.name.toLowerCase().includes(q)),
  )
})

// 标签弹框（批量=多选；单台=传 [id]+现有标签预填）
const tagDialog = ref<{ open: boolean, ids: number[], initial: TagType[] }>({ open: false, ids: [], initial: [] })
function openTagDialog(ids: number[], initial: TagType[] = []) {
  tagDialog.value = { open: true, ids, initial }
}

// 代理 id → 地址（protocol://host:port）/ 名称 映射，列表「绑定代理」列用地址展示
const proxyAddrMap = ref<Map<number, string>>(new Map())
const proxyNameMap = ref<Map<number, string>>(new Map())

async function loadProxyOptions() {
  try {
    // 用全部代理（含 fail 状态）建展示映射；options 仅含「可用」会让已绑定的失败代理回退成 #id。
    const res = await proxyApi.list({ page: 1, size: 999 })
    const addr = new Map<number, string>()
    const name = new Map<number, string>()
    for (const p of res.data.list) {
      addr.set(p.id, `${p.protocol}://${p.host}:${p.port}${p.region ? ` · ${p.region}` : ''}`)
      name.set(p.id, p.name)
    }
    proxyAddrMap.value = addr
    proxyNameMap.value = name
  }
  catch {
    proxyAddrMap.value = new Map()
    proxyNameMap.value = new Map()
  }
}

function proxyLabel(id: number): string {
  if (id <= 0)
    return t('phone.proxyUnbound')
  return proxyAddrMap.value.get(id) ?? `#${id}`
}

const columns = computed<ColumnDef<CloudPhone>[]>(() => [
  {
    id: 'select',
    enableHiding: false,
    header: () => h(Checkbox, {
      modelValue: allRowsSelected.value,
      class: 'size-4',
      'onUpdate:modelValue': (checked: boolean | 'indeterminate') => toggleSelectAll(checked === true),
    }),
    meta: { headClass: 'w-12 pl-4', cellClass: 'w-12 pl-4' },
  },
  { id: 'expander', header: '', enableHiding: false, meta: { cellClass: 'w-8' } },
  { accessorKey: 'id', id: 'id', header: t('table.id'), meta: { label: 'table.id' } },
  { accessorKey: 'name', id: 'name', header: t('phone.colName'), meta: { label: 'phone.colName' } },
  { accessorKey: 'cp_id', id: 'cp_id', header: t('phone.colCpId'), meta: { label: 'phone.colCpId' } },
  { accessorKey: 'status', id: 'status', header: t('phone.colStatus'), meta: { label: 'phone.colStatus' } },
  { accessorKey: 'proxy_id', id: 'proxy_id', header: t('phone.colProxy'), meta: { label: 'phone.colProxy' } },
  { id: 'tags', header: t('phone.tag.col'), enableHiding: true, meta: { label: 'phone.tag.col' } },
  { accessorKey: 'remark', id: 'remark', header: t('phone.colRemark'), enableHiding: true, meta: { label: 'phone.colRemark' } },
  { accessorKey: 'created_at', id: 'created_at', header: t('table.createdAt'), meta: { label: 'table.createdAt' } },
  { id: 'actions', header: t('crud.actions'), enableHiding: false, meta: ACTIONS_COLUMN_META },
])

const dialog = ref({ open: false, id: 0, mode: 'create' as 'create' | 'edit' | 'view' })
const appDialog = ref({ open: false, phone: null as CloudPhone | null })
const adbDrawer = ref({ open: false, phone: null as CloudPhone | null })
const runLogDialog = ref({ open: false, phone: null as CloudPhone | null })
const scriptDialog = ref({ open: false, cpId: '' })
const scriptReport = ref({ open: false, midTaskId: null as number | null })
const opBusy = ref(false)
const poweringOnId = ref<number | null>(null)

// 危险操作（销毁/删除/重置）统一用一个「受控 AlertDialog」确认：
// 之前把 Popconfirm 气泡套在下拉项里，鼠标移向气泡时下拉判定移出而关闭，气泡随之卸载、点不到。
// 改为点击下拉项即关闭下拉并打开根级 AlertDialog，避免浮层套浮层。
const confirmState = ref<{ open: boolean, title: string, desc: string, run: () => void }>({
  open: false,
  title: '',
  desc: '',
  run: () => {},
})
function ask(title: string, desc: string, run: () => void) {
  confirmState.value = { open: true, title, desc, run }
}

// 状态机门禁（与后端一致）：
//   CREATING / STARTING：过渡态，禁止任何操作
//   CREATE_FAILED：仅可删除         CREATED / STOPPED：可开机、可删除
//   RUNNING：可关机、可远控、可管应用；不可删除
function canPowerOn(s: string) {
  return s === 'CREATED' || s === 'STOPPED'
}
function isPoweringOn(s: string) {
  return s === 'STARTING'
}
function isPowerOnLoading(row: CloudPhone) {
  return poweringOnId.value === row.id || isPoweringOn(row.status)
}
function showPowerOnBtn(row: CloudPhone) {
  return canPowerOn(row.status) || isPowerOnLoading(row)
}
function canPowerOff(s: string) {
  return s === 'RUNNING'
}
// 销毁：运行中需先停止，过渡态禁止；CREATE_FAILED/CREATED/STOPPED 可销毁。
function canDestroy(s: string) {
  return s === 'CREATE_FAILED' || s === 'CREATED' || s === 'STOPPED'
}
function isRunning(s: string) {
  return s === 'RUNNING'
}

// 列表状态每 4s 静默轮询自动刷新，无需手动刷新（CP-0068 / #58）：无论过渡态/终态都持续拉取，
// 使外部驱动的状态变化（worker 收敛、别处关机、中台侧变更）也能自动反映到列表。
// 页面不可见（切后台/切标签页）时暂停、恢复可见时立即刷新一次再续，避免后台空跑浪费。
let pollTimer: ReturnType<typeof setInterval> | null = null
function startPolling() {
  if (!pollTimer)
    pollTimer = setInterval(() => load(true), 4000)
}
function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}
function onVisibilityChange() {
  if (document.hidden) {
    stopPolling()
  }
  else {
    load(true) // 回到前台立即刷新一次
    startPolling()
  }
}

// silent=true：后台轮询刷新，不切骨架屏（避免表格抖动）。
// loadSeq：丢弃过期响应，避免「保存标签后刷新」被更早发出的轮询结果盖回空标签。
let loadSeq = 0
async function load(silent = false) {
  const seq = ++loadSeq
  if (!silent)
    loading.value = true
  try {
    const res = await phoneApi.list({
      page: 1,
      size: 999,
      status: statusFilter.value || undefined,
      tag: tagFilter.value || undefined,
    })
    if (seq !== loadSeq)
      return
    data.value = res.data.list
  }
  finally {
    if (!silent && seq === loadSeq)
      loading.value = false
  }
}
watch([statusFilter, tagFilter], () => load())
function refreshAfterTag() {
  void load()
  void loadTagOptions()
}
function openCreate() {
  dialog.value = { open: true, id: 0, mode: 'create' }
}
function openEdit(row: CloudPhone) {
  dialog.value = { open: true, id: row.id, mode: 'edit' }
}
// 销毁：调中台销毁实例并移除本地档案（后端 delete 已是销毁语义：有 cpId 先 destroy 中台再删本地）。
async function destroyRow(row: CloudPhone) {
  await phoneApi.delete(row.id)
  toast.success(t('phone.op.destroyingOk'))
  load()
}

// 远程控制：以独立浏览器窗口（弹窗）打开，不带后台框架，专注投屏操控。
// 默认宽高按云手机竖屏比例（9:16）算，画面铺满高度、宽度贴合，避免黑边；右侧留出窄菜单列。
function openRemoteControl(row: CloudPhone) {
  const ASPECT = 9 / 16 // 竖屏画面比例（与远控默认分辨率一致）
  const SIDEBAR = 56 // 菜单列宽度（w-14）
  // 取屏幕可用高度的 ~88%，再据比例反推画面宽。
  const avail = window.screen?.availHeight ?? 960
  const h = Math.min(960, Math.max(560, Math.round(avail * 0.88)))
  const w = Math.round(h * ASPECT) + SIDEBAR
  const left = window.screenX + Math.max(0, (window.outerWidth - w) / 2)
  const top = window.screenY + 40
  window.open(
    `${import.meta.env.BASE_URL}phone/remote/${row.id}`,
    `cp-remote-${row.id}`,
    `popup=yes,width=${w},height=${h},left=${Math.round(left)},top=${Math.round(top)},resizable=yes`,
  )
}
function openAppManager(row: CloudPhone) {
  appDialog.value = { open: true, phone: row }
}
function openAdb(row: CloudPhone) {
  adbDrawer.value = { open: true, phone: row }
}
function openRunLogs(row: CloudPhone) {
  runLogDialog.value = { open: true, phone: row }
}
function openScriptTest(row: CloudPhone) {
  scriptDialog.value = { open: true, cpId: row.cp_id }
}
function onScriptCreated(firstTaskId: number | null) {
  if (firstTaskId != null)
    scriptReport.value = { open: true, midTaskId: firstTaskId }
}

// 通用操作执行：执行 -> 成功 toast -> 可选刷新列表
async function runOp(fn: () => Promise<unknown>, okMsg: string, refresh = true) {
  if (opBusy.value)
    return
  opBusy.value = true
  try {
    await fn()
    toast.success(okMsg)
    if (refresh)
      load()
  }
  finally {
    opBusy.value = false
  }
}

async function powerOn(row: CloudPhone) {
  // 业务硬约束：未绑定代理不可开机。前端提前拦截并引导去编辑里绑定代理（后端亦兜底）。
  if (!row.proxy_id || row.proxy_id <= 0) {
    toast.error(t('phone.op.needProxy'))
    openEdit(row)
    return
  }
  if (opBusy.value)
    return
  opBusy.value = true
  poweringOnId.value = row.id
  try {
    const res = await phoneApi.power(row.id, '开机')
    // 开机成功；若后端附带代理连通性告警（短超时探测未通过，CP-0028 / #23），
    // 黄色提示但不阻断——开机仍继续。
    const warn = res?.data?.warning
    if (warn)
      toast.warning(warn)
    else
      toast.success(t('phone.op.powerOnOk'))
    load()
  }
  finally {
    poweringOnId.value = null
    opBusy.value = false
  }
}
function powerOff(row: CloudPhone) {
  runOp(() => phoneApi.power(row.id, '关机'), t('phone.op.powerOffOk'))
}
// Root 开关（中台同步生效 ~8s，刷新列表以更新标记）。
function toggleRoot(row: CloudPhone, enable: boolean) {
  runOp(
    () => phoneApi.root(row.id, enable),
    enable ? t('phone.root.enableOk') : t('phone.root.disableOk'),
  )
}

onMounted(() => {
  load()
  loadProxyOptions()
  loadTagOptions()
  startPolling()
  document.addEventListener('visibilitychange', onVisibilityChange)
})
onUnmounted(() => {
  stopPolling()
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
</script>

<template>
  <div class="flex h-full min-h-0 min-w-0 flex-col overflow-hidden">
    <Card class="flex min-h-0 flex-1 flex-col gap-4 overflow-hidden py-5">
      <CardHeader class="shrink-0">
      <div class="flex min-w-0 items-center justify-between gap-4">
        <div class="min-w-0 space-y-1.5">
          <CardTitle>{{ t('phone.title') }}</CardTitle>
          <CardDescription>{{ t('phone.desc') }}</CardDescription>
        </div>
        <div class="flex shrink-0">
          <Button
            class="h-[52px] gap-2.5 rounded-xl px-[32px] has-[>svg]:px-[32px] text-lg font-semibold shadow-[var(--btn-shadow-hover)]"
            @click="openCreate"
          >
            <Plus class="size-6 shrink-0" stroke-width="2.5" />
            {{ t('phone.add') }}
          </Button>
        </div>
      </div>
    </CardHeader>
    <CardContent class="flex min-h-0 min-w-0 flex-1 flex-col items-stretch overflow-auto">
      <DataTable
        v-if="viewMode === 'table'"
        v-model:search-value="filters.q"
        fixed-layout
        pin-actions-column
        class="w-full shrink-0"
        :columns="columns"
        :data="data"
        :loading="loading"
        expandable
        :search="false"
        :get-row-id="(r) => String(r.id)"
      >
        <template #filters>
          <FilterBar class="min-w-0 flex-1">
            <FilterField :label="t('phone.searchLabel')">
              <FilterSearchInput v-model="filters.q" :placeholder="t('phone.searchPlaceholder')" />
            </FilterField>
            <FilterField :label="t('phone.filterStatus')">
              <Select v-model="statusFilterModel" class="flex h-full min-w-0 flex-1">
                <SelectTrigger :class="filterSelectTriggerClass">
                  <SelectValue :placeholder="t('phone.tag.statusAll')" />
                </SelectTrigger>
                <SelectContent
                  :align="filterPopupAlign"
                  :side-offset="filterPopupSideOffset"
                  :class="filterSelectContentClass"
                >
                  <SelectItem :value="ALL_FILTER_VALUE">
                    {{ t('phone.tag.statusAll') }}
                  </SelectItem>
                  <SelectItem v-for="st in statusOptions" :key="st" :value="st">
                    {{ t(`phone.status_${st}`, st) }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </FilterField>
            <FilterField :label="t('phone.filterTag')">
              <Select v-model="tagFilterModel" class="flex h-full min-w-0 flex-1">
                <SelectTrigger :class="filterSelectTriggerClass">
                  <SelectValue :placeholder="t('phone.tag.tagAll')" />
                </SelectTrigger>
                <SelectContent
                  :align="filterPopupAlign"
                  :side-offset="filterPopupSideOffset"
                  :class="filterSelectContentClass"
                >
                  <SelectItem :value="ALL_FILTER_VALUE">
                    {{ t('phone.tag.tagAll') }}
                  </SelectItem>
                  <SelectItem v-for="tg in tagOptions" :key="tg" :value="tg">
                    {{ tg }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </FilterField>
          </FilterBar>
        </template>
        <template #leading-actions>
          <div class="flex items-center rounded-md border p-0.5">
            <Button
              variant="ghost"
              size="icon"
              class="size-7"
              :class="viewMode === 'table' ? 'bg-muted text-foreground' : 'text-muted-foreground'"
              :title="t('phone.viewTable')"
              @click="setView('table')"
            >
              <Menu class="size-4" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              class="size-7"
              :class="viewMode === 'card' ? 'bg-muted text-foreground' : 'text-muted-foreground'"
              :title="t('phone.viewCard')"
              @click="setView('card')"
            >
              <LayoutGrid class="size-4" />
            </Button>
          </div>
          <Button size="sm" variant="outline" class="h-10" :disabled="!selectedIds.size" @click="openTagDialog([...selectedIds])">
            <Tag class="size-4" /> {{ t('phone.tag.batchBtn') }}<span v-if="selectedIds.size">（{{ selectedIds.size }}）</span>
          </Button>
          <Button size="sm" variant="outline" class="h-10" :disabled="!selectedIds.size" @click="openGroupControl">
            <Users class="size-4" /> {{ t('phone.groupControl') }}<span v-if="selectedIds.size">（{{ selectedIds.size }}）</span>
          </Button>
        </template>
        <template #cell-select="{ row }">
          <Checkbox
            :model-value="selectedIds.has(row.id)"
            @update:model-value="checked => toggleSelect(row.id, checked === true)"
          />
        </template>
        <template #cell-id="{ row }">
          <span class="text-muted-foreground">#{{ row.id }}</span>
        </template>
        <template #cell-name="{ row }">
          <TruncatedText :text="row.name" class="font-medium" />
        </template>
        <template #cell-cp_id="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ row.cp_id || '-' }}</span>
        </template>
        <template #cell-status="{ row }">
          <Badge v-bind="phoneStatusBadge(row.status)">
            <span :class="STATUS_BADGE_DOT" />
            {{ t(`phone.status_${row.status}`, row.status) }}
          </Badge>
        </template>
        <template #cell-proxy_id="{ row }">
          <span class="text-muted-foreground" :class="row.proxy_id > 0 && 'font-mono'">{{ proxyLabel(row.proxy_id) }}</span>
        </template>
        <template #cell-tags="{ row }">
          <div class="flex flex-wrap gap-1">
            <Badge v-for="tg in row.tags" :key="tg.name" variant="outline" :class="tagClass(tg.color)">
              <TruncatedText :text="tg.name" />
            </Badge>
            <span v-if="!row.tags?.length" class="text-muted-foreground">-</span>
          </div>
        </template>
        <template #cell-remark="{ row }">
          <TruncatedText :text="row.remark" :max="20" class="text-muted-foreground" />
        </template>
        <template #cell-created_at="{ row }">
          <span class="text-muted-foreground tabular-nums">{{ formatDateTime(row.created_at) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center justify-end gap-2">
            <!-- 常用操作：开关机在远程控制左侧 -->
            <Button
              v-if="showPowerOnBtn(row)"
              size="sm"
              variant="ghost"
              class="relative"
              :class="[powerOnBtnClass, isPowerOnLoading(row) && powerOnBtnLoadingClass]"
              :disabled="isPowerOnLoading(row) || opBusy"
              @click="powerOn(row)"
            >
              <span
                class="inline-flex items-center gap-1.5"
                :class="isPowerOnLoading(row) && 'invisible'"
              >
                <PowerSwitchIcon class="size-4 shrink-0" />
                {{ t('phone.op.powerOn') }}
              </span>
              <span
                v-if="isPowerOnLoading(row)"
                class="absolute inset-0 flex items-center justify-center"
              >
                {{ t('phone.op.powerOnPending') }}
              </span>
            </Button>
            <Popconfirm
              v-if="canPowerOff(row.status)"
              :title="t('phone.op.powerOffConfirm', { name: row.name })"
              @confirm="powerOff(row)"
            >
              <Button
                size="sm"
                variant="destructive"
                :disabled="opBusy"
              >
                <PowerSwitchIcon class="size-4" /> {{ t('phone.op.powerOff') }}
              </Button>
            </Popconfirm>
            <Button
              size="sm"
              variant="outline"
              :disabled="!isRunning(row.status)"
              @click="openRemoteControl(row)"
            >
              <Monitor class="size-4" /> {{ t('phone.op.remoteControl') }}
            </Button>
            <Button
              v-if="row.adb_enabled"
              size="sm"
              variant="outline"
              class="gap-1 border-green-500 text-green-600 hover:bg-green-50 hover:text-green-700 dark:text-green-400 dark:hover:bg-green-950"
              :title="t('phone.adb.markerTip')"
              @click="openAdb(row)"
            >
              <AndroidIcon class="size-4" /> ADB
            </Button>

            <!-- 更多：编辑 / 应用管理 / 删除 -->
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button variant="outline" size="icon-sm" :title="t('crud.more')">
                  <EllipsisVertical class="size-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" class="w-40">
                <DropdownMenuItem @click="openEdit(row)">
                  <SquarePen class="size-4" /> {{ t('crud.edit') }}
                </DropdownMenuItem>
                <DropdownMenuItem @click="openTagDialog([row.id], row.tags ?? [])">
                  <Tag class="size-4" /> {{ t('phone.tag.title') }}
                </DropdownMenuItem>
                <DropdownMenuItem v-if="isRunning(row.status)" @click="openAppManager(row)">
                  <LayoutGrid class="size-4" /> {{ t('phone.app.title') }}
                </DropdownMenuItem>
                <DropdownMenuItem v-if="isRunning(row.status)" @click="openAdb(row)">
                  <Usb class="size-4" /> {{ t('phone.adb.title') }}
                </DropdownMenuItem>
                <DropdownMenuItem v-if="isRunning(row.status) && !row.rooted" @click="toggleRoot(row, true)">
                  <Shield class="size-4" /> {{ t('phone.root.enable') }}
                </DropdownMenuItem>
                <DropdownMenuItem v-if="isRunning(row.status) && row.rooted" @click="toggleRoot(row, false)">
                  <ShieldCheck class="size-4 text-emerald-600" /> {{ t('phone.root.disable') }}
                </DropdownMenuItem>
                <DropdownMenuItem v-if="isRunning(row.status)" @click="openScriptTest(row)">
                  <Terminal class="size-4" /> {{ t('phone.script.menu') }}
                </DropdownMenuItem>
                <DropdownMenuItem v-if="row.cp_id" @click="openRunLogs(row)">
                  <FileClock class="size-4" /> {{ t('phone.runLog.title') }}
                </DropdownMenuItem>
                <template v-if="canDestroy(row.status)">
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    class="text-destructive focus:text-destructive"
                    @click="ask(t('phone.op.destroy'), t('phone.op.destroyConfirm', { name: row.name }), () => destroyRow(row))"
                  >
                    <Trash class="size-4" /> {{ t('phone.op.destroy') }}
                  </DropdownMenuItem>
                </template>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </template>

        <!-- 行展开 = 查看详情 -->
        <template #expanded="{ row }">
          <div class="grid grid-cols-2 gap-x-8 gap-y-2 p-4 text-sm md:grid-cols-3">
            <div><span class="text-muted-foreground">{{ t('phone.colName') }}：</span>{{ row.name }}</div>
            <div><span class="text-muted-foreground">{{ t('phone.colCpId') }}：</span><span class="tabular-nums">{{ row.cp_id || '-' }}</span></div>
            <div>
              <span class="text-muted-foreground">{{ t('phone.colStatus') }}：</span>
              <Badge v-bind="phoneStatusBadge(row.status)">
                <span :class="STATUS_BADGE_DOT" />
                {{ t(`phone.status_${row.status}`, row.status) }}
              </Badge>
            </div>
            <div><span class="text-muted-foreground">{{ t('phone.detailVmId') }}：</span><span class="font-mono text-xs">{{ row.vm_id || '-' }}</span></div>
            <div><span class="text-muted-foreground">{{ t('phone.fImageId') }}：</span><span class="font-mono text-xs">{{ row.image_id || '-' }}</span></div>
            <div><span class="text-muted-foreground">{{ t('phone.colProxy') }}：</span>{{ proxyLabel(row.proxy_id) }}</div>
            <div class="col-span-2 md:col-span-3">
              <span class="text-muted-foreground">{{ t('phone.fRemark') }}：</span>{{ row.remark || '-' }}
            </div>
            <div><span class="text-muted-foreground">{{ t('table.createdAt') }}：</span><span class="tabular-nums">{{ formatDateTime(row.created_at) }}</span></div>
            <div><span class="text-muted-foreground">{{ t('phone.updatedAt') }}：</span><span class="tabular-nums">{{ formatDateTime(row.updated_at) }}</span></div>
          </div>
        </template>
      </DataTable>

      <!-- 卡片视图 -->
      <div v-else class="flex min-h-0 min-w-0 w-full flex-1 flex-col overflow-hidden">
        <div class="mb-4 flex min-w-0 shrink-0 flex-wrap items-center gap-3 py-[3px]">
          <FilterBar class="min-w-0 flex-1">
            <FilterField :label="t('phone.searchLabel')">
              <FilterSearchInput v-model="filters.q" :placeholder="t('phone.searchPlaceholder')" />
            </FilterField>
            <FilterField :label="t('phone.filterStatus')">
              <Select v-model="statusFilterModel" class="flex h-full min-w-0 flex-1">
                <SelectTrigger :class="filterSelectTriggerClass">
                  <SelectValue :placeholder="t('phone.tag.statusAll')" />
                </SelectTrigger>
                <SelectContent
                  :align="filterPopupAlign"
                  :side-offset="filterPopupSideOffset"
                  :class="filterSelectContentClass"
                >
                  <SelectItem :value="ALL_FILTER_VALUE">
                    {{ t('phone.tag.statusAll') }}
                  </SelectItem>
                  <SelectItem v-for="st in statusOptions" :key="st" :value="st">
                    {{ t(`phone.status_${st}`, st) }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </FilterField>
            <FilterField :label="t('phone.filterTag')">
              <Select v-model="tagFilterModel" class="flex h-full min-w-0 flex-1">
                <SelectTrigger :class="filterSelectTriggerClass">
                  <SelectValue :placeholder="t('phone.tag.tagAll')" />
                </SelectTrigger>
                <SelectContent
                  :align="filterPopupAlign"
                  :side-offset="filterPopupSideOffset"
                  :class="filterSelectContentClass"
                >
                  <SelectItem :value="ALL_FILTER_VALUE">
                    {{ t('phone.tag.tagAll') }}
                  </SelectItem>
                  <SelectItem v-for="tg in tagOptions" :key="tg" :value="tg">
                    {{ tg }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </FilterField>
          </FilterBar>
          <div class="ml-auto flex items-center gap-2">
            <div class="flex items-center rounded-md border p-0.5">
              <Button
                variant="ghost"
                size="icon"
                class="size-7"
                :class="viewMode === 'table' ? 'bg-muted text-foreground' : 'text-muted-foreground'"
                :title="t('phone.viewTable')"
                @click="setView('table')"
              >
                <Menu class="size-4" />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                class="size-7"
                :class="viewMode === 'card' ? 'bg-muted text-foreground' : 'text-muted-foreground'"
                :title="t('phone.viewCard')"
                @click="setView('card')"
              >
                <LayoutGrid class="size-4" />
              </Button>
            </div>
            <Button size="sm" variant="outline" class="h-9" :disabled="!selectedIds.size" @click="openTagDialog([...selectedIds])">
              <Tag class="size-4" /> {{ t('phone.tag.batchBtn') }}<span v-if="selectedIds.size">（{{ selectedIds.size }}）</span>
            </Button>
            <Button size="sm" variant="outline" class="h-9" :disabled="!selectedIds.size" @click="openGroupControl">
              <Users class="size-4" /> {{ t('phone.groupControl') }}<span v-if="selectedIds.size">（{{ selectedIds.size }}）</span>
            </Button>
          </div>
        </div>

        <div class="min-h-0 w-full flex-1 overflow-auto">
        <div v-if="loading" class="grid w-full grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-3">
          <Skeleton v-for="i in 10" :key="i" class="h-36 rounded-xl" />
        </div>
        <div v-else-if="!filteredData.length" class="w-full py-16 text-center text-sm text-muted-foreground">
          {{ t('common.empty') }}
        </div>
        <div v-else class="grid w-full grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-3">
          <Card v-for="row in filteredData" :key="row.id" class="flex flex-col gap-2 py-4">
            <CardHeader class="gap-1 px-4">
              <div class="flex items-center justify-between gap-2">
                <div class="flex min-w-0 items-center gap-2">
                  <Checkbox
                    class="shrink-0"
                    :model-value="selectedIds.has(row.id)"
                    @click.stop
                    @update:model-value="checked => toggleSelect(row.id, checked === true)"
                  />
                  <CardTitle class="text-sm">
                    <TruncatedText :text="row.name" />
                  </CardTitle>
                </div>
                <Badge v-bind="phoneStatusBadge(row.status)">
                  <span :class="STATUS_BADGE_DOT" />
                  {{ t(`phone.status_${row.status}`, row.status) }}
                </Badge>
              </div>
              <CardDescription class="truncate font-mono text-xs">
                {{ row.cp_id || '-' }}
              </CardDescription>
            </CardHeader>
            <CardContent class="flex-1 space-y-1 px-4 text-xs">
              <div class="flex items-center justify-between gap-2">
                <span class="shrink-0 text-muted-foreground">{{ t('phone.colProxy') }}</span>
                <span class="truncate" :class="row.proxy_id > 0 && 'font-mono text-xs'">{{ proxyLabel(row.proxy_id) }}</span>
              </div>
              <div v-if="row.tags?.length" class="flex flex-wrap gap-1 pt-0.5">
                <Badge v-for="tg in row.tags" :key="tg.name" variant="outline" :class="tagClass(tg.color)">
                  <TruncatedText :text="tg.name" />
                </Badge>
              </div>
              <div class="flex items-center justify-between gap-2">
                <span class="shrink-0 text-muted-foreground">{{ t('phone.colRemark') }}</span>
                <TruncatedText :text="row.remark" :max="20" class="truncate text-right" />
              </div>
              <div class="flex items-center justify-between gap-2">
                <span class="text-muted-foreground">{{ t('table.createdAt') }}</span>
                <span class="tabular-nums">{{ formatDateTime(row.created_at) }}</span>
              </div>
            </CardContent>
            <CardFooter class="flex flex-wrap items-center gap-1.5 px-4">
              <Button
                v-if="showPowerOnBtn(row)"
                size="sm"
                variant="ghost"
                class="relative"
                :class="[powerOnBtnClass, isPowerOnLoading(row) && powerOnBtnLoadingClass]"
                :disabled="isPowerOnLoading(row) || opBusy"
                @click="powerOn(row)"
              >
                <span
                  class="inline-flex items-center gap-1.5"
                  :class="isPowerOnLoading(row) && 'invisible'"
                >
                  <PowerSwitchIcon class="size-4 shrink-0" />
                  {{ t('phone.op.powerOn') }}
                </span>
                <span
                  v-if="isPowerOnLoading(row)"
                  class="absolute inset-0 flex items-center justify-center"
                >
                  {{ t('phone.op.powerOnPending') }}
                </span>
              </Button>
              <Popconfirm
                v-if="canPowerOff(row.status)"
                :title="t('phone.op.powerOffConfirm', { name: row.name })"
                @confirm="powerOff(row)"
              >
                <Button
                  size="sm"
                  variant="destructive"
                  :disabled="opBusy"
                >
                  <PowerSwitchIcon class="size-4" /> {{ t('phone.op.powerOff') }}
                </Button>
              </Popconfirm>
              <Button
                size="icon"
                variant="outline"
                class="size-8"
                :disabled="!isRunning(row.status)"
                :title="t('phone.op.remoteControl')"
                @click="openRemoteControl(row)"
              >
                <Monitor class="size-4" />
              </Button>
              <Button
                v-if="row.adb_enabled"
                size="icon"
                variant="outline"
                class="size-8 border-green-500 text-green-600 hover:bg-green-50 hover:text-green-700 dark:text-green-400 dark:hover:bg-green-950"
                :title="t('phone.adb.markerTip')"
                @click="openAdb(row)"
              >
                <AndroidIcon class="size-4" />
              </Button>

              <DropdownMenu>
                <DropdownMenuTrigger as-child>
                  <Button variant="outline" size="icon-sm" class="ml-auto" :title="t('crud.more')">
                    <EllipsisVertical class="size-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" class="w-40">
                  <DropdownMenuItem @click="openEdit(row)">
                    <SquarePen class="size-4" /> {{ t('crud.edit') }}
                  </DropdownMenuItem>
                  <DropdownMenuItem @click="openTagDialog([row.id], row.tags ?? [])">
                    <Tag class="size-4" /> {{ t('phone.tag.title') }}
                  </DropdownMenuItem>
                  <DropdownMenuItem v-if="isRunning(row.status)" @click="openAppManager(row)">
                    <LayoutGrid class="size-4" /> {{ t('phone.app.title') }}
                  </DropdownMenuItem>
                  <DropdownMenuItem v-if="isRunning(row.status)" @click="openAdb(row)">
                    <Usb class="size-4" /> {{ t('phone.adb.title') }}
                  </DropdownMenuItem>
                  <DropdownMenuItem v-if="isRunning(row.status) && !row.rooted" @click="toggleRoot(row, true)">
                    <Shield class="size-4" /> {{ t('phone.root.enable') }}
                  </DropdownMenuItem>
                  <DropdownMenuItem v-if="isRunning(row.status) && row.rooted" @click="toggleRoot(row, false)">
                    <ShieldCheck class="size-4 text-emerald-600" /> {{ t('phone.root.disable') }}
                  </DropdownMenuItem>
                  <DropdownMenuItem v-if="isRunning(row.status)" @click="openScriptTest(row)">
                    <Terminal class="size-4" /> {{ t('phone.script.menu') }}
                  </DropdownMenuItem>
                  <DropdownMenuItem v-if="row.cp_id" @click="openRunLogs(row)">
                    <FileClock class="size-4" /> {{ t('phone.runLog.title') }}
                  </DropdownMenuItem>
                  <template v-if="canDestroy(row.status)">
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      class="text-destructive focus:text-destructive"
                      @click="ask(t('phone.op.destroy'), t('phone.op.destroyConfirm', { name: row.name }), () => destroyRow(row))"
                    >
                      <Trash class="size-4" /> {{ t('phone.op.destroy') }}
                    </DropdownMenuItem>
                  </template>
                </DropdownMenuContent>
              </DropdownMenu>
            </CardFooter>
          </Card>
        </div>
        </div>
      </div>
    </CardContent>

    <PhoneFormDialog :id="dialog.id" v-model="dialog.open" :mode="dialog.mode" :phones="data" @success="load" />
    <PhoneTagsDialog v-model="tagDialog.open" :ids="tagDialog.ids" :initial="tagDialog.initial" @success="refreshAfterTag" />
    <AppManagerDialog v-model="appDialog.open" :phone="appDialog.phone" />
    <AdbDrawer v-model="adbDrawer.open" :phone="adbDrawer.phone" @changed="load(true)" />
    <RunLogDialog v-model="runLogDialog.open" :phone="runLogDialog.phone" />
    <TaskCreateDialog v-model="scriptDialog.open" :preset-cp-id="scriptDialog.cpId" @created="onScriptCreated" />
    <TaskReportDialog v-model="scriptReport.open" :mid-task-id="scriptReport.midTaskId" />

    <!-- 危险操作统一确认弹框（根级，避免下拉里套气泡点不到） -->
    <AlertDialog v-model:open="confirmState.open">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ confirmState.title }}</AlertDialogTitle>
          <AlertDialogDescription>{{ confirmState.desc }}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{{ t('crud.cancel') }}</AlertDialogCancel>
          <AlertDialogAction :class="buttonVariants({ variant: 'destructive' })" @click="confirmState.run()">
            {{ t('crud.confirm') }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </Card>
  </div>
</template>
