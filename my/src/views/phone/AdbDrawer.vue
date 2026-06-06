<script setup lang="ts">
import type { AdbInfo, AdbWhitelistEntry, CloudPhone } from '@/types/phone'
import { Check, Copy, Loader2, Power, RefreshCw, TriangleAlert, Usb } from 'lucide-vue-next'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import phoneApi from '@/api/modules/phone'
import Popconfirm from '@/components/Popconfirm.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'

const props = defineProps<{ phone: CloudPhone | null }>()
const open = defineModel<boolean>({ default: false })

const { t } = useI18n()

// 中台 ADB 接口尚未调通，功能暂时不可用：禁用全部操作并提示。接口可用后改回 true。
const ADB_AVAILABLE: boolean = false

const TTL_OPTIONS = [
  { value: 86400, label: '24h' },
  { value: 604800, label: '7d' },
  { value: 2592000, label: '30d' },
]

const info = ref<AdbInfo | null>(null)
const whitelist = ref<AdbWhitelistEntry[]>([])
const loading = ref(false)
const busy = ref(false)
const ttl = ref(86400)
// 白名单编辑框：每行一个 IP。
const ipText = ref('')
const showToken = ref(false)

const enabled = computed(() => info.value?.enabled ?? false)
const connectCmd = computed(() => info.value?.adbAddress ? `adb connect ${info.value.adbAddress}` : '')

function parseIps(): string[] {
  return ipText.value
    .split('\n')
    .map(s => s.trim())
    .filter(Boolean)
}

async function load() {
  if (!props.phone)
    return
  loading.value = true
  try {
    const [i, w] = await Promise.all([
      phoneApi.adbInfo(props.phone.id),
      phoneApi.adbWhitelist(props.phone.id).catch(() => ({ data: [] as AdbWhitelistEntry[] })),
    ])
    info.value = i.data
    whitelist.value = w.data ?? []
    // 用现有白名单回填编辑框（仅未编辑过时）。
    if (!ipText.value)
      ipText.value = whitelist.value.map(e => e.ipAddress).filter(Boolean).join('\n')
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
    whitelist.value = []
    ipText.value = ''
    showToken.value = false
    // 接口未调通时不发起请求，避免无意义的失败提示。
    if (ADB_AVAILABLE)
      load()
  }
})

async function enable() {
  if (!ADB_AVAILABLE || !props.phone || busy.value)
    return
  busy.value = true
  try {
    const res = await phoneApi.adbEnable(props.phone.id, { whiteIp: parseIps(), ttl: ttl.value })
    info.value = res.data
    toast.success(t('phone.adb.enableOk'))
    await load()
  }
  catch {
    toast.error(t('phone.adb.enableFail'))
  }
  finally {
    busy.value = false
  }
}

async function disable() {
  if (!ADB_AVAILABLE || !props.phone || busy.value)
    return
  busy.value = true
  try {
    await phoneApi.adbDisable(props.phone.id)
    toast.success(t('phone.adb.disableOk'))
    await load()
  }
  catch {
    toast.error(t('phone.adb.disableFail'))
  }
  finally {
    busy.value = false
  }
}

async function updateWhitelist() {
  if (!ADB_AVAILABLE || !props.phone || busy.value)
    return
  busy.value = true
  try {
    await phoneApi.adbUpdateWhitelist(props.phone.id, parseIps())
    toast.success(t('phone.adb.whitelistOk'))
    await load()
  }
  catch {
    toast.error(t('phone.adb.whitelistFail'))
  }
  finally {
    busy.value = false
  }
}

