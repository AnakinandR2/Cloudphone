<script setup lang="ts">
import type { TaskReportDetail } from '@/types/automation'
import { CheckCircle2, Loader2, XCircle } from 'lucide-vue-next'
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import automationApi from '@/api/modules/automation'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

const props = defineProps<{ midTaskId: number | null }>()
const open = defineModel<boolean>({ default: false })

const { t } = useI18n()

const POLL_MS = 3000
const TIMEOUT_MS = 120_000

const loading = ref(false)
const detail = ref<TaskReportDetail | null>(null)
const errMsg = ref('')
let timer: ReturnType<typeof setTimeout> | null = null
let deadline = 0

function clearTimer() {
  if (timer) {
    clearTimeout(timer)
    timer = null
  }
}

const statusTone = computed(() => {
  const s = detail.value?.status
  if (s === 'COMPLETED')
    return 'border-green-500 bg-green-500/10 text-green-600 dark:text-green-400'
  if (s === 'FAILED' || s === 'CANCELLED')
    return 'text-destructive border-destructive/40 bg-destructive/10'
  return 'text-amber-600 border-amber-500/40 bg-amber-500/10'
})
const succeeded = computed(() => detail.value?.status === 'COMPLETED')
const polling = computed(() => loading.value || (detail.value != null && !detail.value.terminal))

async function poll() {
  if (props.midTaskId == null)
    return
  try {
    const { data } = await automationApi.taskDetail(props.midTaskId)
    detail.value = data
    if (data.terminal) {
      clearTimer()
      return
    }
  }
  catch (e: any) {
    errMsg.value = e?.message ?? ''
  }
  if (Date.now() > deadline) {
    clearTimer()
    errMsg.value = t('taskLog.timeout')
    return
  }
  timer = setTimeout(poll, POLL_MS)
}

watch(open, async (v) => {
  clearTimer()
  detail.value = null
  errMsg.value = ''
  if (!v || props.midTaskId == null)
    return
  loading.value = true
  deadline = Date.now() + TIMEOUT_MS
  await poll()
  loading.value = false
})

onUnmounted(clearTimer)
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="flex max-h-[85vh] flex-col gap-0 p-0 sm:max-w-2xl">
      <DialogHeader class="border-b p-4">
        <DialogTitle>{{ t('taskLog.reportTitle') }}</DialogTitle>
        <DialogDescription>{{ t('taskLog.reportDesc') }}</DialogDescription>
      </DialogHeader>

      <div class="min-h-0 flex-1 space-y-4 overflow-y-auto p-4 text-sm">
        <div class="flex items-center gap-3">
          <Loader2 v-if="polling" class="size-4 animate-spin text-muted-foreground" />
          <CheckCircle2 v-else-if="succeeded" class="size-4 text-green-600" />
          <XCircle v-else-if="detail?.terminal" class="size-4 text-destructive" />
          <span class="text-muted-foreground">{{ t('taskLog.colStatus') }}：</span>
          <Badge v-if="detail" variant="outline" :class="statusTone">
            {{ detail.statusDesc || detail.status }}
          </Badge>
          <span v-if="detail?.taskNo" class="ml-auto font-mono text-xs text-muted-foreground">#{{ detail.taskNo }}</span>
        </div>

        <p v-if="errMsg" class="rounded-md bg-destructive/10 px-3 py-2 text-destructive">{{ errMsg }}</p>

        <div v-if="detail?.result">
          <div class="mb-1 text-xs text-muted-foreground">{{ t('taskLog.result') }}</div>
          <pre class="overflow-x-auto rounded-md bg-muted p-3 text-xs">{{ detail.result }}</pre>
        </div>

        <div v-if="detail?.runLog">
          <div class="mb-1 text-xs text-muted-foreground">
            {{ t('taskLog.log') }}
            <span v-if="detail.runDurationMs"> · {{ (detail.runDurationMs / 1000).toFixed(1) }}s</span>
          </div>
          <pre class="max-h-60 overflow-auto whitespace-pre-wrap rounded-md bg-muted p-3 text-xs leading-relaxed">{{ detail.runLog }}</pre>
        </div>

        <div v-if="detail?.screenshotUrl">
          <div class="mb-1 text-xs text-muted-foreground">{{ t('taskLog.screenshot') }}</div>
          <img :src="detail.screenshotUrl" class="max-h-72 rounded-md border" alt="screenshot">
        </div>

        <p v-if="!detail && !loading && !errMsg" class="text-muted-foreground">
          {{ t('taskLog.waiting') }}
        </p>
      </div>
    </DialogContent>
  </Dialog>
</template>
