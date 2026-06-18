<script setup lang="ts">
import type { AutomationScript } from '@/types/automation'
import type { CloudPhone } from '@/types/phone'
import { CalendarClock, Loader2 } from 'lucide-vue-next'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import automationApi from '@/api/modules/automation'
import phoneApi from '@/api/modules/phone'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

const props = defineProps<{
  presetScriptId?: number
  presetCpId?: string
}>()
const open = defineModel<boolean>({ default: false })
const emit = defineEmits<{ created: [firstTaskId: number | null] }>()

const { t } = useI18n()

const mode = ref<'once' | 'plan'>('once')
const scripts = ref<AutomationScript[]>([])
const phones = ref<CloudPhone[]>([])
const scriptId = ref<number>(0)
const selected = ref<Set<string>>(new Set())
const name = ref('')
const frequency = ref<'INTERVAL' | 'DAILY'>('INTERVAL')
const intervalValue = ref(60)
const executionTime = ref('09:00')
const startTime = ref('')
const endTime = ref('')
const submitting = ref(false)

const runnablePhones = computed(() => phones.value.filter(p => p.cp_id))

// 把 Date 格式化为 <input type="datetime-local"> 需要的「本地 YYYY-MM-DDTHH:mm」。
function toLocalInput(d: Date) {
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`
}
function fmt(d: Date) {
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

// 预计下次运行时间（按所填频率/起止本地计算；中台按北京时间解析，本地=北京时一致）。
// 返回 Date / null(已超出结束) / undefined(非计划模式)。
const nextRun = computed<Date | null | undefined>(() => {
  if (mode.value !== 'plan')
    return undefined
  const now = new Date()
  const start = startTime.value ? new Date(startTime.value) : now
  const end = endTime.value ? new Date(endTime.value) : null
  let next: Date
  if (frequency.value === 'INTERVAL') {
    const stepMs = Math.max(1, intervalValue.value) * 60000
    if (start.getTime() >= now.getTime()) {
      next = start
    }
    else {
      const k = Math.ceil((now.getTime() - start.getTime()) / stepMs)
      next = new Date(start.getTime() + k * stepMs)
    }
  }
  else {
    const [hh, mm] = (executionTime.value || '09:00').split(':').map(Number)
    const base = new Date(Math.max(now.getTime(), start.getTime()))
    next = new Date(base.getFullYear(), base.getMonth(), base.getDate(), hh, mm, 0)
    if (next.getTime() < base.getTime())
      next.setDate(next.getDate() + 1)
  }
  if (end && next.getTime() > end.getTime())
    return null
  return next
})
const nextRunText = computed(() => (nextRun.value ? fmt(nextRun.value) : ''))

watch(open, async (v) => {
  if (!v)
    return
  mode.value = 'once'
  name.value = ''
  selected.value = new Set(props.presetCpId ? [props.presetCpId] : [])
  scriptId.value = props.presetScriptId ?? 0
  // 默认填充起止：起=当前，止=一年后（可见可改）。
  const now = new Date()
  const oneYear = new Date(now)
  oneYear.setFullYear(oneYear.getFullYear() + 1)
  startTime.value = toLocalInput(now)
  endTime.value = toLocalInput(oneYear)
  const [s, p] = await Promise.all([
    automationApi.usableScripts(),
    phoneApi.list({ page: 1, size: 200 }),
  ])
  scripts.value = s.data ?? []
  phones.value = p.data.list ?? []
  if (!scriptId.value && scripts.value.length)
    scriptId.value = scripts.value[0].id
})

function toggle(cpId: string, on: boolean) {
  const next = new Set(selected.value)
  if (on)
    next.add(cpId)
  else next.delete(cpId)
  selected.value = next
}

async function submit() {
  if (!scriptId.value) {
    toast.error(t('taskSchedule.errScript'))
    return
  }
  const cpIds = [...selected.value]
  if (!cpIds.length) {
    toast.error(t('taskSchedule.errTargets'))
    return
  }
  submitting.value = true
  try {
    if (mode.value === 'once') {
      const { data } = await automationApi.runTask(scriptId.value, cpIds, name.value)
      toast.success(t('taskSchedule.runOk', { n: data.length }))
      open.value = false
      emit('created', data[0]?.midTaskId ?? null)
    }
    else {
      if (!name.value.trim()) {
        toast.error(t('taskSchedule.errName'))
        return
      }
      if (!startTime.value || !endTime.value) {
        toast.error(t('taskSchedule.errTime'))
        return
      }
      if (new Date(endTime.value).getTime() <= new Date(startTime.value).getTime()) {
        toast.error(t('taskSchedule.errTimeOrder'))
        return
      }
      // datetime-local 是「YYYY-MM-DDTHH:mm」，中台要秒，补 :00。
      await automationApi.createPlan({
        scriptId: scriptId.value,
        name: name.value,
        frequency: frequency.value,
        intervalValue: frequency.value === 'INTERVAL' ? intervalValue.value : undefined,
        executionTime: frequency.value === 'DAILY' ? `${executionTime.value}:00` : undefined,
        startTime: `${startTime.value}:00`,
        endTime: `${endTime.value}:00`,
        cpIds,
      })
      toast.success(t('taskSchedule.planOk'))
      open.value = false
      emit('created', null)
    }
  }
  finally {
    submitting.value = false
  }
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="flex max-h-[88vh] flex-col gap-0 p-0 sm:max-w-xl">
      <DialogHeader class="border-b p-4">
        <DialogTitle>{{ t('taskSchedule.newTask') }}</DialogTitle>
        <DialogDescription>{{ t('taskSchedule.newDesc') }}</DialogDescription>
      </DialogHeader>

      <div class="min-h-0 flex-1 space-y-4 overflow-y-auto p-4">
        <Tabs v-model="mode">
          <TabsList class="w-full">
            <TabsTrigger value="once" class="flex-1">
              {{ t('taskSchedule.modeOnce') }}
            </TabsTrigger>
            <TabsTrigger value="plan" class="flex-1">
              {{ t('taskSchedule.modePlan') }}
            </TabsTrigger>
          </TabsList>
        </Tabs>

        <div class="grid gap-2">
          <Label>{{ t('taskSchedule.colScript') }}</Label>
          <NativeSelect v-model="scriptId">
            <NativeSelectOption v-if="!scripts.length" :value="0" disabled>
              {{ t('taskSchedule.noScript') }}
            </NativeSelectOption>
            <NativeSelectOption v-for="s in scripts" :key="s.id" :value="s.id">
              {{ s.name }}{{ s.store ? ` · ${t('script.tabStore')}` : '' }}
            </NativeSelectOption>
          </NativeSelect>
        </div>

        <div class="grid gap-2">
          <Label>{{ mode === 'plan' ? t('taskSchedule.planName') : t('taskSchedule.taskName') }}</Label>
          <Input v-model="name" :placeholder="t('taskSchedule.namePh')" />
        </div>

        <!-- 周期设置 -->
        <template v-if="mode === 'plan'">
          <div class="grid gap-2">
            <Label>{{ t('taskSchedule.colTrigger') }}</Label>
            <div class="flex gap-2">
              <Button :variant="frequency === 'INTERVAL' ? 'default' : 'outline'" size="sm" @click="frequency = 'INTERVAL'">
                {{ t('taskSchedule.triggerInterval') }}
              </Button>
              <Button :variant="frequency === 'DAILY' ? 'default' : 'outline'" size="sm" @click="frequency = 'DAILY'">
                {{ t('taskSchedule.triggerDaily') }}
              </Button>
            </div>
          </div>
          <div v-if="frequency === 'INTERVAL'" class="grid gap-2">
            <Label>{{ t('taskSchedule.intervalLabel') }}</Label>
            <Input v-model.number="intervalValue" type="number" min="1" class="w-32" />
          </div>
          <div v-else class="grid gap-2">
            <Label>{{ t('taskSchedule.timeLabel') }}</Label>
            <Input v-model="executionTime" type="time" class="w-32" />
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div class="grid gap-2">
              <Label>{{ t('taskSchedule.startTime') }}</Label>
              <Input v-model="startTime" type="datetime-local" />
            </div>
            <div class="grid gap-2">
              <Label>{{ t('taskSchedule.endTime') }}</Label>
              <Input v-model="endTime" type="datetime-local" />
            </div>
          </div>
          <!-- 预计下次运行时间 -->
          <div class="flex items-center gap-2 rounded-md bg-muted px-3 py-2 text-sm">
            <CalendarClock class="size-4 text-muted-foreground" />
            <span class="text-muted-foreground">{{ t('taskSchedule.nextRun') }}：</span>
            <span v-if="nextRunText" class="font-medium tabular-nums">{{ nextRunText }}</span>
            <span v-else class="text-destructive">{{ t('taskSchedule.nextRunEnded') }}</span>
          </div>
        </template>

        <!-- 目标机器 -->
        <div class="grid gap-2">
          <Label>{{ t('taskSchedule.colTargets') }} <span class="text-xs text-muted-foreground">({{ selected.size }})</span></Label>
          <div class="max-h-44 space-y-1 overflow-y-auto rounded-md border p-2">
            <p v-if="!runnablePhones.length" class="p-2 text-xs text-muted-foreground">
              {{ t('taskSchedule.noPhone') }}
            </p>
            <label v-for="p in runnablePhones" :key="p.id" class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-sm hover:bg-muted">
              <Checkbox :model-value="selected.has(p.cp_id)" @update:model-value="(v) => toggle(p.cp_id, v === true)" />
              <span class="font-medium">{{ p.name }}</span>
              <code class="ml-auto font-mono text-xs text-muted-foreground">{{ p.cp_id }}</code>
            </label>
          </div>
        </div>
      </div>

      <DialogFooter class="border-t p-3">
        <Button variant="outline" @click="open = false">
          {{ t('crud.cancel') }}
        </Button>
        <Button :disabled="submitting" @click="submit">
          <Loader2 v-if="submitting" class="size-4 animate-spin" />
          {{ mode === 'once' ? t('taskSchedule.runNow') : t('taskSchedule.createPlan') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
