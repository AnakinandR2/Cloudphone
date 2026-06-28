<script setup lang="ts">
import type { AppRef, MarketApp, UserApp } from '@/types/app'
import type { InstalledApp } from '@/types/phone'
import { Check, Download, Loader2, Package, RefreshCw, Trash2 } from 'lucide-vue-next'
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import appApi from '@/api/modules/app'
import phoneApi from '@/api/modules/phone'
import Popconfirm from '@/components/Popconfirm.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { fmtBytes } from '@/utils/bytes'

// phoneIds=安装/卸载的目标机（单机=[id]，群控=全部）；masterId=读取「已安装」列表的主控机。
const props = defineProps<{ phoneIds: number[], masterId: number }>()

const { t } = useI18n()

const isGroup = computed(() => props.phoneIds.length > 1)

type Tab = 'mine' | 'market' | 'installed'
const tab = ref<Tab>('mine')

const mine = ref<UserApp[]>([])
const market = ref<MarketApp[]>([])
const installed = ref<InstalledApp[]>([])
const loading = ref(false)
const search = ref('')
// 正在安装的 row.key（source-id）/ 正在卸载的 packageName，用于按钮 loading 与禁用。
const installing = ref<Set<string>>(new Set())
const uninstalling = ref<Set<string>>(new Set())

// 主控已安装应用的包名集合，用于在「我的应用 / 应用市场」标记「已安装」。
const installedPkgs = computed(() => new Set(installed.value.map(a => a.packageName)))

interface Row {
  key: string
  source: 'user' | 'market'
  refId: number
  appName: string
  packageName: string
  version: string
  fileSize: string
  iconPath: string
  parsing: boolean
  failed: boolean
  installed: boolean
}

function userRow(a: UserApp): Row {
  return {
    key: `user-${a.file_id}`,
    source: 'user',
    refId: a.file_id,
    appName: a.app_name,
    packageName: a.package_name,
    version: a.version,
    fileSize: a.size_bytes ? fmtBytes(a.size_bytes) : '',
    iconPath: a.icon_url,
    parsing: a.parse_status === 'parsing',
    failed: a.parse_status === 'failed',
    installed: installedPkgs.value.has(a.package_name),
  }
}
function marketRow(a: MarketApp): Row {
  return {
    key: `market-${a.id}`,
    source: 'market',
    refId: a.id,
    appName: a.app_name,
    packageName: a.package_name,
    version: a.version,
    fileSize: a.size_bytes ? fmtBytes(a.size_bytes) : '',
    iconPath: a.icon_url,
    parsing: a.parse_status === 'parsing',
    failed: a.parse_status === 'failed',
    installed: installedPkgs.value.has(a.package_name),
  }
}

const rows = computed<Row[]>(() => {
  const list = tab.value === 'market' ? market.value.map(marketRow) : mine.value.map(userRow)
  const kw = search.value.trim().toLowerCase()
  if (!kw)
    return list
  return list.filter(r => r.appName.toLowerCase().includes(kw) || r.packageName.toLowerCase().includes(kw))
})

const installedRows = computed(() => {
  const kw = search.value.trim().toLowerCase()
  if (!kw)
    return installed.value
  return installed.value.filter(a =>
    (a.appName || '').toLowerCase().includes(kw) || a.packageName.toLowerCase().includes(kw),
  )
})

// 拉取当前 tab 的列表数据。
async function load() {
  loading.value = true
  try {
    if (tab.value === 'mine')
      mine.value = (await appApi.userList()).data ?? []
    else if (tab.value === 'market')
      market.value = (await appApi.market()).data ?? []
    else
      installed.value = (await phoneApi.apps(props.masterId)).data ?? []
  }
  catch {
    toast.error(t('phone.rc.appLoadFail'))
  }
  finally {
    loading.value = false
  }
}

// 静默刷新主控已安装列表（供标记 + 安装/卸载后同步），不切 loading。
async function refreshInstalled() {
  if (!props.masterId)
    return
  try {
    installed.value = (await phoneApi.apps(props.masterId)).data ?? []
  }
  catch { /* 标记失效不打扰 */ }
}

watch(tab, load)
// 群控切换主控机时，已安装列表跟随主控刷新。
watch(() => props.masterId, refreshInstalled)

