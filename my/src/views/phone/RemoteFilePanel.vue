<script setup lang="ts">
import type { PhoneFile } from '@/types/phone'
import { ArrowRight, ArrowUp, Download, File as FileIcon, Folder, Loader2, RefreshCw, Trash } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import phoneApi from '@/api/modules/phone'
import Popconfirm from '@/components/Popconfirm.vue'
import { Button } from '@/components/ui/button'

const props = defineProps<{ phoneId: number, phones?: { id: number, name: string }[] }>()

const { t } = useI18n()

const ROOT = '/sdcard'
const currentPath = ref(ROOT)
const files = ref<PhoneFile[]>([])
const loading = ref(false)
const downloading = ref(false)
const deleting = ref(false)
const selected = ref<Set<string>>(new Set())

// 群控：可选删除目标范围（主控 / 所有）；单机时无此概念。
const isGroup = computed(() => (props.phones?.length ?? 0) > 1)
const scope = ref<'master' | 'all'>('master')
const myName = computed(() => props.phones?.find(p => p.id === props.phoneId)?.name ?? '')
const scopeTargets = computed(() =>
  scope.value === 'all' && props.phones?.length
    ? props.phones
    : [{ id: props.phoneId, name: myName.value }],
)

// 目录在前，再按名称排序。
const sorted = computed(() =>
  [...files.value].sort((a, b) => {
    const ad = a.type === 'directory' ? 0 : 1
    const bd = b.type === 'directory' ? 0 : 1
    return ad - bd || a.name.localeCompare(b.name)
  }),
)

const fileItems = computed(() => sorted.value.filter(f => f.type === 'file'))
const allSelected = computed(() => fileItems.value.length > 0 && fileItems.value.every(f => selected.value.has(f.path)))
const selectedCount = computed(() => selected.value.size)

function isAtRoot() {
  return currentPath.value === '/' || currentPath.value === ROOT
}

async function load() {
  loading.value = true
  selected.value = new Set()
  try {
    const res = await phoneApi.fileList(props.phoneId, currentPath.value)
    files.value = res.data ?? []
  }
  catch {
    files.value = []
    toast.error(t('phone.rc.fileLoadFail'))
  }
  finally {
    loading.value = false
  }
}

function enter(f: PhoneFile) {
  if (f.type !== 'directory')
    return
  currentPath.value = f.path
  load()
}

