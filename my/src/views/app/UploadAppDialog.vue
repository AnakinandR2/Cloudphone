<script setup lang="ts">
import type { ParsedAppInfo } from '@/types/app'
import { Loader2, Package, UploadCloud, X } from 'lucide-vue-next'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import appApi from '@/api/modules/app'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { computeBlobMd5, computeFileMd5 } from '@/utils/md5'

const emit = defineEmits<{ success: [] }>()
const open = defineModel<boolean>('open', { default: false })

const { t } = useI18n()

// 上传阶段机：idle → hashing → uploading → completing → parsing → parsed →（确认）submitting。
type Phase = 'idle' | 'hashing' | 'uploading' | 'completing' | 'parsing' | 'parsed'
const phase = ref<Phase>('idle')
const progress = ref(0)
const errorMsg = ref('')
const submitting = ref(false)
const isDragging = ref(false)
const fileName = ref('')
const fileSizeText = ref('')
const parsed = ref<ParsedAppInfo | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)

// 取消标志：关闭弹窗 / 重选文件时置 true，让进行中的分片循环尽快退出。
let aborted = false

const CONCURRENCY = 3
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

const busy = computed(() => phase.value !== 'idle' && phase.value !== 'parsed')
const phaseText = computed(() => {
  switch (phase.value) {
    case 'hashing': return t('app.phaseHashing')
    case 'uploading': return t('app.phaseUploading', { n: progress.value })
    case 'completing': return t('app.phaseCompleting')
    case 'parsing': return t('app.phaseParsing')
    default: return ''
  }
})

function resetState() {
  aborted = false
  phase.value = 'idle'
  progress.value = 0
  errorMsg.value = ''
  submitting.value = false
  isDragging.value = false
  fileName.value = ''
  fileSizeText.value = ''
  parsed.value = null
}

