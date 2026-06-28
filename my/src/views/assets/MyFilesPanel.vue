<script setup lang="ts">
import type {
  FileType,
  LibraryFile,
  LibraryFolder,
  LibraryOverview,
  LibraryTag,
} from '@/types/library'
import {
  Download,
  File as FileIcon,
  FileText,
  Folder,
  FolderInput,
  FolderPlus,
  Image as ImageIcon,
  Music,
  Package,
  Search,
  Trash2,
  Upload,
  Video,
} from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import libraryApi, { uploadLibraryFile } from '@/api/modules/library'
import Popconfirm from '@/components/Popconfirm.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { fmtBytes } from '@/utils/bytes'
import { formatDateTimeFull } from '@/utils/date'
import FileActionsMenu from '@/views/library/FileActionsMenu.vue'
import FolderTree from '@/views/library/FolderTree.vue'

const props = defineProps<{ overview: LibraryOverview | null }>()
const emit = defineEmits<{ changed: [] }>()

const { t } = useI18n()

// ---- 数据态 ----
const folders = ref<LibraryFolder[]>([])
const tags = ref<LibraryTag[]>([])
const files = ref<LibraryFile[]>([])
const total = ref(0)
const loading = ref(false)

// ---- 筛选/视图态 ----
const selectedFolder = ref(0) // 0=根
const fileTypeFilter = ref<FileType | ''>('')
const tagFilter = ref(0)
const keyword = ref('')
const page = ref(1)
const pageSize = 20

const selectedIds = ref<Set<number>>(new Set())

// 锁定态：超额（used > capacity）→ 禁用取用（下载/取文件）。
const locked = computed(() => props.overview?.usage.locked ?? false)

// ---- 文件类型图标/筛选项 ----
const FILE_TYPES: { value: FileType | '', label: string }[] = [
  { value: '', label: 'all' },
  { value: 'image', label: 'image' },
  { value: 'video', label: 'video' },
  { value: 'audio', label: 'audio' },
  { value: 'document', label: 'document' },
  { value: 'app', label: 'app' },
  { value: 'other', label: 'other' },
]
function typeIcon(ft: FileType) {
  switch (ft) {
    case 'image': return ImageIcon
    case 'video': return Video
    case 'audio': return Music
    case 'document': return FileText
    case 'app': return Package
    default: return FileIcon
  }
}

// ---- 加载 ----
async function loadFolders() {
  try {
    folders.value = (await libraryApi.listFolders()).data ?? []
  }
  catch {
    folders.value = []
  }
}
async function loadTags() {
  try {
    tags.value = (await libraryApi.listTags()).data ?? []
  }
  catch {
    tags.value = []
  }
}
async function loadFiles() {
  loading.value = true
  try {
    const res = await libraryApi.listFiles({
      folder_id: selectedFolder.value,
      file_type: fileTypeFilter.value || undefined,
      tag_id: tagFilter.value || undefined,
      keyword: keyword.value || undefined,
      page: page.value,
      size: pageSize,
    })
    files.value = res.data.list ?? []
    total.value = res.data.total ?? 0
    selectedIds.value = new Set()
  }
  catch {
    files.value = []
    total.value = 0
  }
  finally {
    loading.value = false
  }
}

function reloadFiles() {
  page.value = 1
  loadFiles()
}

onMounted(() => {
  loadFolders()
  loadTags()
  loadFiles()
})

// 筛选/翻页变化 → 重载。
function selectFolder(id: number) {
  selectedFolder.value = id
  reloadFiles()
}
function selectType(ft: FileType | '') {
  fileTypeFilter.value = ft
  reloadFiles()
}
function selectTag(id: number) {
  tagFilter.value = tagFilter.value === id ? 0 : id
  reloadFiles()
}

// ---- 多选 ----
function toggleSelect(id: number) {
  if (selectedIds.value.has(id)) selectedIds.value.delete(id)
  else selectedIds.value.add(id)
  selectedIds.value = new Set(selectedIds.value)
}
const hasSelection = computed(() => selectedIds.value.size > 0)

// ---- 上传 ----
const uploading = ref(false)
const uploadPct = ref(0)
const fileInput = ref<HTMLInputElement | null>(null)
const dragOver = ref(false)

function triggerPick() {
  if (locked.value) {
    toast.warning(t('library.lockedUploadHint'))
    return
  }
  fileInput.value?.click()
}

// 流式算 md5/slice_md5 → presign → 秒传命中则跳过 PUT/confirm，否则直传 S3 + confirm。
async function uploadOne(file: File): Promise<boolean> {
  const res = await uploadLibraryFile(file, {
    folderId: selectedFolder.value,
    onHashing: () => { uploadPct.value = 0 },
    onProgress: (pct) => { uploadPct.value = pct },
  })
  return res.instant
}