// 把操作广播到全部目标机（群控乐观执行：存在则生效，不存在则中台跳过）。
async function broadcast(fn: (pid: number) => Promise<unknown>) {
  const results = await Promise.allSettled(props.phoneIds.map(fn))
  const ok = results.filter(r => r.status === 'fulfilled').length
  return { ok, total: props.phoneIds.length }
}

function resultDesc(ok: number, total: number, name: string) {
  return isGroup.value ? t('phone.rc.appGroupResult', { ok, total }) : name
}

async function install(row: Row) {
  // 仅就绪应用可安装。
  if (row.parsing || row.failed || installing.value.has(row.key))
    return
  installing.value = new Set(installing.value).add(row.key)
  try {
    const refs: AppRef[] = [{ source: row.source, id: row.refId }]
    // 按 URL 安装：一次性下发到全部目标机（中台异步）。
    const { data } = await phoneApi.installByUrl(props.phoneIds, refs)
    const n = data?.task_info_list?.length ?? 0
    if (n > 0)
      toast.success(t('phone.rc.appDispatched'), { description: resultDesc(n, props.phoneIds.length, row.appName) })
    else
      toast.error(t('phone.rc.appInstallFail'))
    await refreshInstalled()
  }
  catch {
    toast.error(t('phone.rc.appInstallFail'))
  }
  finally {
    const next = new Set(installing.value)
    next.delete(row.key)
    installing.value = next
  }
}

async function uninstall(app: InstalledApp) {
  const pkg = app.packageName
  if (!pkg || uninstalling.value.has(pkg))
    return
  uninstalling.value = new Set(uninstalling.value).add(pkg)
  try {
    // 中台按 appId 卸载（packageNames 会被拒：「应用id列表不能为空」）；appId 是租户全局，跨机一致。
    const payload = app.id ? { appIds: [app.id] } : { packageNames: [pkg] }
    const { ok, total } = await broadcast(pid => phoneApi.uninstallApp(pid, payload))
    if (ok > 0)
      toast.success(t('phone.rc.appUninstallOk'), { description: resultDesc(ok, total, app.appName || pkg) })
    else
      toast.error(t('phone.rc.appUninstallFail'))
    await refreshInstalled()
  }
  finally {
    const next = new Set(uninstalling.value)
    next.delete(pkg)
    uninstalling.value = next
  }
}

