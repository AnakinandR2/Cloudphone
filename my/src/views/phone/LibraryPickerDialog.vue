<script setup lang="ts">
import type { FileType, LibraryFile, LibraryFolder } from '@/types/library'
import {
  File as FileIcon,
  FileText,
  Image as ImageIcon,
  Music,
  Package,
  Search,
  Video,
} from 'lucide-vue-next'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import libraryApi from '@/api/modules/library'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import FolderTree from '@/views/library/FolderTree.vue'
import { fmtBytes } from '@/utils/bytes'
import { formatDateTimeFull } from '@/utils/date'

// 受控弹层：父组件传 open + 目标台数；确认回传选中的 file_ids。
const props = defineProps<{ open: boolean, targetCount: number }>()
const emit = defineEmits<{
  'update:open': [open: boolean]
  'confirm': [fileIds: number[]]
}>()

const { t } = useI18n()

// 与本地上传一致：一次最多 10 个文件。
const MAX_PICK = 10

// ---- 浏览态 ----
const folders = ref<LibraryFolder[]>([])
const files = ref<LibraryFile[]>([])
const loading = ref(false)
const selectedFolder = ref(0) // 0=根
const fileTypeFilter = ref<FileType | ''>('')
const keyword = ref('')

// ---- 选择态 ----
const selectedIds = ref<number[]>([])

// 文件类型筛选项（与素材库一致）。
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

async function loadFolders() {
  try {
    folders.value = (await libraryApi.listFolders()).data ?? []
  }
  catch {
    folders.value = []
  }
}

async function loadFiles() {
  loading.value = true
  try {
    const res = await libraryApi.listFiles({
      folder_id: selectedFolder.value,
      file_type: fileTypeFilter.value || undefined,
      keyword: keyword.value || undefined,
      page: 1,
      size: 200,
    })
    files.value = res.data.list ?? []
  }
  catch {
    files.value = []
  }
  finally {
    loading.value = false
  }
}

// 打开时初始化；关闭时清空选择避免残留。
watch(
  () => props.open,
  (open) => {
    if (open) {
      selectedIds.value = []
      keyword.value = ''
      loadFolders()
      loadFiles()
    }
  },
)

function selectFolder(id: number) {
  selectedFolder.value = id
  loadFiles()
}
function selectType(ft: FileType | '') {
  fileTypeFilter.value = ft
  loadFiles()
}

const selectedCount = computed(() => selectedIds.value.length)

function isPicked(id: number) {
  return selectedIds.value.includes(id)
}

// 勾选：超过上限禁选并提示。
function togglePick(id: number, checked: boolean) {
  if (checked) {
    if (selectedIds.value.includes(id))
      return
    if (selectedIds.value.length >= MAX_PICK) {
      toast.warning(t('phone.rc.pickLimit', { n: MAX_PICK }))
      return
    }
    selectedIds.value = [...selectedIds.value, id]
  }
  else {
    selectedIds.value = selectedIds.value.filter(x => x !== id)
  }
}

function confirm() {
  if (!selectedIds.value.length)
    return
  emit('confirm', [...selectedIds.value])
  emit('update:open', false)
}
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="flex h-[88vh] max-h-[88vh] flex-col gap-3 sm:max-w-5xl">
      <DialogHeader>
        <DialogTitle>{{ t('phone.rc.pickerTitle') }}</DialogTitle>
      </DialogHeader>

      <!-- 工具栏：类型筛选 + 搜索 -->
      <div class="flex flex-wrap items-center gap-2">
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
            :placeholder="t('phone.rc.pickerSearch')"
            @keyup.enter="loadFiles"
          />
        </div>
      </div>

      <div class="grid min-h-0 flex-1 gap-3 sm:grid-cols-[200px_1fr]">
        <!-- 文件夹树 -->
        <aside class="bg-card hidden min-h-0 overflow-y-auto rounded-md border p-2 sm:block">
          <FolderTree
            :folders="folders"
            :selected="selectedFolder"
            @update:selected="selectFolder"
          />
        </aside>

        <!-- 文件表格 -->
        <section class="min-h-0 overflow-y-auto rounded-md border">
          <!-- 空态 -->
          <div
            v-if="!loading && !files.length"
            class="text-muted-foreground flex h-48 items-center justify-center text-sm"
          >
            {{ t('phone.rc.pickerEmpty') }}
          </div>

          <table v-else class="w-full text-sm">
            <thead>
              <tr class="text-muted-foreground bg-muted/40 border-b text-left text-xs">
                <th class="w-8 px-2 py-2" />
                <th class="px-2 py-2 font-medium">{{ t('library.colName') }}</th>
                <th class="px-2 py-2 font-medium">{{ t('library.colType') }}</th>
                <th class="px-2 py-2 font-medium">{{ t('library.colSize') }}</th>
                <th class="px-2 py-2 font-medium">{{ t('library.colUploaded') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="f in files"
                :key="f.id"
                class="hover:bg-muted/30 cursor-pointer border-b transition-colors"
                @click="togglePick(f.id, !isPicked(f.id))"
              >
                <td class="px-2 py-2" @click.stop>
                  <Checkbox
                    :model-value="isPicked(f.id)"
                    @update:model-value="(v) => togglePick(f.id, v === true)"
                  />
                </td>
                <td class="max-w-[260px] truncate px-2 py-2" :title="f.name">{{ f.name }}</td>
                <td class="px-2 py-2">
                  <span class="inline-flex items-center gap-1">
                    <component :is="typeIcon(f.file_type)" class="text-muted-foreground size-3.5" />
                    {{ t(`library.fileType.${f.file_type}`) }}
                  </span>
                </td>
                <td class="px-2 py-2 tabular-nums">{{ fmtBytes(f.size_bytes) }}</td>
                <td class="text-muted-foreground px-2 py-2 text-xs tabular-nums">{{ formatDateTimeFull(f.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </section>
      </div>

      <!-- 底部：已选 N + 推送按钮 -->
      <div class="flex items-center justify-between border-t pt-3">
        <span class="text-muted-foreground text-sm">{{ t('phone.rc.pickerSelected', { n: selectedCount }) }}</span>
        <Button :disabled="!selectedCount" @click="confirm">
          {{ t('phone.rc.pushTo', { n: targetCount }) }}
        </Button>
      </div>
    </DialogContent>
  </Dialog>
</template>
