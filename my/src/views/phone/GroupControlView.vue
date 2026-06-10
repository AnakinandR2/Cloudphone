<script setup lang="ts">
import type { CloudPhone } from '@/types/phone'
import {
  AppWindow,
  Camera,
  ChevronLeft,
  Circle,
  CloudUpload,
  FolderOpen,
  Keyboard,
  Maximize,
  Monitor,
  MoreHorizontal,
  Power,
  RotateCw,
  SignalHigh,
  Square,
  Video,
  Volume1,
  Volume2,
  VolumeX,
} from 'lucide-vue-next'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { toast } from 'vue-sonner'
import phoneApi from '@/api/modules/phone'
import Popconfirm from '@/components/Popconfirm.vue'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import GroupPhoneCell from './GroupPhoneCell.vue'
import RemoteAppPanel from './RemoteAppPanel.vue'
import RemoteFilePanel from './RemoteFilePanel.vue'
import RemoteUploadPanel from './RemoteUploadPanel.vue'

// 串流选项（与 useWebRTC 一致；群控「显示」统一下发到全部）。
const RES_OPTIONS = [{ label: '360x640' }, { label: '720x1280' }, { label: '1080x1920' }]
const QUALITY_OPTIONS = [{ value: 50, label: '高清' }, { value: 30, label: '标清' }, { value: 10, label: '流畅' }]
const FPS_OPTIONS = Array.from({ length: 11 }, (_, i) => 10 + i * 5)

const { t } = useI18n()
const route = useRoute()

const ids = computed<number[]>(() =>
  String(route.query.ids ?? '')
    .split(',')
    .map(s => Number(s.trim()))
    .filter(n => Number.isFinite(n) && n > 0),
)

const phones = ref<{ id: number, name: string }[]>([])
const masterIdx = ref(0)
const masterPhoneId = computed(() => phones.value[masterIdx.value]?.id ?? 0)
const masterName = computed(() => phones.value[masterIdx.value]?.name ?? '')
const groupMuted = ref(true)
const netLatency = 37

// 展示缩放（cell 像素宽，slider 50%~200% 映射到 80~320px）+ 主控放大模式。
const zoom = ref(100)
const masterFocus = ref(true)
const cellSize = computed(() => Math.round(160 * zoom.value / 100))

// 参与群控的云机集合（每台左上角勾选；默认全选）。仅选中的接收群控指令。
const selectedCells = ref<Set<number>>(new Set())
const selectedCount = computed(() => selectedCells.value.size)
const allCellsSelected = computed(() => phones.value.length > 0 && selectedCount.value === phones.value.length)
function toggleCell(id: number) {
  const next = new Set(selectedCells.value)
  if (next.has(id))
    next.delete(id)
  else next.add(id)
  selectedCells.value = next
}
function toggleAllCells() {
  selectedCells.value = allCellsSelected.value ? new Set() : new Set(phones.value.map(p => p.id))
}
const selectedPhones = computed(() => phones.value.filter(p => selectedCells.value.has(p.id)))

type Panel = 'display' | 'keys' | 'files' | 'apps' | 'upload' | null
const activePanel = ref<Panel>(null)
function togglePanel(name: Exclude<Panel, null>) {
  activePanel.value = activePanel.value === name ? null : name
}

const groupRes = ref('720x1280')
const groupQuality = ref(30)
const groupFps = ref(30)

const cellEls = ref<InstanceType<typeof GroupPhoneCell>[]>([])
function setCell(i: number, el: unknown) {
  if (el)
    cellEls.value[i] = el as InstanceType<typeof GroupPhoneCell>
}
function eachCell(fn: (c: InstanceType<typeof GroupPhoneCell>) => void) {
  for (let i = 0; i < cellEls.value.length; i++) {
    const c = cellEls.value[i]
    const ph = phones.value[i]
    if (c && ph && selectedCells.value.has(ph.id))
      fn(c)
  }
}

function onPointer(p: { kind: 'down' | 'move' | 'up', nx: number, ny: number, button: number }) {
  eachCell(c => c.applyPointer(p.kind, p.nx, p.ny, p.button))
}
function groupButton(btn: string) {
  eachCell(c => c.sendButton(btn))
}
function groupMute() {
  groupMuted.value = !groupMuted.value
  eachCell(c => c.setMute(groupMuted.value))
}
function setMaster(i: number) {
  masterIdx.value = i
}
function comingSoon() {
  toast.info(t('phone.rc.comingSoon'))
}