onMounted(() => {
  refreshInstalled()
  load()
})
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <!-- tab + 刷新 -->
    <div class="flex shrink-0 items-center gap-1.5 border-b p-1.5">
      <Tabs v-model="tab" class="min-w-0 flex-1">
        <TabsList class="grid w-full grid-cols-3">
          <TabsTrigger value="mine" class="text-xs">
            {{ t('phone.rc.appMine') }}
          </TabsTrigger>
          <TabsTrigger value="market" class="text-xs">
            {{ t('phone.rc.appMarket') }}
          </TabsTrigger>
          <TabsTrigger value="installed" class="text-xs">
            {{ t('phone.rc.appInstalledTab') }}
          </TabsTrigger>
        </TabsList>
      </Tabs>
      <Button variant="ghost" size="icon" class="size-8 shrink-0" :disabled="loading" @click="load">
        <RefreshCw class="size-3.5" :class="loading && 'animate-spin'" />
      </Button>
    </div>

    <!-- 群控提示：操作广播到全部机器 -->
    <div v-if="isGroup" class="shrink-0 border-b bg-muted/40 px-2.5 py-1 text-[11px] text-muted-foreground">
      {{ t('phone.rc.appGroupHint', { n: phoneIds.length }) }}
    </div>

    <!-- 搜索 -->
    <div class="shrink-0 border-b p-1.5">
      <input
        v-model="search"
        :placeholder="t('phone.rc.appSearch')"
        class="h-7 w-full rounded-md border bg-transparent px-2 text-xs outline-none focus:ring-1 focus:ring-ring"
      >
    </div>

    <!-- 列表区 -->
    <div class="min-h-0 flex-1 overflow-y-auto">
      <div v-if="loading" class="flex items-center justify-center gap-2 p-6 text-xs text-muted-foreground">
        <Loader2 class="size-4 animate-spin" /> {{ t('common.loading', '加载中…') }}
      </div>

      <!-- 已安装应用：可卸载 -->
      <template v-else-if="tab === 'installed'">
        <div v-if="installedRows.length === 0" class="p-6 text-center text-xs text-muted-foreground">
          {{ t('phone.rc.appEmpty') }}
        </div>
        <ul v-else class="divide-y">
          <li v-for="app in installedRows" :key="app.packageName" class="flex items-center gap-2.5 p-2.5">
            <div class="flex size-9 shrink-0 items-center justify-center overflow-hidden rounded bg-muted/40">
              <img
                v-if="app.iconPath"
                :src="app.iconPath"
                :alt="app.appName"
                class="size-full object-cover"
                @error="(e) => ((e.target as HTMLImageElement).style.display = 'none')"
              >
              <Package v-else class="size-4 text-muted-foreground" />
            </div>
            <div class="min-w-0 flex-1">
              <p class="truncate text-xs font-medium">
                {{ app.appName || app.packageName }}
              </p>
              <p class="truncate text-[11px] text-muted-foreground">
                {{ app.packageName }}
              </p>
              <p class="text-[11px] text-muted-foreground tabular-nums">
                {{ app.version || '-' }}<span v-if="app.fileSize"> · {{ app.fileSize }}</span>
              </p>
            </div>
            <Popconfirm
              tone="warning"
              :title="isGroup ? t('phone.rc.appUninstallConfirmGroup', { name: app.appName || app.packageName, n: phoneIds.length }) : t('phone.rc.appUninstallConfirm', { name: app.appName || app.packageName })"
              @confirm="uninstall(app)"
            >
              <Button
                variant="outline"
                size="sm"
                class="h-7 shrink-0 gap-1 px-2 text-xs"
                :disabled="uninstalling.has(app.packageName)"
              >
                <Loader2 v-if="uninstalling.has(app.packageName)" class="size-3.5 animate-spin" />
                <Trash2 v-else class="size-3.5" />
                {{ t('phone.rc.appUninstall') }}
              </Button>
            </Popconfirm>
          </li>
        </ul>
      </template>

      <!-- 我的应用 / 应用市场：可安装，已安装标记 -->
      <template v-else>
        <div v-if="rows.length === 0" class="p-6 text-center text-xs text-muted-foreground">
          {{ t('phone.rc.appEmpty') }}
        </div>
        <ul v-else class="divide-y">
          <li v-for="row in rows" :key="row.key" class="flex items-center gap-2.5 p-2.5">
            <div class="flex size-9 shrink-0 items-center justify-center overflow-hidden rounded bg-muted/40">
              <img
                v-if="row.iconPath"
                :src="row.iconPath"
                :alt="row.appName"
                class="size-full object-cover"
                @error="(e) => ((e.target as HTMLImageElement).style.display = 'none')"
              >
              <Package v-else class="size-4 text-muted-foreground" />
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-1.5">
                <p class="truncate text-xs font-medium">
                  {{ row.appName || row.packageName }}
                </p>
                <Badge
                  v-if="row.installed"
                  variant="outline"
                  class="h-4 shrink-0 gap-0.5 border-green-500 px-1 text-[10px] text-green-600 dark:text-green-400"
                >
                  <Check class="size-2.5" /> {{ t('phone.rc.appInstalled') }}
                </Badge>
              </div>
              <p class="truncate text-[11px] text-muted-foreground">
                {{ row.packageName || '-' }}
              </p>
              <p class="text-[11px] text-muted-foreground tabular-nums">
                {{ row.version || '-' }}<span v-if="row.fileSize"> · {{ row.fileSize }}</span>
              </p>
            </div>
            <Button
              variant="outline"
              size="sm"
              class="h-7 shrink-0 gap-1 px-2 text-xs"
              :class="row.failed ? 'border-red-500 text-red-600 dark:text-red-400' : ''"
              :disabled="row.parsing || row.failed || installing.has(row.key)"
              @click="install(row)"
            >
              <Loader2 v-if="installing.has(row.key)" class="size-3.5 animate-spin" />
              <Download v-else class="size-3.5" />
              {{ row.parsing ? t('app.statusParsing') : row.failed ? t('app.statusFailed') : (row.installed ? t('phone.rc.appReinstall') : t('phone.rc.appInstall')) }}
            </Button>
          </li>
        </ul>
      </template>
    </div>
  </div>
</template>