async function copy(text: string) {
  if (!text)
    return
  try {
    await navigator.clipboard.writeText(text)
    toast.success(t('phone.adb.copied'))
  }
  catch {
    toast.error(t('phone.adb.copyFail'))
  }
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
          <!-- 中台接口未调通：危险色警告，功能暂时不可用 -->
          <div
            v-if="!ADB_AVAILABLE"
            class="flex items-start gap-2 rounded-md border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive"
          >
            <TriangleAlert class="mt-0.5 size-4 shrink-0" />
            <span>{{ t('phone.adb.unavailable') }}</span>
          </div>

          <!-- 状态 + 开关 -->
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
              <Button variant="outline" size="sm" class="gap-1 border-amber-500 text-amber-600 dark:text-amber-400" :disabled="busy || !ADB_AVAILABLE">
                <Power class="size-3.5" /> {{ t('phone.adb.disable') }}
              </Button>
            </Popconfirm>
          </div>

          <!-- 连接信息（已开启） -->
          <div v-if="enabled" class="flex flex-col gap-3 rounded-md border p-3">
            <div class="flex flex-col gap-1">
              <span class="text-xs font-medium text-muted-foreground">{{ t('phone.adb.address') }}</span>
              <div class="flex items-center gap-2">
                <code class="min-w-0 flex-1 truncate rounded bg-muted px-2 py-1 text-xs">{{ info?.adbAddress || '-' }}</code>
                <Button variant="ghost" size="icon" class="size-7 shrink-0" @click="copy(info?.adbAddress ?? '')">
                  <Copy class="size-3.5" />
                </Button>
              </div>
            </div>

            <div class="flex flex-col gap-1">
              <span class="text-xs font-medium text-muted-foreground">{{ t('phone.adb.command') }}</span>
              <div class="flex items-center gap-2">
                <code class="min-w-0 flex-1 truncate rounded bg-muted px-2 py-1 text-xs">{{ connectCmd || '-' }}</code>
                <Button variant="ghost" size="icon" class="size-7 shrink-0" @click="copy(connectCmd)">
                  <Copy class="size-3.5" />
                </Button>
              </div>
            </div>

            <div v-if="info?.adbToken" class="flex flex-col gap-1">
              <span class="text-xs font-medium text-muted-foreground">{{ t('phone.adb.token') }}</span>
              <div class="flex items-center gap-2">
                <code class="min-w-0 flex-1 truncate rounded bg-muted px-2 py-1 text-xs">
                  {{ showToken ? info.adbToken : '••••••••••••' }}
                </code>
                <Button variant="ghost" size="sm" class="h-7 shrink-0 px-2 text-xs" @click="showToken = !showToken">
                  {{ showToken ? t('phone.adb.hide') : t('phone.adb.show') }}
                </Button>
                <Button variant="ghost" size="icon" class="size-7 shrink-0" @click="copy(info?.adbToken ?? '')">
                  <Copy class="size-3.5" />
                </Button>
              </div>
            </div>

            <div v-if="info?.adbTokenExpiredAt" class="text-xs text-muted-foreground">
              {{ t('phone.adb.expireAt', { time: info.adbTokenExpiredAt }) }}
            </div>
          </div>

          <!-- 白名单 IP -->
          <div class="flex flex-col gap-2 rounded-md border p-3">
            <div class="flex items-center justify-between">
              <span class="text-sm font-medium">{{ t('phone.adb.whitelist') }}</span>
              <span class="text-xs text-muted-foreground">{{ t('phone.adb.whitelistHint') }}</span>
            </div>
            <textarea
              v-model="ipText"
              :placeholder="t('phone.adb.ipPlaceholder')"
              rows="3"
              class="w-full resize-y rounded-md border bg-transparent px-2 py-1.5 font-mono text-xs outline-none focus:ring-1 focus:ring-ring"
            />
            <Button v-if="enabled" variant="outline" size="sm" class="self-start gap-1" :disabled="busy || !ADB_AVAILABLE" @click="updateWhitelist">
              <RefreshCw class="size-3.5" :class="busy && 'animate-spin'" /> {{ t('phone.adb.applyWhitelist') }}
            </Button>
          </div>

          <!-- 开启（未开启时）：TTL + 开启按钮 -->
          <div v-if="!enabled" class="flex items-end gap-2 rounded-md border p-3">
            <div class="flex flex-col gap-1">
              <span class="text-xs font-medium text-muted-foreground">{{ t('phone.adb.ttl') }}</span>
              <NativeSelect v-model.number="ttl" class="h-9 w-28">
                <NativeSelectOption v-for="o in TTL_OPTIONS" :key="o.value" :value="o.value">
                  {{ o.label }}
                </NativeSelectOption>
              </NativeSelect>
            </div>
            <Button class="flex-1 gap-1" :disabled="busy || !ADB_AVAILABLE" @click="enable">
              <Loader2 v-if="busy" class="size-4 animate-spin" />
              <Check v-else class="size-4" />
              {{ t('phone.adb.enable') }}
            </Button>
          </div>
        </div>
      </div>
    </SheetContent>
  </Sheet>
</template>