function applyStream() {
  eachCell(c => c.setStream(groupRes.value, groupQuality.value, groupFps.value))
}
function groupFullscreen() {
  document.documentElement.requestFullscreen?.().catch(() => {})
}

// 仅截主控当前帧并下载。
async function shotMaster() {
  const cell = cellEls.value[masterIdx.value]
  if (!cell)
    return
  const blob = await cell.captureShot()
  if (!blob)
    return
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${masterName.value || 'master'}-${Date.now()}.png`
  a.click()
  URL.revokeObjectURL(url)
}

// 截屏：逐台截当前帧依次下载（无 zip 依赖）。
async function groupScreenshot() {
  let n = 0
  for (let i = 0; i < cellEls.value.length; i++) {
    const cell = cellEls.value[i]
    const ph = phones.value[i]
    if (!cell || !ph || !selectedCells.value.has(ph.id))
      continue
    const blob = await cell.captureShot()
    if (!blob)
      continue
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${phones.value[i]?.name ?? `phone-${i}`}-${Date.now()}.png`
    a.click()
    URL.revokeObjectURL(url)
    n++
    await new Promise(r => setTimeout(r, 150))
  }
  if (n)
    toast.success(t('phone.group.shotAllOk', { n }))
}

async function groupNewDevice() {
  const targets = selectedPhones.value
  let fail = 0
  for (const p of targets) {
    try {
      await phoneApi.newDevice(p.id)
    }
    catch {
      fail++
    }
  }
  if (fail)
    toast.error(t('phone.rc.newDeviceFail'), { description: `${fail}/${targets.length}` })
  else
    toast.success(t('phone.rc.newDeviceOk'))
}

function groupRetry() {
  eachCell(c => c.retry())
}

onMounted(async () => {
  document.title = t('phone.group.title')
  try {
    const res = await phoneApi.list({ page: 1, size: 999 })
    const all = res.data.list as CloudPhone[]
    const map = new Map(all.map(p => [p.id, p.name]))
    phones.value = ids.value.map(id => ({ id, name: map.get(id) ?? `#${id}` }))
  }
  catch {
    phones.value = ids.value.map(id => ({ id, name: `#${id}` }))
  }
  selectedCells.value = new Set(phones.value.map(p => p.id))
})
</script>