async function onFilesPicked(fileList: FileList | File[]) {
  const list = Array.from(fileList)
  if (!list.length) return
  uploading.value = true
  uploadPct.value = 0
  let okCount = 0
  let instantCount = 0
  try {
    for (const f of list) {
      try {
        if (await uploadOne(f)) instantCount++
        okCount++
      }
      catch {
        // 单文件失败已由拦截器提示，继续下一个
      }
    }
    if (okCount > 0) {
      if (instantCount > 0) toast.success(t('library.instantOk', { n: instantCount }))
      if (okCount > instantCount) toast.success(t('library.uploadOk', { n: okCount - instantCount }))
      emit('changed')
      reloadFiles()
    }
  }
  finally {
    uploading.value = false
    uploadPct.value = 0
    if (fileInput.value) fileInput.value.value = ''
  }
}

function onInputChange(e: Event) {
  const input = e.target as HTMLInputElement
  if (input.files) onFilesPicked(input.files)
}
function onDrop(e: DragEvent) {
  dragOver.value = false
  if (locked.value) {
    toast.warning(t('library.lockedUploadHint'))
    return
  }
  if (e.dataTransfer?.files) onFilesPicked(e.dataTransfer.files)
}

// ---- 文件操作 ----
async function download(f: LibraryFile) {
  if (locked.value) {
    toast.warning(t('library.lockedDownloadHint'))
    return
  }
  try {
    const res = await libraryApi.downloadUrl(f.id)
    window.open(res.data.url, '_blank')
  }
  catch {
    // 拦截器提示（含锁定 403）
  }
}

async function deleteFile(f: LibraryFile) {
  try {
    await libraryApi.deleteFile(f.id)
    toast.success(t('library.deleteOk'))
    emit('changed')
    loadFiles()
  }
  catch {
    // 拦截器
  }
}

async function deleteSelected() {
  const ids = Array.from(selectedIds.value)
  let ok = 0
  for (const id of ids) {
    try {
      await libraryApi.deleteFile(id)
      ok++
    }
    catch {
      // 继续
    }
  }
  if (ok > 0) {
    toast.success(t('library.deleteOk'))
    emit('changed')
    loadFiles()
  }
}

// ---- 文件重命名 / 移动 ----
const bulkMoveOpen = ref(false)
// 全部文件夹的全路径标签（供移动选择器）。
const folderTargets = computed(() => {
  const byId = new Map(folders.value.map(f => [f.id, f]))
  function pathOf(f: LibraryFolder): string {
    const parts = [f.name]
    let p = f.parent_id
    while (p) {
      const pf = byId.get(p)
      if (!pf) break
      parts.unshift(pf.name)
      p = pf.parent_id
    }
    return parts.join(' / ')
  }
  return folders.value
    .map(f => ({ id: f.id, label: pathOf(f) }))
    .sort((a, b) => a.label.localeCompare(b.label))
})
async function renameFileById(id: number, name: string) {
  try {
    await libraryApi.updateFile(id, { name })
    toast.success(t('library.renameOk'))
    loadFiles()
  }
  catch {
    // 拦截器
  }
}
async function moveFileById(id: number, folderId: number) {
  try {
    await libraryApi.updateFile(id, { folder_id: folderId })
    toast.success(t('library.moveOk'))
    loadFiles()
  }
  catch {
    // 拦截器
  }
}
async function moveSelected(folderId: number) {
  const ids = Array.from(selectedIds.value)
  let ok = 0
  for (const id of ids) {
    try {
      await libraryApi.updateFile(id, { folder_id: folderId })
      ok++
    }
    catch {
      // 继续
    }
  }
  bulkMoveOpen.value = false
  if (ok > 0) {
    toast.success(t('library.moveOk'))
    selectedIds.value = new Set()
    loadFiles()
  }
}

// ---- 新建文件夹（就地 popover 输入，替代浏览器原生 prompt）----
const folderPopoverOpen = ref(false)
const newFolderName = ref('')
async function submitNewFolder() {
  const name = newFolderName.value.trim()
  if (!name) return
  try {
    await libraryApi.createFolder(name, selectedFolder.value)
    toast.success(t('library.folder.createOk'))
    folderPopoverOpen.value = false
    newFolderName.value = ''
    loadFolders()
  }
  catch {
    // 拦截器
  }
}

async function renameFolderById(id: number, name: string) {
  try {
    await libraryApi.renameFolder(id, name)
    toast.success(t('library.folder.renameOk'))
    loadFolders()
  }
  catch {
    // 拦截器
  }
}
async function moveFolderById(id: number, parentId: number) {
  try {
    await libraryApi.moveFolder(id, parentId)
    toast.success(t('library.folder.moveOk'))
    loadFolders()
  }
  catch {
    // 拦截器
  }
}
async function deleteFolderById(id: number) {
  try {
    await libraryApi.deleteFolder(id)
    toast.success(t('library.folder.deleteOk'))
    if (selectedFolder.value === id) selectFolder(0)
    loadFolders()
  }
  catch {
    // 拦截器
  }
}

