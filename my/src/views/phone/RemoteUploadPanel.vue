<script setup lang="ts">
import { Images, Loader2, Upload, X } from 'lucide-vue-next'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import phoneApi from '@/api/modules/phone'
import { Button } from '@/components/ui/button'
import LibraryPickerDialog from './LibraryPickerDialog.vue'

const props = defineProps<{ phoneId: number, phones?: { id: number, name: string }[] }>()

const { t } = useI18n()

const UPLOAD_DIR = '/sdcard/Download'
const MAX_UPLOAD = 10 // 中台 batch-upload 一次限 1-10 个文件

const uploading = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

// 群控：可选上传目标范围（主控 / 所有）；单机时无此概念。
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

// 上传：文件始终落在 UPLOAD_DIR，不跟随目录浏览。
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
        await phoneApi.fileUpload(tg.id, UPLOAD_DIR, list, (pct) => {
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
    return
  }
  // 单台（主控）
  try {
    await phoneApi.fileUpload(tgts[0].id, UPLOAD_DIR, list)
    toast.success(t('phone.rc.fileUploadOk'), { description: t('phone.rc.fileUploadHint') })
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

// ---- 从素材库选择并推送 ----
const pickerOpen = ref(false)
function pickFromLibrary() {
  pickerOpen.value = true
}

// 推送阶段失败可能整请求前置失败（锁定/文件失效）：从错误消息辨识锁定态以给精准提示。
function isLockedErr(err: unknown): boolean {
  const msg = err && typeof err === 'object'
    ? ((err as { message?: string, response?: { data?: { message?: string } } }).response?.data?.message
        ?? (err as { message?: string }).message
        ?? '')
    : ''
  return /超限|超额|locked/i.test(msg)
}

// 选择器确认：复用现有 scope 决定目标手机，推送到 /sdcard/Download。
async function pushFromLibrary(fileIds: number[]) {
  if (!fileIds.length || uploading.value)
    return
  const tgts = scopeTargets.value
  const phoneIds = tgts.map(t => t.id)
  uploading.value = true
  // 多台（群控「所有」）：用返回 results 填充 jobs（无逐台实时 %，最终态满格/红）。
  if (tgts.length > 1) {
    jobs.value = tgts.map(tg => ({ id: tg.id, name: tg.name, progress: 0, status: 'uploading' as const }))
    try {
      const res = await phoneApi.pushFromLibrary(phoneIds, fileIds)
      const results = res.data.results ?? []
      let okCount = 0
      for (const job of jobs.value) {
        const r = results.find(x => x.phone_id === job.id)
        if (r?.ok) {
          job.progress = 100
          job.status = 'done'
          okCount++
        }
        else {
          job.status = 'error'
        }
      }
      if (okCount === tgts.length)
        toast.success(t('phone.rc.pushOk'))
      else if (okCount > 0)
        toast.warning(t('phone.rc.pushPartial', { ok: okCount, total: tgts.length }))
      else
        toast.error(t('phone.rc.pushFail'))
    }
    catch (err) {
      // 整请求前置失败（锁定/文件失效/非属主）：全部标红。
      jobs.value.forEach((job) => { job.status = 'error' })
      toast.error(isLockedErr(err) ? t('phone.rc.pushLocked') : t('phone.rc.pushFail'))
    }
    finally {
      uploading.value = false
    }
    return
  }
  // 单台（主控）
  try {
    const res = await phoneApi.pushFromLibrary(phoneIds, fileIds)
    const ok = res.data.results?.[0]?.ok ?? false
    if (ok)
      toast.success(t('phone.rc.pushOk'), { description: t('phone.rc.fileUploadHint') })
    else
      toast.error(t('phone.rc.pushFail'))
  }
  catch (err) {
    toast.error(isLockedErr(err) ? t('phone.rc.pushLocked') : t('phone.rc.pushFail'))
  }
  finally {
    uploading.value = false
  }
}

function onPicked(e: Event) {
  const input = e.target as HTMLInputElement
  const list = input.files ? Array.from(input.files) : []
  input.value = '' // 允许再次选择同一文件
  uploadFiles(list)
}
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <!-- 标题栏 -->
    <div class="flex items-center border-b px-3 py-2">
      <span class="text-sm font-medium">{{ t('phone.rc.uploadTitle') }}</span>
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

    <div class="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto p-3">
      <!-- 固定上传路径提示 -->
      <div class="rounded-md border bg-muted/40 px-3 py-2 text-xs">
        <div class="font-mono font-medium text-foreground">{{ UPLOAD_DIR }}</div>
        <div class="mt-0.5 text-muted-foreground">{{ t('phone.rc.uploadFixedPathHint') }}</div>
      </div>

      <!-- 上传本地文件 -->
      <div class="flex flex-col gap-2">
        <Button
          variant="outline"
          class="w-full gap-2"
          :disabled="uploading"
          @click="pickUpload"
        >
          <Loader2 v-if="uploading" class="size-4 animate-spin" />
          <Upload v-else class="size-4" />
          {{ t('phone.rc.uploadPick') }}
        </Button>
        <input ref="fileInput" type="file" multiple class="hidden" @change="onPicked">
      </div>

      <!-- 多台上传进度（群控「所有」） -->
      <div v-if="jobs.length" class="rounded-md border px-2 py-1.5 text-xs">
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

      <!-- 素材库 -->
      <div class="flex flex-col gap-1.5">
        <div class="text-xs font-medium text-muted-foreground">{{ t('phone.rc.materialTitle') }}</div>
        <Button
          variant="outline"
          class="w-full gap-2"
          :disabled="uploading"
          @click="pickFromLibrary"
        >
          <Images class="size-4" />
          {{ t('phone.rc.pickFromLibrary') }}
        </Button>
      </div>
    </div>

    <!-- 素材库选择弹层 -->
    <LibraryPickerDialog
      v-model:open="pickerOpen"
      :target-count="scopeTargets.length"
      @confirm="pushFromLibrary"
    />
  </div>
</template>
