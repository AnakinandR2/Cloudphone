<script setup lang="ts">
import { Loader2, UploadCloud, X } from 'lucide-vue-next'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import appApi from '@/api/modules/app'
import { uploadLibraryFile } from '@/api/modules/library'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

const emit = defineEmits<{ success: [] }>()
const open = defineModel<boolean>('open', { default: false })

const { t } = useI18n()

// 上传阶段机：idle → hashing（流式算 md5/slice_md5）→ uploading（直传素材库 S3）→ parsing（服务端 finalize 回读解析）。
type Phase = 'idle' | 'hashing' | 'uploading' | 'parsing'
const phase = ref<Phase>('idle')
const progress = ref(0)
const errorMsg = ref('')
const isDragging = ref(false)
const fileName = ref('')
const fileSizeText = ref('')
const fileInput = ref<HTMLInputElement | null>(null)

const MAX_SIZE = 5 * 1024 * 1024 * 1024

function formatSize(bytes: number): string {
  if (bytes >= 1073741824)
    return `${(bytes / 1073741824).toFixed(2)} GB`
  if (bytes >= 1048576)
    return `${(bytes / 1048576).toFixed(2)} MB`
  if (bytes >= 1024)
    return `${(bytes / 1024).toFixed(2)} KB`
  return `${bytes} B`
}

const busy = computed(() => phase.value !== 'idle')
const phaseText = computed(() => {
  switch (phase.value) {
    case 'hashing': return t('app.phaseHashing')
    case 'uploading': return t('app.phaseUploading', { n: progress.value })
    case 'parsing': return t('app.phaseParsing')
    default: return ''
  }
})

function resetState() {
  phase.value = 'idle'
  progress.value = 0
  errorMsg.value = ''
  isDragging.value = false
  fileName.value = ''
  fileSizeText.value = ''
}

function close() {
  open.value = false
  resetState()
}

function validateFile(file: File): string | null {
  const name = file.name.toLowerCase()
  if (!name.endsWith('.apk') && !name.endsWith('.xapk'))
    return t('app.fileTypeErr')
  if (file.size > MAX_SIZE)
    return t('app.fileTooLarge')
  return null
}

function pickFile() {
  fileInput.value?.click()
}
function onFileInput(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (file)
    handleFile(file)
}
function onDragOver(e: DragEvent) {
  e.preventDefault()
  isDragging.value = true
}
function onDragLeave() {
  isDragging.value = false
}
function onDrop(e: DragEvent) {
  e.preventDefault()
  isDragging.value = false
  const file = e.dataTransfer?.files?.[0]
  if (file)
    handleFile(file)
}

// 素材库两段式直传 → finalize 服务端回读解析。
async function handleFile(file: File) {
  const err = validateFile(file)
  if (err) {
    errorMsg.value = err
    return
  }
  errorMsg.value = ''
  fileName.value = file.name
  fileSizeText.value = formatSize(file.size)
  progress.value = 0

  try {
    // 1) 流式算 md5/slice_md5 → presign（含配额预占 + 秒传查重）。
    //    秒传命中跳过 PUT/confirm；否则直传 S3 + confirm。folder_id=0 根目录。
    phase.value = 'hashing'
    const { fileId, instant } = await uploadLibraryFile(file, {
      folderId: 0,
      onHashing: () => { phase.value = 'hashing' },
      onProgress: (pct) => { phase.value = 'uploading'; progress.value = pct },
    })
    // 2) finalize（服务端回读对象解析元数据 + 图标 + MD5，写 app_user_meta）。
    phase.value = 'parsing'
    const { data: app } = await appApi.finalize(fileId)
    if (app.parse_status === 'failed') {
      toast.warning(t('app.parseFailedToast'), { description: app.parse_error || fileName.value })
    }
    else if (instant) {
      toast.success(t('app.instantOk'), { description: app.app_name || fileName.value })
    }
    else {
      toast.success(t('app.uploadOk'), { description: app.app_name || fileName.value })
    }
    emit('success')
    close()
  }
  catch (e: any) {
    errorMsg.value = e?.message || t('app.uploadFail')
    phase.value = 'idle'
    fileName.value = ''
  }
}

function onOpenChange(v: boolean) {
  if (!v)
    close()
}
</script>

<template>
  <Dialog v-model:open="open" @update:open="onOpenChange">
    <DialogContent class="sm:max-w-md" @interact-outside="(e: Event) => busy && e.preventDefault()">
      <DialogHeader>
        <DialogTitle>{{ t('app.uploadTitle') }}</DialogTitle>
        <DialogDescription>{{ t('app.dropHintSub') }}</DialogDescription>
      </DialogHeader>

      <!-- 选择文件（空闲态） -->
      <div
        v-if="phase === 'idle'"
        class="flex cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed py-10 text-center transition-colors"
        :class="isDragging ? 'border-primary bg-primary/5' : 'border-muted-foreground/25 hover:border-primary/50'"
        @click="pickFile"
        @dragover="onDragOver"
        @dragleave="onDragLeave"
        @drop="onDrop"
      >
        <UploadCloud class="size-8 text-muted-foreground" />
        <p class="text-sm">
          {{ t('app.dropHint') }}
        </p>
        <input ref="fileInput" type="file" accept=".apk,.xapk" class="hidden" @change="onFileInput">
      </div>

      <!-- 上传 / 解析进行中 -->
      <div v-else class="space-y-3 py-2">
        <div class="flex items-center gap-2 text-sm">
          <Loader2 class="size-4 animate-spin text-primary" />
          <span class="truncate">{{ fileName }}</span>
          <span class="ml-auto shrink-0 text-muted-foreground">{{ fileSizeText }}</span>
        </div>
        <div class="h-2 w-full overflow-hidden rounded-full bg-muted">
          <div class="h-full rounded-full bg-primary transition-all duration-200" :style="{ width: `${progress}%` }" />
        </div>
        <p class="text-center text-xs text-muted-foreground">
          {{ phaseText }}
        </p>
      </div>

      <p v-if="errorMsg" class="text-sm text-destructive">
        {{ errorMsg }}
      </p>

      <DialogFooter>
        <Button variant="outline" :disabled="busy" @click="close">
          <X class="size-4" /> {{ t('app.cancel') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