function goUp() {
  if (isAtRoot())
    return
  const parent = currentPath.value.replace(/\/+(?:[^/]+\/*)?$/, '') || '/'
  currentPath.value = parent
  load()
}

function toggleOne(f: PhoneFile, checked: boolean) {
  const next = new Set(selected.value)
  if (checked)
    next.add(f.path)
  else next.delete(f.path)
  selected.value = next
}

function toggleAll(checked: boolean) {
  selected.value = checked ? new Set(fileItems.value.map(f => f.path)) : new Set()
}

async function downloadOne(f: PhoneFile) {
  const blob = await phoneApi.fileDownload(props.phoneId, f.path)
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = f.name
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

async function onDownloadOne(f: PhoneFile) {
  try {
    await downloadOne(f)
  }
  catch {
    toast.error(t('phone.rc.fileDownloadFail'), { description: f.name })
  }
}

async function onDownloadSelected() {
  const targets = fileItems.value.filter(f => selected.value.has(f.path))
  if (!targets.length)
    return
  downloading.value = true
  let fail = 0
  for (const f of targets) {
    try {
      await downloadOne(f)
    }
    catch {
      fail++
    }
  }
  downloading.value = false
  if (fail)
    toast.error(t('phone.rc.fileDownloadFail'), { description: `${fail}/${targets.length}` })
  else
    toast.success(t('phone.rc.fileDownloadOk'))
}

async function onDeleteSelected() {
  const paths = [...selected.value]
  if (!paths.length)
    return
  const tgts = scopeTargets.value
  deleting.value = true
  try {
    if (tgts.length > 1) {
      const rs = await Promise.allSettled(tgts.map(tg => phoneApi.fileDelete(tg.id, paths)))
      const fail = rs.filter(r => r.status === 'rejected').length
      if (fail)
        toast.error(t('phone.rc.fileDeleteFail'), { description: `${fail}/${tgts.length}` })
      else
        toast.success(t('phone.rc.fileDeleteOk'))
    }
    else {
      await phoneApi.fileDelete(tgts[0].id, paths)
      toast.success(t('phone.rc.fileDeleteOk'))
    }
    await load()
  }
  catch {
    toast.error(t('phone.rc.fileDeleteFail'))
  }
  finally {
    deleting.value = false
  }
}

function fmtSize(n: number) {
  if (n < 1024)
    return `${n} B`
  if (n < 1024 ** 2)
    return `${(n / 1024).toFixed(1)} KB`
  if (n < 1024 ** 3)
    return `${(n / 1024 / 1024).toFixed(1)} MB`
  return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`
}

onMounted(load)
</script>

<template>
  <div class="relative flex h-full min-h-0 flex-col">
    <!-- 路径 + 工具栏 -->
    <div class="flex items-center gap-1 border-b px-2 py-1.5">
      <Button variant="ghost" size="icon" class="size-7" :disabled="isAtRoot()" :title="t('phone.rc.fileUp')" @click="goUp">
        <ArrowUp class="size-4" />
      </Button>
      <Button variant="ghost" size="icon" class="size-7" :title="t('phone.rc.fileRefresh')" @click="load">
        <RefreshCw class="size-4" :class="loading ? 'animate-spin' : ''" />
      </Button>
      <input
        v-model="currentPath"
        class="min-w-0 flex-1 rounded border bg-transparent px-1.5 py-1 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
        spellcheck="false"
        :title="currentPath"
        @keyup.enter="load"
      >
      <Button
        variant="outline"
        size="sm"
        class="h-7 gap-1 px-2 text-xs"
        @click="load"
      >
        <ArrowRight class="size-3.5" />
        {{ t('phone.rc.fileGo') }}
      </Button>
    </div>

    <!-- 群控范围：主控 / 群控 切换 + 说明（同一行） -->
    <div v-if="isGroup" class="flex items-center gap-2 border-b px-2 py-1 text-[11px] text-muted-foreground">
      <div class="flex shrink-0 items-center rounded-md border p-0.5">
        <button type="button" class="rounded px-1.5 py-0.5" :class="scope === 'master' ? 'bg-muted font-medium text-foreground' : ''" @click="scope = 'master'">
          {{ t('phone.rc.scopeMaster') }}
        </button>
        <button type="button" class="rounded px-1.5 py-0.5" :class="scope === 'all' ? 'bg-muted font-medium text-foreground' : ''" @click="scope = 'all'">
          {{ t('phone.rc.scopeGroup', { n: phones?.length ?? 0 }) }}
        </button>
      </div>
      <span class="truncate">{{ scope === 'master' ? t('phone.rc.fileScopeMasterHint') : t('phone.rc.fileScopeAllHint') }}</span>
    </div>

    <!-- 多选操作条 -->
    <div class="flex items-center gap-2 border-b px-2 py-1.5">
      <label class="flex cursor-pointer items-center gap-1.5 text-xs">
        <input
          type="checkbox"
          class="size-3.5 accent-primary"
          :checked="allSelected"
          :disabled="!fileItems.length"
          @change="toggleAll(($event.target as HTMLInputElement).checked)"
        >
        {{ t('phone.rc.fileSelectAll') }}
      </label>
      <Button
        size="sm"
        class="ml-auto h-7 gap-1 px-2 text-xs"
        :disabled="!selectedCount || downloading"
        @click="onDownloadSelected"
      >
        <Loader2 v-if="downloading" class="size-3.5 animate-spin" />
        <Download v-else class="size-3.5" />
        {{ t('phone.rc.fileDownloadSel') }}<span v-if="selectedCount">（{{ selectedCount }}）</span>
      </Button>

      <Popconfirm tone="danger" :title="t('phone.rc.fileDeleteConfirm', { n: selectedCount })" @confirm="onDeleteSelected">
        <Button
          size="sm"
          variant="outline"
          class="h-7 gap-1 px-2 text-xs text-destructive hover:text-destructive"
          :disabled="!selectedCount || deleting"
        >
          <Loader2 v-if="deleting" class="size-3.5 animate-spin" />
          <Trash v-else class="size-3.5" />
          {{ t('phone.rc.fileDeleteSel') }}<span v-if="selectedCount">（{{ selectedCount }}）</span>
        </Button>
      </Popconfirm>
    </div>

    <!-- 文件列表 -->
    <div class="min-h-0 flex-1 overflow-y-auto">
      <div v-if="loading" class="flex h-full items-center justify-center text-muted-foreground">
        <Loader2 class="size-5 animate-spin" />
      </div>
      <template v-else>
        <div v-if="!sorted.length" class="px-3 py-6 text-center text-xs text-muted-foreground">
          {{ t('phone.rc.fileEmpty') }}
        </div>
        <ul v-else class="divide-y">
          <li
            v-for="f in sorted"
            :key="f.path"
            class="flex items-center gap-2 px-2 py-1.5 text-sm hover:bg-muted/50"
          >
            <input
              v-if="f.type === 'file'"
              type="checkbox"
              class="size-3.5 shrink-0 accent-primary"
              :checked="selected.has(f.path)"
              @change="toggleOne(f, ($event.target as HTMLInputElement).checked)"
            >
            <span v-else class="inline-block size-3.5 shrink-0" />

            <component :is="f.type === 'directory' ? Folder : FileIcon" class="size-4 shrink-0" :class="f.type === 'directory' ? 'text-amber-500' : 'text-muted-foreground'" />

            <button
              type="button"
              class="min-w-0 flex-1 truncate text-left"
              :class="f.type === 'directory' ? 'cursor-pointer hover:underline' : 'cursor-default'"
              :title="f.name"
              @click="enter(f)"
            >
              {{ f.name }}
            </button>

            <span class="hidden shrink-0 text-[11px] text-muted-foreground sm:inline">{{ f.type === 'file' ? fmtSize(f.size) : '' }}</span>

            <Button
              v-if="f.type === 'file'"
              variant="ghost"
              size="icon"
              class="size-7 shrink-0"
              :title="t('phone.rc.fileDownload')"
              @click="onDownloadOne(f)"
            >
              <Download class="size-4" />
            </Button>
          </li>
        </ul>
      </template>
    </div>
  </div>
</template>