function close() {
  aborted = true
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
function handleRemoveFile() {
  aborted = true
  resetState()
}

// 浏览器驱动的分片上传：计算整文件 MD5 → initiate（可能秒传）→ 逐片上传 → 合并 → 解析。
async function handleFile(file: File) {
  const err = validateFile(file)
  if (err) {
    errorMsg.value = err
    return
  }
  aborted = false
  errorMsg.value = ''
  fileName.value = file.name
  fileSizeText.value = formatSize(file.size)
  progress.value = 0

  try {
    // 1) 整文件 MD5（中台 initiate 必需，命中即秒传）
    phase.value = 'hashing'
    const contentMd5 = await computeFileMd5(file)
    if (aborted)
      return

    // 2) initiate
    phase.value = 'uploading'
    const { data: init } = await appApi.uploadInitiate({ fileName: file.name, fileSize: file.size, contentMd5 })
    if (aborted)
      return

    // 秒传命中：中台已持有相同 MD5 的 APK，直接用 appInfo 填充确认面板，跳过分片/合并/解析。
    if (init.uploadSuccess) {
      const a = init.appInfo
      parsed.value = {
        uploadId: '', // 空 uploadId 标记秒传：确认后直接 create、无需轮询
        appName: a?.appName ?? '',
        packageName: a?.packageName ?? '',
        version: a?.version ?? '',
        iconPath: a?.iconPath ?? '',
        fileSize: a?.fileSize || fileSizeText.value,
        md5: a?.md5 || contentMd5,
      }
      progress.value = 100
      phase.value = 'parsed'
      return
    }

    // 3) 逐片上传（3 并发；JS 单线程，partCursor++ 不会竞态）
    const { uploadId, totalParts, partSize } = init
    const partProgress = Array.from({ length: totalParts }, () => 0)
    const refresh = () => {
      progress.value = Math.round(partProgress.reduce((s, p) => s + p, 0) / totalParts)
    }
    let cursor = 0
    const worker = async () => {
      while (cursor < totalParts) {
        if (aborted)
          return
        const i = cursor++
        const start = i * partSize
        const chunk = file.slice(start, Math.min(start + partSize, file.size))
        const chunkMd5 = await computeBlobMd5(chunk)
        if (aborted)
          return
        const fd = new FormData()
        fd.append('file', chunk, file.name)
        await appApi.uploadPart(fd, { uploadId, partNumber: i + 1, contentMd5: chunkMd5 }, (pct) => {
          partProgress[i] = pct
          refresh()
        })
        partProgress[i] = 100
        refresh()
      }
    }
    await Promise.all(Array.from({ length: Math.min(CONCURRENCY, totalParts) }, worker))
    if (aborted)
      return

    // 4) 合并
    phase.value = 'completing'
    await appApi.uploadComplete(uploadId)
    if (aborted)
      return

    // 5) 解析元信息
    phase.value = 'parsing'
    const { data: info } = await appApi.uploadParse(uploadId)
    if (aborted)
      return
    parsed.value = {
      uploadId: info.uploadId || uploadId,
      appName: info.appName ?? '',
      packageName: info.packageName ?? '',
      version: info.version ?? '',
      iconPath: info.iconPath ?? '',
      fileSize: info.fileSize || fileSizeText.value,
      md5: info.md5 || contentMd5,
    }
    phase.value = 'parsed'
  }
  catch (e: any) {
    if (aborted)
      return
    errorMsg.value = e?.message || t('app.uploadFail')
    phase.value = 'idle'
    fileName.value = ''
  }
}

function sleep(ms: number) {
  return new Promise<void>(resolve => setTimeout(resolve, ms))
}

// create 成功后轮询处理状态，直到 OSS_SUCCESS（最多 ~25 分钟）。
async function pollAfterCreate(uploadId: string): Promise<'success' | 'pending'> {
  for (let attempt = 0; attempt < 300; attempt++) {
    if (!open.value)
      throw new Error(t('app.uploadCanceled'))
    const { data } = await appApi.uploadStatus(uploadId)
    if (data.status === 'OSS_SUCCESS')
      return 'success'
    await sleep(5000)
  }
  return 'pending'
}

async function handleConfirm() {
  if (!parsed.value)
    return
  submitting.value = true
  errorMsg.value = ''
  try {
    await appApi.uploadCreate({
      uploadId: parsed.value.uploadId,
      appName: parsed.value.appName,
      packageName: parsed.value.packageName,
      version: parsed.value.version,
      iconPath: parsed.value.iconPath,
      fileSize: parsed.value.fileSize,
      md5: parsed.value.md5,
    })
    // 秒传命中（uploadId 为空）：文件已就绪，无需轮询。
    if (!parsed.value.uploadId) {
      toast.success(t('app.uploadOk'), { description: fileName.value })
      emit('success')
      close()
      return
    }
    const outcome = await pollAfterCreate(parsed.value.uploadId)
    if (outcome === 'success') {
      toast.success(t('app.uploadOk'), { description: fileName.value })
      emit('success')
      close()
    }
    else {
      toast.info(t('app.processingLong'))
      emit('success') // 列表里以「创建中」呈现，由列表轮询收敛
      close()
    }
  }
  catch (e: any) {
    if (e?.message === t('app.uploadCanceled'))
      return
    errorMsg.value = e?.message || t('app.createFail')
  }
  finally {
    submitting.value = false
  }
}

// 关闭弹窗时复位（避免下次打开残留上次状态）。
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

      <!-- 上传 / 处理进行中 -->
      <div v-else-if="busy" class="space-y-3 py-2">
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

      <!-- 解析完成：确认应用信息 -->
      <div v-else-if="phase === 'parsed' && parsed" class="space-y-3 py-2">
        <div class="flex items-center gap-3">
          <img v-if="parsed.iconPath" :src="parsed.iconPath" class="size-12 shrink-0 rounded-lg" alt="">
          <span v-else class="flex size-12 shrink-0 items-center justify-center rounded-lg bg-muted">
            <Package class="size-6 text-muted-foreground" />
          </span>
          <div class="min-w-0">
            <p class="truncate font-medium">
              {{ parsed.appName || fileName }}
            </p>
            <p class="truncate text-xs text-muted-foreground">
              {{ parsed.packageName || '-' }}
            </p>
          </div>
        </div>
        <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 text-sm">
          <dt class="text-muted-foreground">
            {{ t('app.fieldVersion') }}
          </dt>
          <dd class="tabular-nums">
            {{ parsed.version || '-' }}
          </dd>
          <dt class="text-muted-foreground">
            {{ t('app.fieldSize') }}
          </dt>
          <dd class="tabular-nums">
            {{ parsed.fileSize || fileSizeText }}
          </dd>
        </dl>
      </div>

      <p v-if="errorMsg" class="text-sm text-destructive">
        {{ errorMsg }}
      </p>

      <DialogFooter>
        <template v-if="phase === 'parsed'">
          <Button variant="outline" :disabled="submitting" @click="handleRemoveFile">
            {{ t('app.reupload') }}
          </Button>
          <Button :disabled="submitting" @click="handleConfirm">
            <Loader2 v-if="submitting" class="size-4 animate-spin" />
            {{ t('app.confirm') }}
          </Button>
        </template>
        <Button v-else variant="outline" :disabled="busy && phase !== 'idle'" @click="close">
          <X class="size-4" /> {{ t('app.cancel') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