<template>
  <div class="flex h-screen flex-col overflow-hidden bg-muted/30">
    <div class="flex items-center gap-2 border-b bg-background px-3 py-1.5 text-xs text-muted-foreground">
      <span class="font-medium text-foreground">{{ t('phone.group.title') }}</span>
      <span class="truncate">{{ t('phone.group.masterHint') }}</span>
      <div class="ml-auto flex items-center gap-3">
        <div class="flex items-center gap-1.5">
          <span>{{ t('phone.group.zoom') }}</span>
          <input v-model.number="zoom" type="range" min="50" max="200" step="10" class="w-24 accent-primary">
          <span class="w-8 tabular-nums">{{ zoom }}%</span>
        </div>
        <label class="flex cursor-pointer items-center gap-1.5">
          <input v-model="masterFocus" type="checkbox" class="size-3.5 accent-primary">
          {{ t('phone.group.masterFocus') }}
        </label>
        <Button size="sm" variant="outline" class="h-7 text-xs" @click="toggleAllCells">
          {{ allCellsSelected ? t('phone.group.deselectAll') : t('phone.group.selectAll') }}
        </Button>
        <span class="tabular-nums">{{ t('phone.group.selectedCount', { n: selectedCount, total: phones.length }) }}</span>
      </div>
    </div>

    <div class="flex min-h-0 flex-1">
      <div class="min-h-0 flex-1 overflow-auto p-2">
        <div class="grid gap-2" :style="{ gridTemplateColumns: `repeat(auto-fill, minmax(${cellSize}px, 1fr))` }">
          <GroupPhoneCell
            v-for="(p, i) in phones"
            :ref="(el) => setCell(i, el)"
            :key="p.id"
            :phone-id="p.id"
            :name="p.name"
            :master="i === masterIdx"
            :style="masterFocus && i === masterIdx ? { gridColumn: 'span 2', gridRow: 'span 2', order: -1 } : undefined"
            :selected="selectedCells.has(p.id)"
            @toggle-selected="toggleCell(p.id)"
            @pointer="onPointer"
            @select-master="setMaster(i)"
          />
        </div>
      </div>

      <!-- 显示 -->
      <div v-if="activePanel === 'display'" class="flex w-56 shrink-0 flex-col gap-4 overflow-y-auto border-l bg-background p-4">
        <div class="text-xs text-muted-foreground">
          {{ t('phone.group.applyAll') }}
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs font-medium text-muted-foreground">{{ t('phone.rc.resolution') }}</label>
          <NativeSelect v-model="groupRes" class="h-9 w-full" @update:model-value="applyStream">
            <NativeSelectOption v-for="r in RES_OPTIONS" :key="r.label" :value="r.label">
              {{ r.label }}
            </NativeSelectOption>
          </NativeSelect>
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs font-medium text-muted-foreground">{{ t('phone.rc.quality') }}</label>
          <NativeSelect v-model.number="groupQuality" class="h-9 w-full" @update:model-value="applyStream">
            <NativeSelectOption v-for="q in QUALITY_OPTIONS" :key="q.value" :value="q.value">
              {{ q.label }}
            </NativeSelectOption>
          </NativeSelect>
        </div>
        <div class="flex flex-col gap-1.5">
          <label class="text-xs font-medium text-muted-foreground">{{ t('phone.rc.fps') }}</label>
          <NativeSelect v-model.number="groupFps" class="h-9 w-full" @update:model-value="applyStream">
            <NativeSelectOption v-for="f in FPS_OPTIONS" :key="f" :value="f">
              {{ f }}fps
            </NativeSelectOption>
          </NativeSelect>
        </div>
        <Button variant="outline" class="w-full justify-start gap-2" @click="groupFullscreen">
          <Maximize class="size-4" />
          {{ t('phone.rc.fullscreen') }}
        </Button>
      </div>

      <!-- 按键（全体） -->
      <div v-if="activePanel === 'keys'" class="flex w-56 shrink-0 flex-col content-start gap-5 overflow-y-auto border-l bg-background p-4">
        <div class="flex items-start justify-around gap-2">
          <div class="flex flex-col items-center gap-1.5">
            <Button :variant="groupMuted ? 'secondary' : 'outline'" class="size-12 rounded-full p-0" @click="groupMute">
              <component :is="groupMuted ? VolumeX : Volume2" class="size-5" />
            </Button>
            <span class="text-xs">{{ groupMuted ? t('phone.rc.unmute') : t('phone.rc.mute') }}</span>
          </div>
          <div class="flex flex-col items-center gap-1.5">
            <Button variant="outline" class="size-12 rounded-full p-0" @click="groupButton('volume_up')">
              <Volume2 class="size-5" />
            </Button>
            <span class="text-xs">{{ t('phone.rc.btnVolUp') }}</span>
          </div>
          <div class="flex flex-col items-center gap-1.5">
            <Button variant="outline" class="size-12 rounded-full p-0" @click="groupButton('volume_down')">
              <Volume1 class="size-5" />
            </Button>
            <span class="text-xs">{{ t('phone.rc.btnVolDown') }}</span>
          </div>
        </div>
        <div class="flex items-start justify-around gap-2">
          <div class="flex flex-col items-center gap-1.5">
            <Button variant="outline" class="size-12 rounded-full p-0" @click="groupButton('power')">
              <Power class="size-5" />
            </Button>
            <span class="text-xs">{{ t('phone.rc.btnPower') }}</span>
          </div>
        </div>
        <div class="flex items-start justify-around gap-2">
          <div class="flex flex-col items-center gap-1.5">
            <Button variant="outline" class="size-12 rounded-full p-0" @click="groupButton('back')">
              <ChevronLeft class="size-5" />
            </Button>
            <span class="text-xs">{{ t('phone.rc.btnBack') }}</span>
          </div>
          <div class="flex flex-col items-center gap-1.5">
            <Button variant="outline" class="size-12 rounded-full p-0" @click="groupButton('home')">
              <Circle class="size-5" />
            </Button>
            <span class="text-xs">{{ t('phone.rc.btnHome') }}</span>
          </div>
          <div class="flex flex-col items-center gap-1.5">
            <Button variant="outline" class="size-12 rounded-full p-0" @click="groupButton('recent')">
              <Square class="size-5" />
            </Button>
            <span class="text-xs">{{ t('phone.rc.btnRecent') }}</span>
          </div>
        </div>
      </div>

      <!-- 文件（展示主控；文件浏览/下载/删除；上传见上传面板） -->
      <div v-if="activePanel === 'files'" class="w-[380px] shrink-0 overflow-hidden border-l bg-background">
        <RemoteFilePanel :key="masterPhoneId" :phone-id="masterPhoneId" :phones="selectedPhones" />
      </div>

      <!-- 应用（全体安装，列表读主控） -->
      <div v-if="activePanel === 'apps'" class="flex w-[420px] shrink-0 flex-col overflow-hidden border-l bg-background">
        <RemoteAppPanel :phone-ids="phones.map(p => p.id)" :master-id="masterPhoneId" />
      </div>

      <!-- 上传（固定 /sdcard/Download，可选主控/群控范围） -->
      <div v-if="activePanel === 'upload'" class="w-[380px] shrink-0 overflow-hidden border-l bg-background">
        <RemoteUploadPanel :key="masterPhoneId" :phone-id="masterPhoneId" :phones="selectedPhones" />
      </div>

      <!-- 侧栏 -->
      <TooltipProvider :delay-duration="150">
        <div class="flex w-16 shrink-0 flex-col gap-0.5 overflow-y-auto border-l bg-background p-1">
          <Tooltip>
            <TooltipTrigger as-child>
              <Button variant="ghost" class="h-auto flex-col gap-1 px-0.5 py-1.5">
                <SignalHigh class="size-4 text-green-500" />
                <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.net') }}</span>
              </Button>
            </TooltipTrigger>
            <TooltipContent side="left" class="max-w-[220px] text-xs">
              {{ t('phone.group.netMasterOnly', { ms: netLatency }) }}
            </TooltipContent>
          </Tooltip>

          <Button :variant="activePanel === 'display' ? 'secondary' : 'ghost'" class="h-auto flex-col gap-1 px-0.5 py-1.5" @click="togglePanel('display')">
            <Monitor class="size-4" />
            <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.display') }}</span>
          </Button>
          <Button :variant="activePanel === 'keys' ? 'secondary' : 'ghost'" class="h-auto flex-col gap-1 px-0.5 py-1.5" @click="togglePanel('keys')">
            <Keyboard class="size-4" />
            <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.keys') }}</span>
          </Button>
          <Button :variant="activePanel === 'files' ? 'secondary' : 'ghost'" class="h-auto flex-col gap-1 px-0.5 py-1.5" @click="togglePanel('files')">
            <FolderOpen class="size-4" />
            <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.files') }}</span>
          </Button>
          <Button :variant="activePanel === 'upload' ? 'secondary' : 'ghost'" class="h-auto flex-col gap-1 px-0.5 py-1.5" @click="togglePanel('upload')">
            <CloudUpload class="size-4" />
            <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.panelUpload') }}</span>
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" class="h-auto flex-col gap-1 px-0.5 py-1.5">
                <Camera class="size-4" />
                <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.screenshot') }}</span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" side="left" class="w-28">
              <DropdownMenuItem @click="shotMaster">
                {{ t('phone.rc.scopeMaster') }}
              </DropdownMenuItem>
              <DropdownMenuItem @click="groupScreenshot">
                {{ t('phone.rc.scopeGroup', { n: selectedCount }) }}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <Button variant="ghost" class="h-auto flex-col gap-1 px-0.5 py-1.5" @click="comingSoon">
            <RotateCw class="size-4" />
            <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.rotate') }}</span>
          </Button>
          <Button variant="ghost" class="h-auto flex-col gap-1 px-0.5 py-1.5" @click="comingSoon">
            <Video class="size-4" />
            <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.live') }}</span>
          </Button>
          <Button :variant="activePanel === 'apps' ? 'secondary' : 'ghost'" class="h-auto flex-col gap-1 px-0.5 py-1.5" @click="togglePanel('apps')">
            <AppWindow class="size-4" />
            <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.apps') }}</span>
          </Button>

          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="ghost" class="h-auto flex-col gap-1 px-0.5 py-1.5">
                <MoreHorizontal class="size-4" />
                <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.more') }}</span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" side="left" class="w-32">
              <DropdownMenuItem @click="groupRetry">
                {{ t('phone.rc.reconnect') }}
              </DropdownMenuItem>
              <Popconfirm tone="danger" :title="t('phone.group.newDeviceConfirm', { n: phones.length })" @confirm="groupNewDevice">
                <DropdownMenuItem @select.prevent>
                  {{ t('phone.rc.newDevice') }}
                </DropdownMenuItem>
              </Popconfirm>
              <DropdownMenuItem @click="comingSoon">
                {{ t('phone.rc.clean') }}
              </DropdownMenuItem>
              <DropdownMenuItem @click="comingSoon">
                {{ t('phone.rc.accelerate') }}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </TooltipProvider>
    </div>
  </div>
</template>