// 翻页。
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
function gotoPage(p: number) {
  if (p < 1 || p > totalPages.value) return
  page.value = p
  loadFiles()
}
</script>

<template>
  <div class="grid gap-4 lg:grid-cols-[220px_1fr]">
    <!-- 左侧：文件夹树 -->
    <aside class="bg-card flex flex-col gap-2 rounded-xl border p-3">
      <div class="flex items-center justify-between">
        <span class="text-muted-foreground text-xs font-medium">{{ t('library.folder.title') }}</span>
        <Popover v-model:open="folderPopoverOpen">
          <PopoverTrigger as-child>
            <button type="button" class="text-muted-foreground hover:text-foreground" :title="t('library.folder.new')">
              <FolderPlus class="size-4" />
            </button>
          </PopoverTrigger>
          <PopoverContent side="bottom" align="end" class="w-60 p-3">
            <form class="flex flex-col gap-2" @submit.prevent="submitNewFolder">
              <span class="text-muted-foreground text-xs font-medium">{{ t('library.folder.new') }}</span>
              <Input
                v-model="newFolderName"
                autofocus
                :placeholder="t('library.folder.newPrompt')"
                class="h-8"
              />
              <div class="flex justify-end gap-2">
                <Button type="button" variant="outline" size="sm" class="h-7 px-2.5" @click="folderPopoverOpen = false">
                  {{ t('crud.cancel') }}
                </Button>
                <Button type="submit" size="sm" class="h-7 px-2.5" :disabled="!newFolderName.trim()">
                  {{ t('crud.confirm') }}
                </Button>
              </div>
            </form>
          </PopoverContent>
        </Popover>
      </div>
      <FolderTree
        :folders="folders"
        :selected="selectedFolder"
        @update:selected="selectFolder"
        @rename="renameFolderById"
        @move="moveFolderById"
        @delete="deleteFolderById"
      />
    </aside>

    <!-- 右侧：文件区 -->
    <section class="bg-card flex flex-col gap-3 rounded-xl border p-4">
      <!-- 工具栏 -->
      <div class="flex flex-wrap items-center gap-2">
        <!-- 类型筛选 -->
        <div class="flex flex-wrap gap-1">
          <button
            v-for="ft in FILE_TYPES"
            :key="ft.value || 'all'"
            type="button"
            class="rounded-md px-2.5 py-1 text-xs transition-colors"
            :class="fileTypeFilter === ft.value ? 'bg-primary text-primary-foreground' : 'bg-muted/60 hover:bg-muted'"
            @click="selectType(ft.value)"
          >
            {{ t(`library.fileType.${ft.label}`) }}
          </button>
        </div>

        <div class="relative ml-auto">
          <Search class="text-muted-foreground absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2" />
          <Input
            v-model="keyword"
            class="h-8 w-44 pl-8 text-sm"
            :placeholder="t('library.searchPlaceholder')"
            @keyup.enter="reloadFiles"
          />
        </div>

        <Button size="sm" :disabled="uploading || locked" @click="triggerPick">
          <Upload class="size-4" />
          {{ uploading ? t('library.uploading', { pct: uploadPct }) : t('library.upload') }}
        </Button>
        <input ref="fileInput" type="file" multiple class="hidden" @change="onInputChange">
      </div>

      <!-- 标签筛选 -->
      <div v-if="tags.length" class="flex flex-wrap items-center gap-1.5">
        <span class="text-muted-foreground text-xs">{{ t('library.tags') }}:</span>
        <button
          v-for="tag in tags"
          :key="tag.id"
          type="button"
          class="rounded-full border px-2 py-0.5 text-xs transition-colors"
          :class="tagFilter === tag.id ? 'border-primary bg-primary/10 text-primary' : 'hover:bg-muted/60'"
          :style="tagFilter === tag.id ? {} : { borderColor: tag.color || undefined }"
          @click="selectTag(tag.id)"
        >
          {{ tag.name }}
        </button>
      </div>

      <!-- 多选操作条 -->
      <div v-if="hasSelection" class="bg-muted/40 flex items-center justify-between rounded-md px-3 py-2 text-sm">
        <span>{{ t('library.selectedCount', { n: selectedIds.size }) }}</span>
        <div class="flex items-center gap-2">
          <Popover v-model:open="bulkMoveOpen">
            <PopoverTrigger as-child>
              <Button size="sm" variant="outline">
                <FolderInput class="size-4" />
                {{ t('library.moveSelected') }}
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" class="w-56 p-1">
              <div class="text-muted-foreground px-1 pb-1 text-xs">
                {{ t('library.moveToHint') }}
              </div>
              <div class="max-h-56 overflow-y-auto">
                <button type="button" class="hover:bg-muted flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm" @click="moveSelected(0)">
                  <Folder class="size-3.5 shrink-0" /> {{ t('library.folder.root') }}
                </button>
                <button
                  v-for="tg in folderTargets"
                  :key="tg.id"
                  type="button"
                  class="hover:bg-muted flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm"
                  @click="moveSelected(tg.id)"
                >
                  <Folder class="size-3.5 shrink-0" /> <span class="truncate">{{ tg.label }}</span>
                </button>
              </div>
            </PopoverContent>
          </Popover>
          <Popconfirm :title="t('library.deleteSelectedConfirm')" tone="danger" @confirm="deleteSelected">
            <Button size="sm" variant="destructive">
              <Trash2 class="size-4" />
              {{ t('library.deleteSelected') }}
            </Button>
          </Popconfirm>
        </div>
      </div>

      <!-- 拖拽上传区 / 文件区 -->
      <div
        class="relative min-h-[280px] rounded-lg transition-colors"
        :class="dragOver ? 'bg-primary/5 ring-primary/40 ring-2' : ''"
        @dragover.prevent="dragOver = true"
        @dragleave.prevent="dragOver = false"
        @drop.prevent="onDrop"
      >
        <!-- 空态 -->
        <div v-if="!loading && !files.length" class="text-muted-foreground flex h-[280px] flex-col items-center justify-center gap-2 text-sm">
          <Upload class="size-8 opacity-40" />
          <span>{{ t('library.emptyHint') }}</span>
          <Button v-if="!locked" variant="outline" size="sm" @click="triggerPick">
            {{ t('library.upload') }}
          </Button>
        </div>

        <!-- 列表 -->
        <table v-else class="w-full text-sm">
          <thead>
            <tr class="text-muted-foreground border-b text-left text-xs">
              <th class="w-8 py-2" />
              <th class="py-2 font-medium">
                {{ t('library.colName') }}
              </th>
              <th class="py-2 font-medium">
                {{ t('library.colType') }}
              </th>
              <th class="py-2 font-medium">
                {{ t('library.colSize') }}
              </th>
              <th class="py-2 font-medium">
                {{ t('library.colUploaded') }}
              </th>
              <th class="py-2 text-right font-medium">
                {{ t('crud.actions') }}
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="f in files" :key="f.id" class="hover:bg-muted/30 border-b transition-colors">
              <td class="py-2">
                <input type="checkbox" :checked="selectedIds.has(f.id)" @change="toggleSelect(f.id)">
              </td>
              <td class="max-w-[280px] truncate py-2" :title="f.name">{{ f.name }}</td>
              <td class="py-2">
                <span class="inline-flex items-center gap-1">
                  <component :is="typeIcon(f.file_type)" class="text-muted-foreground size-3.5" />
                  {{ t(`library.fileType.${f.file_type}`) }}
                </span>
              </td>
              <td class="py-2 tabular-nums">{{ fmtBytes(f.size_bytes) }}</td>
              <td class="text-muted-foreground py-2 text-xs tabular-nums">{{ formatDateTimeFull(f.created_at) }}</td>
              <td class="py-2 text-right">
                <Button size="icon" variant="ghost" class="size-7" :disabled="locked" @click="download(f)">
                  <Download class="size-3.5" />
                </Button>
                <FileActionsMenu :file="f" :folders="folders" @rename="renameFileById" @move="moveFileById" />
                <Popconfirm :title="t('library.deleteConfirm')" tone="danger" @confirm="deleteFile(f)">
                  <Button size="icon" variant="ghost" class="text-destructive size-7">
                    <Trash2 class="size-3.5" />
                  </Button>
                </Popconfirm>
              </td>
            </tr>
          </tbody>
        </table>

        <!-- 拖拽提示遮罩 -->
        <div v-if="dragOver" class="text-primary pointer-events-none absolute inset-0 flex items-center justify-center rounded-lg text-sm font-medium">
          {{ t('library.dropHint') }}
        </div>
      </div>

      <!-- 分页 -->
      <div v-if="totalPages > 1" class="flex items-center justify-end gap-2 text-sm">
        <Button size="sm" variant="outline" :disabled="page <= 1" @click="gotoPage(page - 1)">
          {{ t('library.prevPage') }}
        </Button>
        <span class="text-muted-foreground tabular-nums">{{ page }} / {{ totalPages }}</span>
        <Button size="sm" variant="outline" :disabled="page >= totalPages" @click="gotoPage(page + 1)">
          {{ t('library.nextPage') }}
        </Button>
      </div>
    </section>
  </div>
</template>
