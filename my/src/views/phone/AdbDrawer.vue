<script setup lang="ts">
import type { AdbInfo, CloudPhone } from '@/types/phone'
import { Check, Clock, Copy, Eye, EyeOff, Loader2, Power, RefreshCw, Usb } from 'lucide-vue-next'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import phoneApi from '@/api/modules/phone'
import Popconfirm from '@/components/Popconfirm.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'

const props = defineProps<{ phone: CloudPhone | null }>()
const open = defineModel<boolean>({ default: false })
// changed：开/关 ADB 后通知父级刷新列表，使「已开 ADB」标记即时更新。
const emit = defineEmits<{ changed: [] }>()

const { t } = useI18n()

const info = ref<AdbInfo | null>(null)
const loading = ref(false)
const busy = ref(false)
const showToken = ref(false)

// 倒计时：now 每秒自增，驱动剩余时间重算。
const now = ref(Date.now())
let timer: ReturnType<typeof setInterval> | null = null

const enabled = computed(() => info.value?.enabled ?? false)
const connectCmd1 = computed(() => info.value?.adbAddress ? `adb connect ${info.value.adbAddress}` : '')
const connectCmd2 = computed(() =>
  info.value?.adbToken && info.value?.adbAddress
    ? `adb -s ${info.value.adbAddress} shell xlogin ${info.value.adbToken}`
    : '')

// 过期时间戳（毫秒）；无法解析时为 NaN。
const expireAtMs = computed(() => {
  const raw = info.value?.adbTokenExpiredAt
  if (!raw)
    return Number.NaN
  return Date.parse(raw)
})
const remainingMs = computed(() => Number.isNaN(expireAtMs.value) ? Number.NaN : expireAtMs.value - now.value)
const expired = computed(() => !Number.isNaN(remainingMs.value) && remainingMs.value <= 0)
const remainingText = computed(() => {
  const ms = remainingMs.value
  if (Number.isNaN(ms))
    return info.value?.adbTokenExpiredAt || '-'
  const total = Math.max(0, Math.floor(ms / 1000))
  const d = Math.floor(total / 86400)
  const h = Math.floor((total % 86400) / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  // token 有效期固定 +60 天，天数 ≥1 时省略秒，避免长跨度下秒级跳动无意义。
  return d > 0
    ? t('phone.adb.remainingFmtD', { d, h, m })
    : t('phone.adb.remainingFmt', { h, m, s })
})

function startTimer() {
  stopTimer()
  now.value = Date.now()
  timer = setInterval(() => { now.value = Date.now() }, 1000)
}
function stopTimer() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

async function load() {
  if (!props.phone)
    return
  loading.value = true
  try {
    const { data } = await phoneApi.adbInfo(props.phone.id)
    info.value = data
  }
  catch {
    toast.error(t('phone.adb.loadFail'))
  }
  finally {
    loading.value = false
  }
}

watch(open, (v) => {
  if (v) {
    info.value = null
    showToken.value = false
    startTimer()
    load()
  }
  else {
    stopTimer()
  }
})

onBeforeUnmount(stopTimer)

// enable 兼开启与续期：renew=true 时用「续期」文案。
async function enable(renew = false) {
  if (!props.phone || busy.value)
    return
  busy.value = true
  try {
    const { data } = await phoneApi.adbEnable(props.phone.id)
    info.value = data
    toast.success(t(renew ? 'phone.adb.renewOk' : 'phone.adb.enableOk'))
    if (!renew)
      emit('changed') // 开启使标记从无到有

  }
  catch {
    toast.error(t(renew ? 'phone.adb.renewFail' : 'phone.adb.enableFail'))
  }
  finally {
    busy.value = false
  }
}

async function disable() {
  if (!props.phone || busy.value)
    return
  busy.value = true
  try {
    await phoneApi.adbDisable(props.phone.id)
    toast.success(t('phone.adb.disableOk'))
    await load()
    emit('changed') // 关闭使标记消失
  }
  catch {
    toast.error(t('phone.adb.disableFail'))
  }
  finally {
    busy.value = false
  }
}

// copyText 优先用 Clipboard API；非安全上下文（如 http）下 navigator.clipboard 不可用，
// 回退到临时 textarea + execCommand，保证非 HTTPS 环境也能复制。
async function copyText(text: string): Promise<boolean> {
  // 1) Clipboard API：仅在安全上下文且文档已聚焦时用（未聚焦会抛 "Document is not focused"）。
  try {
    if (navigator.clipboard?.writeText && window.isSecureContext && document.hasFocus()) {
      await navigator.clipboard.writeText(text)
      return true
    }
  }
  catch { /* 落到回退 */ }
  // 2) 回退：临时 textarea 自己抢焦点 + execCommand，兼容非 HTTPS / 文档未聚焦。
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.setAttribute('readonly', '')
    ta.style.position = 'fixed'
    ta.style.top = '0'
    ta.style.left = '0'
    ta.style.width = '1px'
    ta.style.height = '1px'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.focus()
    ta.select()
    ta.setSelectionRange(0, text.length)
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    return ok
  }
  catch {
    return false
  }
}
async function copy(text: string) {
  if (!text)
    return
  if (await copyText(text))
    toast.success(t('phone.adb.copied'))
  else
    toast.error(t('phone.adb.copyFail'))
}
// 只读输入框聚焦时自动全选，方便整段复制。
function selectAll(e: FocusEvent) {
  (e.target as HTMLInputElement).select()
}
</script>

