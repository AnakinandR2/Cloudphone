<script setup lang="ts">
import type { PhoneFile } from '@/types/phone'
import { ArrowUp, Download, File as FileIcon, Folder, Images, Loader2, RefreshCw, Trash, Upload, X } from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import phoneApi from '@/api/modules/phone'
import Popconfirm from '@/components/Popconfirm.vue'
import { Button } from '@/components/ui/button'

const props = defineProps<{ phoneId: number, phones?: { id: number, name: string }[] }>()

const { t } = useI18n()

const ROOT = '/sdcard'
const MAX_UPLOAD = 10 // 中台 batch-upload 一次限 1-10 个文件
const currentPath = ref(ROOT)
const files = ref<PhoneFile[]>([])
const loading = ref(false)
const downloading = ref(false)
const deleting = ref(false)
const uploading = ref(false)
const dragging = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const selected = ref<Set<string>>(new Set())

// 群控：可选上传/删除/操作目标范围（主控 / 所有）；单机时无此概念。
const isGroup = computed(() => (props.phones?.length ?? 0) > 1)
const scope = ref<'master' | 'all'>('master')
const myName = computed(() => props.phones?.find(p => p.id === props.phoneId)?.name ?? '')
const scopeTargets = computed(() =>
  scope.value === 'all' && props.phones?.length
    ? props.phones
    : [{ id: props.phoneId, name: myName.value }],
)

// 多台上传时逐台进度（乐观显示）。
interface UploadJob { id: number, name: string, progress: number, status: 'uploading' | 'done' | 'error' }
const jobs = ref<UploadJob[]>([])
function clearJobs() {
  jobs.value = []
}

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

// 上传：把选中的本地文件传到当前目录，成功后刷新列表。
async function uploadFiles(list: File[]) {
  if (!list.length || uploading.value)
    return
  if (list.length > MAX_UPLOAD) {
    toast.error(t('phone.rc.fileUploadLimit', { n: MAX_UPLOAD }))
    return
  }
  const tgts = scopeTargets.value
  uploading.value = true
  // 多台（群控「所有」）：乐观并发，逐台显示进度。
  if (tgts.length > 1) {
    jobs.value = tgts.map(tg => ({ id: tg.id, name: tg.name, progress: 0, status: 'uploading' as const }))
    await Promise.all(tgts.map(async (tg) => {
      const job = jobs.value.find(j => j.id === tg.id)
      try {
        await phoneApi.fileUpload(tg.id, currentPath.value, list, (pct) => {
          if (job)
            job.progress = pct
        })
        if (job) {
          job.progress = 100
          job.status = 'done'
        }
      }
      catch {
        if (job)
          job.status = 'error'
      }
    }))
    uploading.value = false
    await load()
    return
  }
  // 单台（主控）
  try {
    await phoneApi.fileUpload(tgts[0].id, currentPath.value, list)
    // 中台已知 bug：folderPath 被忽略，文件始终落在 /sdcard/Download，提示用户实际落点。
    toast.success(t('phone.rc.fileUploadOk'), { description: t('phone.rc.fileUploadHint') })
    await load()
  }
  catch {
    toast.error(t('phone.rc.fileUploadFail'))
  }
  finally {
    uploading.value = false
  }
}

function pickUpload() {
  fileInput.value?.click()
}

// 素材中心：功能开发中，仅提示。
function openMaterial() {
  toast.info(t('phone.rc.comingSoon'))
}

function onPicked(e: Event) {
  const input = e.target as HTMLInputElement
  const list = input.files ? Array.from(input.files) : []
  input.value = '' // 允许再次选择同一文件
  uploadFiles(list)
}

function onDrop(e: DragEvent) {
  dragging.value = false
  const list = e.dataTransfer?.files ? Array.from(e.dataTransfer.files) : []
  uploadFiles(list)
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
  <div
    class="relative flex h-full min-h-0 flex-col"
    @dragover.prevent="dragging = true"
    @dragenter.prevent="dragging = true"
    @dragleave.prevent="dragging = false"
    @drop.prevent="onDrop"
  >
    <!-- 拖拽上传覆盖层 -->
    <div
      v-if="dragging"
      class="pointer-events-none absolute inset-0 z-10 m-1 flex items-center justify-center rounded border-2 border-dashed border-primary bg-primary/10 text-xs font-medium text-primary"
    >
      {{ t('phone.rc.fileDropHint') }}
    </div>

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
        :disabled="uploading"
        :title="t('phone.rc.fileUploadTip')"
        @click="pickUpload"
      >
        <Loader2 v-if="uploading" class="size-3.5 animate-spin" />
        <Upload v-else class="size-3.5" />
        {{ t('phone.rc.fileUploadBtn') }}
      </Button>
      <Button
        variant="ghost"
        size="sm"
        class="h-7 gap-1 px-2 text-xs"
        :title="t('phone.rc.fileMaterial')"
        @click="openMaterial"
      >
        <Images class="size-3.5" />
        {{ t('phone.rc.fileMaterial') }}
      </Button>
      <input ref="fileInput" type="file" multiple class="hidden" @change="onPicked">
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

    <!-- 多台上传进度（群控「所有」） -->
    <div v-if="jobs.length" class="border-b px-2 py-1.5 text-xs">
      <div class="mb-1 flex items-center justify-between">
        <span class="font-medium">{{ t('phone.rc.fileUploadProgress') }}</span>
        <button type="button" class="text-muted-foreground hover:text-foreground" @click="clearJobs">
          <X class="size-3.5" />
        </button>
      </div>
      <div class="max-h-28 space-y-1 overflow-y-auto">
        <div v-for="j in jobs" :key="j.id" class="flex items-center gap-2">
          <span class="w-16 shrink-0 truncate">{{ j.name }}</span>
          <div class="h-1.5 flex-1 overflow-hidden rounded bg-muted">
            <div class="h-full transition-all" :class="j.status === 'error' ? 'bg-destructive' : 'bg-primary'" :style="{ width: `${j.progress}%` }" />
          </div>
          <span class="w-10 shrink-0 text-right tabular-nums" :class="j.status === 'error' ? 'text-destructive' : 'text-muted-foreground'">
            {{ j.status === 'error' ? t('phone.rc.fileUploadFailShort') : `${j.progress}%` }}
          </span>
        </div>
      </div>
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
        <!-- 底部拖拽上传区（含空目录），点击亦可选文件 -->
        <button
          type="button"
          class="m-2 flex w-[calc(100%-1rem)] items-center justify-center gap-2 rounded border border-dashed px-3 py-4 text-xs text-muted-foreground transition-colors hover:border-primary hover:text-primary disabled:opacity-50"
          :disabled="uploading"
          @click="pickUpload"
        >
          <Upload class="size-4" />
          {{ t('phone.rc.fileDropArea') }}
        </button>
      </template>
    </div>
  </div>
</template>
