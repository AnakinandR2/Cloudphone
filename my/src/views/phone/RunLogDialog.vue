<script setup lang="ts">
import type { CloudPhone, RunLog } from '@/types/phone'
import { ChevronLeft, ChevronRight, Loader2 } from 'lucide-vue-next'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import phoneApi from '@/api/modules/phone'
import { runSessionStatusBadge } from '@/utils/statusBadge'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

const props = defineProps<{ phone: CloudPhone | null }>()
const open = defineModel<boolean>({ default: false })

const { t } = useI18n()

const PAGE_SIZE = 20
const logs = ref<RunLog[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))

async function load() {
  if (!props.phone)
    return
  loading.value = true
  try {
    const { data } = await phoneApi.runLogs(props.phone.id, { page: page.value, size: PAGE_SIZE })
    logs.value = data.list ?? []
    total.value = data.total ?? 0
  }
  catch {
    toast.error(t('phone.runLog.loadFail'))
    logs.value = []
    total.value = 0
  }
  finally {
    loading.value = false
  }
}

function go(p: number) {
  if (p < 1 || p > totalPages.value || loading.value)
    return
  page.value = p
  load()
}

watch(open, (v) => {
  if (v) {
    page.value = 1
    logs.value = []
    total.value = 0
    load()
  }
})

function reasonClass(log: RunLog) {
  return log.powerOffReasonCode && log.powerOffReasonCode !== 'SHUTDOWN'
    ? 'text-destructive'
    : 'text-muted-foreground'
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="flex max-h-[85vh] flex-col gap-0 p-0 sm:max-w-3xl">
      <DialogHeader class="border-b p-4">
        <DialogTitle>{{ t('phone.runLog.title') }}<span v-if="phone" class="text-muted-foreground"> — {{ phone.name }}</span></DialogTitle>
        <DialogDescription>{{ t('phone.runLog.desc') }}</DialogDescription>
      </DialogHeader>

      <div class="min-h-0 flex-1 overflow-y-auto">
        <div v-if="loading" class="flex items-center justify-center gap-2 p-10 text-sm text-muted-foreground">
          <Loader2 class="size-4 animate-spin" /> {{ t('common.loading', '加载中…') }}
        </div>
        <div v-else-if="!logs.length" class="p-10 text-center text-sm text-muted-foreground">
          {{ t('phone.runLog.empty') }}
        </div>
        <table v-else class="w-full text-sm">
          <thead class="sticky top-0 bg-muted/50 text-xs text-muted-foreground">
            <tr>
              <th class="px-3 py-2 text-left font-medium">{{ t('phone.runLog.powerOn') }}</th>
              <th class="px-3 py-2 text-left font-medium">{{ t('phone.runLog.powerOff') }}</th>
              <th class="px-3 py-2 text-left font-medium">{{ t('phone.runLog.duration') }}</th>
              <th class="px-3 py-2 text-left font-medium">{{ t('phone.runLog.status') }}</th>
              <th class="px-3 py-2 text-left font-medium">{{ t('phone.runLog.reason') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="log in logs" :key="log.logNo" class="border-t">
              <td class="px-3 py-2 tabular-nums">{{ log.powerOnTime }}</td>
              <td class="px-3 py-2 tabular-nums">{{ log.powerOffTime }}</td>
              <td class="px-3 py-2 tabular-nums">{{ log.duration }}</td>
              <td class="px-3 py-2">
                <Badge v-bind="runSessionStatusBadge(log.sessionStatus)">{{ log.sessionStatus }}</Badge>
              </td>
              <td class="px-3 py-2" :class="reasonClass(log)">{{ log.powerOffReason }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 分页 -->
      <div class="flex items-center justify-between gap-2 border-t p-3 text-xs text-muted-foreground">
        <span>{{ t('phone.runLog.totalN', { n: total }) }}</span>
        <div class="flex items-center gap-2">
          <Button variant="outline" size="icon" class="size-7" :disabled="page <= 1 || loading" @click="go(page - 1)">
            <ChevronLeft class="size-4" />
          </Button>
          <span class="tabular-nums">{{ t('phone.runLog.pageOf', { cur: page, total: totalPages }) }}</span>
          <Button variant="outline" size="icon" class="size-7" :disabled="page >= totalPages || loading" @click="go(page + 1)">
            <ChevronRight class="size-4" />
          </Button>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>