<template>
  <Sheet v-model:open="open">
    <SheetContent side="right" class="flex w-full flex-col gap-0 p-0 sm:max-w-md">
      <SheetHeader class="border-b">
        <SheetTitle class="flex items-center gap-2">
          <Usb class="size-4" /> {{ t('phone.adb.title') }}<span v-if="phone" class="text-muted-foreground"> — {{ phone.name }}</span>
        </SheetTitle>
        <SheetDescription>{{ t('phone.adb.desc') }}</SheetDescription>
      </SheetHeader>

      <div class="min-h-0 flex-1 overflow-y-auto">
        <div v-if="loading" class="flex items-center justify-center gap-2 p-8 text-sm text-muted-foreground">
          <Loader2 class="size-4 animate-spin" /> {{ t('common.loading', '加载中…') }}
        </div>

        <div v-else class="flex flex-col gap-4 p-4">
          <!-- 状态 + 关闭 -->
          <div class="flex items-center justify-between gap-2 rounded-md border p-3">
            <div class="flex items-center gap-2">
              <span class="text-sm font-medium">{{ t('phone.adb.status') }}</span>
              <Badge v-if="enabled" class="border-green-500 bg-green-500/10 text-green-600 dark:text-green-400" variant="outline">
                {{ t('phone.adb.enabled') }}
              </Badge>
              <Badge v-else variant="outline" class="text-muted-foreground">
                {{ t('phone.adb.disabled') }}
              </Badge>
            </div>
            <Popconfirm
              v-if="enabled"
              tone="warning"
              :title="t('phone.adb.disableConfirm')"
              @confirm="disable"
            >
              <Button variant="outline" size="sm" class="gap-1 border-amber-500 text-amber-600 dark:text-amber-400" :disabled="busy">
                <Power class="size-3.5" /> {{ t('phone.adb.disable') }}
              </Button>
            </Popconfirm>
          </div>

          <!-- 未开启：单个开启按钮 -->
          <div v-if="!enabled" class="flex flex-col gap-2 rounded-md border p-3">
            <Button class="gap-1" :disabled="busy" @click="enable(false)">
              <Loader2 v-if="busy" class="size-4 animate-spin" />
              <Check v-else class="size-4" />
              {{ t('phone.adb.enable') }}
            </Button>
          </div>

          <!-- 已开启：剩余时间 + 续期 -->
          <template v-else>
            <div class="flex items-center justify-between gap-2 rounded-md border p-3">
              <div class="flex min-w-0 items-center gap-2">
                <Clock class="size-4 shrink-0 text-muted-foreground" />
                <div class="flex min-w-0 flex-col">
                  <span class="text-xs text-muted-foreground">{{ t('phone.adb.remaining') }}</span>
                  <span
                    class="truncate text-sm font-medium tabular-nums"
                    :class="expired ? 'text-destructive' : ''"
                  >
                    {{ expired ? t('phone.adb.expired') : remainingText }}
                  </span>
                </div>
              </div>
              <Button variant="outline" size="sm" class="shrink-0 gap-1" :disabled="busy" @click="enable(true)">
                <RefreshCw class="size-3.5" :class="busy && 'animate-spin'" /> {{ t('phone.adb.renew') }}
              </Button>
            </div>

            <!-- 连接地址 + Token -->
            <div class="flex flex-col gap-3 rounded-md border p-3">
              <div class="flex flex-col gap-1">
                <span class="text-xs font-medium text-muted-foreground">{{ t('phone.adb.address') }}</span>
                <div class="flex items-center gap-2">
                  <input
                    :value="info?.adbAddress || '-'"
                    readonly
                    class="min-w-0 flex-1 rounded border bg-muted px-2 py-1 font-mono text-xs outline-none focus:ring-1 focus:ring-ring"
                    @focus="selectAll"
                  >
                  <Button variant="ghost" size="icon" class="size-7 shrink-0" :title="t('phone.adb.copy')" @click="copy(info?.adbAddress ?? '')">
                    <Copy class="size-3.5" />
                  </Button>
                </div>
              </div>

              <div v-if="info?.adbToken" class="flex flex-col gap-1">
                <span class="text-xs font-medium text-muted-foreground">{{ t('phone.adb.token') }}</span>
                <div class="flex items-center gap-2">
                  <input
                    :value="info.adbToken"
                    :type="showToken ? 'text' : 'password'"
                    readonly
                    class="min-w-0 flex-1 rounded border bg-muted px-2 py-1 font-mono text-xs outline-none focus:ring-1 focus:ring-ring"
                    @focus="selectAll"
                  >
                  <Button variant="ghost" size="icon" class="size-7 shrink-0" :title="showToken ? t('phone.adb.hide') : t('phone.adb.show')" @click="showToken = !showToken">
                    <Eye v-if="!showToken" class="size-3.5" />
                    <EyeOff v-else class="size-3.5" />
                  </Button>
                  <Button variant="ghost" size="icon" class="size-7 shrink-0" :title="t('phone.adb.copy')" @click="copy(info?.adbToken ?? '')">
                    <Copy class="size-3.5" />
                  </Button>
                </div>
              </div>
            </div>

            <!-- 两步连接引导 -->
            <div class="flex flex-col gap-3 rounded-md border p-3">
              <span class="text-sm font-medium">{{ t('phone.adb.connectGuide') }}</span>
              <div class="flex flex-col gap-1">
                <span class="text-xs text-muted-foreground">{{ t('phone.adb.step1') }}</span>
                <div class="flex items-center gap-2">
                  <input
                    :value="connectCmd1 || '-'"
                    readonly
                    class="min-w-0 flex-1 rounded border bg-muted px-2 py-1 font-mono text-xs outline-none focus:ring-1 focus:ring-ring"
                    @focus="selectAll"
                  >
                  <Button variant="ghost" size="icon" class="size-7 shrink-0" :title="t('phone.adb.copy')" @click="copy(connectCmd1)">
                    <Copy class="size-3.5" />
                  </Button>
                </div>
              </div>
              <div class="flex flex-col gap-1">
                <span class="text-xs text-muted-foreground">{{ t('phone.adb.step2') }}</span>
                <div class="flex items-center gap-2">
                  <input
                    :value="connectCmd2 || '-'"
                    readonly
                    class="min-w-0 flex-1 rounded border bg-muted px-2 py-1 font-mono text-xs outline-none focus:ring-1 focus:ring-ring"
                    @focus="selectAll"
                  >
                  <Button variant="ghost" size="icon" class="size-7 shrink-0" :title="t('phone.adb.copy')" @click="copy(connectCmd2)">
                    <Copy class="size-3.5" />
                  </Button>
                </div>
              </div>
            </div>
          </template>
        </div>
      </div>
    </SheetContent>
  </Sheet>
</template>
