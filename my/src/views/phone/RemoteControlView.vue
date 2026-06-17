<script setup lang="ts">
import type { CloudPhone } from '@/types/phone'
import {
  AppWindow,
  Camera,
  ChevronLeft,
  Circle,
  Clock,
  CloudUpload,
  Download,
  Eraser,
  FolderOpen,
  Keyboard,
  Maximize,
  Mic,
  Monitor,
  MoreHorizontal,
  Power,
  Radio,
  RefreshCw,
  Rocket,
  RotateCw,
  Search,
  SignalHigh,
  Smartphone,
  Square,
  Video,
  Volume1,
  Volume2,
  VolumeX,
  X,
} from 'lucide-vue-next'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'

import { toast } from 'vue-sonner'
import phoneApi from '@/api/modules/phone'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Switch } from '@/components/ui/switch'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useRemoteInput } from '@/composables/useRemoteInput'
import { useWebRTC } from '@/composables/useWebRTC'
import RemoteAppPanel from './RemoteAppPanel.vue'
import RemoteFilePanel from './RemoteFilePanel.vue'
import RemoteUploadPanel from './RemoteUploadPanel.vue'
import { computeRemoteWindowSize } from './remoteWindowFit'

const { t } = useI18n()
const route = useRoute()

const id = computed(() => Number(route.params.id))
const phone = ref<CloudPhone | null>(null)
const videoRef = ref<HTMLVideoElement | null>(null)
const frameRef = ref<HTMLElement | null>(null)

const {
  connState,
  connStatusText,
  connected,
  controlReady,
  landscape,
  rotateDevice,
  rttMs,
  latencyLevel,
  videoInjecting,
  audioInjecting,
  videoInjectBusy,
  audioInjectBusy,
  videoInputs,
  audioInputs,
  selectedVideoId,
  selectedAudioId,
  mirrorEnabled,
  cameraPreviewStream,
  refreshMediaDevices,
  toggleVideoInject,
  toggleAudioInject,
  setVideoDevice,
  setAudioDevice,
  setMirror,
  isMuted,
  resOptions,
  qualityOptions,
  fpsOptions,
  selectedRes,
  selectedQuality,
  selectedFps,
  deviceWidth,
  deviceHeight,
  connect,
  retry,
  cleanup,
  toggleMute,
  tryAutoUnmute,
  sendDC,
} = useWebRTC({ id, videoRef })

const input = useRemoteInput({ videoRef, connState, deviceWidth, deviceHeight, sendDC, tryAutoUnmute })

// 画面比例优先用真实串流尺寸（避免与请求分辨率不一致导致 object-contain 黑边）。
const streamW = ref(0)
const streamH = ref(0)
const aspect = computed(() => {
  if (streamW.value && streamH.value)
    return `${streamW.value}/${streamH.value}`
  return `${deviceWidth.value}/${deviceHeight.value}`
})
// 横屏：实际推流宽>高（设备旋转后推流分辨率交换）。横屏时画面按可用区域等比收纳，避免撑爆窗口。
const isLandscape = computed(() => streamW.value > 0 && streamW.value > streamH.value)

// 标题写进 html title（替代原第一行），并带上连接状态（如「控制就绪」）。
const docTitle = computed(() => {
  const base = phone.value ? `${t('phone.rc.title')} — ${phone.value.name}` : t('phone.rc.title')
  return `${base} · ${connStatusText.value}`
})
watch(docTitle, (v) => {
  document.title = v
}, { immediate: true })

// 改分辨率/质量/帧率：已连接则重连生效。
function onSettingChange() {
  if (connState.value === 'connected' || connState.value === 'connecting')
    retry()
}

// 旋转设备：发 rotate_device 指令，设备旋转后推流分辨率交换，由 onVideoReady 自动重排窗口。
function onRotate() {
  if (!rotateDevice())
    toast.error(t('phone.rc.rotateFail'))
}

function fullscreen() {
  frameRef.value?.requestFullscreen?.().catch(() => {})
}

// 「更多」相关动作均暂未实现具体功能，确认后仅提示开发中。
function comingSoon() {
  toast.info(t('phone.rc.comingSoon'))
}

// 受控确认弹框：清理 / 加速点击后先二次确认，确认再执行 run。
const confirmState = ref<{ open: boolean, title: string, desc: string, run: () => void }>({
  open: false,
  title: '',
  desc: '',
  run: () => {},
})
function ask(title: string, desc: string, run: () => void) {
  confirmState.value = { open: true, title, desc, run }
}

// 检测 IP：点击后将跳转到 IP 检测网站（开发中）。
function detectIp() {
  toast.info(t('phone.rc.detectIpSoon'))
}

// 加速：确认将关闭所有 APP 进程。
function boost() {
  ask(t('phone.rc.accelerate'), t('phone.rc.accelerateConfirm'), comingSoon)
}

// 清理：确认清理 sdcard 文件与上传的文件。
function clean() {
  ask(t('phone.rc.clean'), t('phone.rc.cleanConfirm'), comingSoon)
}

// 一键新机：擦数据 + 重装系统（保留代理）。
function newDevice() {
  ask(t('phone.rc.newDevice'), t('phone.rc.newDeviceConfirm'), async () => {
    try {
      await phoneApi.newDevice(id.value)
      toast.success(t('phone.rc.newDeviceOk'))
    }
    catch { /* 拦截器已提示 */ }
  })
}

// 截图后在画面右上角显示缩略图，点「下载」才真正触发下载。
const shotUrl = ref<string | null>(null)
function screenshot() {
  const v = videoRef.value
  if (!v || !v.videoWidth || !v.videoHeight)
    return
  const canvas = document.createElement('canvas')
  canvas.width = v.videoWidth
  canvas.height = v.videoHeight
  const ctx = canvas.getContext('2d')
  if (!ctx)
    return
  ctx.drawImage(v, 0, 0, canvas.width, canvas.height)
  canvas.toBlob((blob) => {
    if (!blob)
      return
    if (shotUrl.value)
      URL.revokeObjectURL(shotUrl.value)
    shotUrl.value = URL.createObjectURL(blob)
  }, 'image/png')
}
function downloadShot() {
  if (!shotUrl.value)
    return
  const a = document.createElement('a')
  a.href = shotUrl.value
  a.download = `${phone.value?.name ?? `phone-${id.value}`}-${Date.now()}.png`
  a.click()
}
function closeShot() {
  if (shotUrl.value)
    URL.revokeObjectURL(shotUrl.value)
  shotUrl.value = null
}

// 侧栏展开面板：display=显示参数 / keys=按键 / files=文件管理 / apps=应用 / upload=上传 / camera=摄像头注入。
type Panel = 'display' | 'keys' | 'files' | 'apps' | 'upload' | 'camera' | null
const activePanel = ref<Panel>(null)

// 布局常量（与模板里的宽度保持一致；无 padding / gap，元素直接相贴）。
const SIDEBAR_W = 56 // w-14

// 各展开面板的宽度（与模板里对应面板的 w-* 一致；不同菜单可设不同宽度）。
function panelWidth(p: Panel): number {
  switch (p) {
    case 'display':
      return 224 // w-56
    case 'keys':
      return 224 // w-56
    case 'files':
      return 380 // w-[380px]
    case 'apps':
      return 380 // w-[380px]
    case 'upload':
      return 380 // w-[380px]
    case 'camera':
      return 280 // w-[280px]
    default:
      return 0
  }
}

// 设备长边映射到屏上的目标长度（与 PhoneView.openRemoteControl 的初始高一致）。
const VIDEO_LONG = (() => {
  const avail = window.screen?.availHeight ?? 960
  return Math.min(960, Math.max(560, Math.round(avail * 0.88)))
})()

// 按实时推流比例把弹窗 resize 成对应形态：竖屏=窄高、横屏=宽扁 → 实现「窗体旋转」。
// 设备/应用自动转横屏时推流分辨率交换，由 onVideoReady(@resize) 触发本函数重排窗体。
function fitWindowToOrientation(panel: Panel) {
  if (typeof window.resizeTo !== 'function')
    return
  const { outerW, outerH } = computeRemoteWindowSize({
    streamW: streamW.value,
    streamH: streamH.value,
    panelW: panelWidth(panel),
    sidebarW: SIDEBAR_W,
    longEdge: VIDEO_LONG,
    availW: window.screen?.availWidth ?? 99999,
    availH: window.screen?.availHeight ?? 99999,
    chromeW: Math.max(0, window.outerWidth - window.innerWidth),
    chromeH: Math.max(0, window.outerHeight - window.innerHeight),
  })
  window.resizeTo(outerW, outerH)
}

// 画面就绪 / 方向变化时按当前展开的面板贴合窗体。
function fitWindow() {
  fitWindowToOrientation(activePanel.value)
}

// 开关面板：
//  - 再次点当前项 → 关闭并恢复正常尺寸；
//  - 切换到别的项 / 从无到有 → 先收起旧内容、按目标面板重算窗口宽度，待尺寸稳定后再显示新内容。
function togglePanel(name: Exclude<Panel, null>) {
  if (activePanel.value === name) {
    activePanel.value = null
    fitWindowToOrientation(null)
    return
  }
  activePanel.value = null
  fitWindowToOrientation(name)
  activePanel.value = name
}

// ---- 摄像头/麦克风注入（直播）：视频与音频相互独立 ----
const cameraPreviewRef = ref<HTMLVideoElement | null>(null)
watch(cameraPreviewStream, (s) => {
  if (cameraPreviewRef.value)
    cameraPreviewRef.value.srcObject = s ?? null
})
// 打开面板时刷新设备列表。
watch(activePanel, (p) => {
  if (p === 'camera')
    refreshMediaDevices()
})
// 可注入：已连接且控制通道就绪。
const canInject = computed(() => connected.value && controlReady.value)

function mapCameraError(e: any): string {
  if (typeof window !== 'undefined' && window.isSecureContext === false)
    return t('phone.rc.cameraErrInsecure')
  switch (e?.name) {
    case 'NotAllowedError': return t('phone.rc.cameraErrDenied')
    case 'NotFoundError':
    case 'OverconstrainedError': return t('phone.rc.cameraErrNoDevice')
    case 'InvalidStateError': return t('phone.rc.cameraErrNotReady')
    default: return t('phone.rc.cameraErrGeneric')
  }
}
async function onToggleVideo() {
  try {
    await toggleVideoInject()
    toast.success(videoInjecting.value ? t('phone.rc.cameraOn') : t('phone.rc.cameraOff'))
  }
  catch (e) {
    toast.error(mapCameraError(e))
  }
}
async function onToggleAudio() {
  try {
    await toggleAudioInject()
    toast.success(audioInjecting.value ? t('phone.rc.micOn') : t('phone.rc.micOff'))
  }
  catch (e) {
    toast.error(mapCameraError(e))
  }
}
async function onToggleMirror(on: boolean) {
  try { await setMirror(on) }
  catch (e) { toast.error(mapCameraError(e)) }
}
// 设备下拉用可写 computed 桥接（set 里切换设备并 toast 错误）。
const videoDeviceModel = computed({
  get: () => selectedVideoId.value,
  set: (v: string) => { setVideoDevice(v).catch(e => toast.error(mapCameraError(e))) },
})
const audioDeviceModel = computed({
  get: () => selectedAudioId.value,
  set: (v: string) => { setAudioDevice(v).catch(e => toast.error(mapCameraError(e))) },
})

// 画面尺寸就绪 / 变化时：记录真实串流比例并（等比例应用到 DOM 后）贴合窗口。
async function onVideoReady() {
  const v = videoRef.value
  if (v?.videoWidth && v?.videoHeight) {
    streamW.value = v.videoWidth
    streamH.value = v.videoHeight
  }
  await nextTick()
  fitWindow()
}

// 网络状况：来自 WebRTC getStats() 的真实 RTT（latencyLevel 三档）。
const netColorClass = computed(() => {
  if (!connected.value)
    return 'text-muted-foreground'
  switch (latencyLevel.value) {
    case 'good': return 'text-green-500'
    case 'fair': return 'text-amber-500'
    case 'poor': return 'text-red-500'
    default: return 'text-muted-foreground'
  }
})
const netTipText = computed(() => {
  if (!connected.value)
    return t('phone.rc.netOffline')
  if (rttMs.value == null)
    return t('phone.rc.netMeasuring')
  const q = latencyLevel.value === 'good'
    ? t('phone.rc.netGood')
    : latencyLevel.value === 'fair' ? t('phone.rc.netFair') : t('phone.rc.netPoor')
  return `${q} · ${t('phone.rc.netRtt', { ms: rttMs.value })}`
})

// 本次使用计时（前端模拟，尚未接入计费）。
const elapsed = ref(0)
let timerHandle: ReturnType<typeof setInterval> | null = null
const elapsedText = computed(() => {
  const s = elapsed.value
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(Math.floor(s / 3600))}:${pad(Math.floor((s % 3600) / 60))}:${pad(s % 60)}`
})

onMounted(async () => {
  window.addEventListener('mouseup', input.onGlobalMouseUp)
  timerHandle = setInterval(() => {
    elapsed.value++
  }, 1000)
  await nextTick()
  // 初始就按画面实际宽度贴合窗口一次（不必等连接）；后续画面就绪/改分辨率时由 onVideoReady 再贴合。
  fitWindow()
  try {
    const res = await phoneApi.detail(id.value)
    phone.value = res.data
  }
  catch { /* ignore */ }
  connect()
})

onBeforeUnmount(() => {
  if (timerHandle)
    clearInterval(timerHandle)
  if (shotUrl.value)
    URL.revokeObjectURL(shotUrl.value)
  window.removeEventListener('mouseup', input.onGlobalMouseUp)
  cleanup()
})
</script>

<template>
  <div class="flex h-screen flex-col overflow-hidden">
    <!-- 主体：手机画面（左）+ 图标操作列（中）+ 展开面板（右），无边距，元素相贴 -->
    <div class="flex min-h-0 flex-1 items-stretch">
      <div
        ref="frameRef"
        class="relative flex h-full min-h-0 items-center justify-center"
        :class="isLandscape ? 'min-w-0 flex-1' : 'shrink-0'"
      >
        <video
          ref="videoRef"
          class="touch-none select-none bg-black object-contain"
          :class="isLandscape ? 'max-h-full max-w-full' : 'h-full w-auto max-w-full'"
          :style="{ aspectRatio: aspect }"
          autoplay
          playsinline
          muted
          @loadedmetadata="onVideoReady"
          @resize="onVideoReady"
          @mousedown="input.onMouseDown"
          @mousemove="input.onMouseMove"
          @mouseup="input.onMouseUp"
          @touchstart.prevent="input.onTouchStart"
          @touchmove.prevent="input.onTouchMove"
          @touchend.prevent="input.onTouchEnd"
        />
        <div
          v-if="!connected"
          class="absolute inset-0 flex flex-col items-center justify-center gap-3 bg-black/70 text-center text-sm text-white"
        >
          <p>{{ connStatusText }}</p>
          <Button v-if="connState !== 'connecting'" size="sm" @click="connect">
            {{ t('phone.rc.connect') }}
          </Button>
          <RefreshCw v-else class="size-6 animate-spin opacity-80" />
        </div>

        <!-- 截图缩略图：右上角，点「下载」才触发下载 -->
        <div v-if="shotUrl" class="absolute right-2 top-2 z-20 w-28 overflow-hidden rounded-md border-2 border-white/80 bg-black/80 shadow-lg">
          <img :src="shotUrl" class="block w-full" alt="">
          <div class="flex items-center gap-1 p-1">
            <Button size="sm" class="h-6 flex-1 gap-1 px-1 text-[10px]" @click="downloadShot">
              <Download class="size-3" /> {{ t('phone.rc.download') }}
            </Button>
            <Button size="icon" variant="ghost" class="size-6 shrink-0 text-white hover:bg-white/20 hover:text-white" @click="closeShot">
              <X class="size-3" />
            </Button>
          </div>
        </div>
      </div>

      <!-- 图标 + 文字操作列（尽量窄） -->
      <div class="flex w-14 shrink-0 flex-col gap-0.5 overflow-y-auto">
        <TooltipProvider :delay-duration="150">
          <Tooltip>
            <TooltipTrigger as-child>
              <Button variant="ghost" class="h-auto flex-col gap-1 px-0.5 py-1.5">
                <SignalHigh class="size-4" :class="netColorClass" />
                <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.net') }}</span>
              </Button>
            </TooltipTrigger>
            <TooltipContent side="left" class="max-w-[220px] text-xs">
              {{ netTipText }}
            </TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger as-child>
              <Button variant="ghost" class="h-auto flex-col gap-1 px-0.5 py-1.5" @click="comingSoon">
                <Clock class="size-4" />
                <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.timer') }}</span>
              </Button>
            </TooltipTrigger>
            <TooltipContent side="left" class="max-w-[220px] text-xs">
              {{ t('phone.rc.timerTip', { time: elapsedText }) }}
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
        <Button
          :variant="activePanel === 'display' ? 'secondary' : 'ghost'"
          class="h-auto flex-col gap-1 px-0.5 py-1.5"
          @click="togglePanel('display')"
        >
          <Monitor class="size-4" />
          <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.display') }}</span>
        </Button>
        <Button
          :variant="activePanel === 'keys' ? 'secondary' : 'ghost'"
          class="h-auto flex-col gap-1 px-0.5 py-1.5"
          @click="togglePanel('keys')"
        >
          <Keyboard class="size-4" />
          <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.keys') }}</span>
        </Button>
        <Button
          :variant="activePanel === 'files' ? 'secondary' : 'ghost'"
          class="h-auto flex-col gap-1 px-0.5 py-1.5"
          @click="togglePanel('files')"
        >
          <FolderOpen class="size-4" />
          <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.files') }}</span>
        </Button>
        <Button
          :variant="activePanel === 'upload' ? 'secondary' : 'ghost'"
          class="h-auto flex-col gap-1 px-0.5 py-1.5"
          :title="t('phone.rc.panelUpload')"
          @click="togglePanel('upload')"
        >
          <CloudUpload class="size-4" />
          <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.panelUpload') }}</span>
        </Button>
        <Button variant="ghost" :disabled="!connected" class="h-auto flex-col gap-1 px-0.5 py-1.5" @click="screenshot">
          <Camera class="size-4" />
          <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.screenshot') }}</span>
        </Button>
        <Button variant="ghost" :disabled="!controlReady" class="h-auto flex-col gap-1 px-0.5 py-1.5" @click="onRotate">
          <Smartphone class="size-4 transition-transform" :class="landscape ? '-rotate-90' : ''" />
          <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.rotate') }}</span>
        </Button>
        <Button
          :variant="activePanel === 'camera' ? 'secondary' : 'ghost'"
          class="h-auto flex-col gap-1 px-0.5 py-1.5"
          @click="togglePanel('camera')"
        >
          <Radio class="size-4" />
          <span class="text-center text-[10px] leading-tight">{{ t('phone.rc.live') }}</span>
        </Button>
        <Button
          :variant="activePanel === 'apps' ? 'secondary' : 'ghost'"
          class="h-auto flex-col gap-1 px-0.5 py-1.5"
          @click="togglePanel('apps')"
        >
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
          <DropdownMenuContent align="start" side="right" class="w-32">
            <DropdownMenuItem @click="retry">
              <RefreshCw class="size-4" /> {{ t('phone.rc.reconnect') }}
            </DropdownMenuItem>
            <DropdownMenuItem @click="newDevice">
              <RotateCw class="size-4" /> {{ t('phone.rc.newDevice') }}
            </DropdownMenuItem>
            <DropdownMenuItem @click="clean">
              <Eraser class="size-4" /> {{ t('phone.rc.clean') }}
            </DropdownMenuItem>
            <DropdownMenuItem @click="boost">
              <Rocket class="size-4" /> {{ t('phone.rc.accelerate') }}
            </DropdownMenuItem>
            <DropdownMenuItem @click="detectIp">
              <Search class="size-4" /> {{ t('phone.rc.detectIp') }}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <!-- 展开面板：显示参数 -->
      <Transition name="panel">
        <div
          v-if="activePanel === 'display'"
          class="flex w-56 shrink-0 flex-col gap-4 overflow-y-auto rounded-md border p-4"
        >
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-medium text-muted-foreground">{{ t('phone.rc.resolution') }}</label>
            <NativeSelect v-model="selectedRes" class="h-9 w-full" @update:model-value="onSettingChange">
              <NativeSelectOption v-for="r in resOptions" :key="r.label" :value="r.label">
                {{ r.label }}
              </NativeSelectOption>
            </NativeSelect>
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-medium text-muted-foreground">{{ t('phone.rc.quality') }}</label>
            <NativeSelect v-model.number="selectedQuality" class="h-9 w-full" @update:model-value="onSettingChange">
              <NativeSelectOption v-for="q in qualityOptions" :key="q.value" :value="q.value">
                {{ q.label }}
              </NativeSelectOption>
            </NativeSelect>
          </div>
          <div class="flex flex-col gap-1.5">
            <label class="text-xs font-medium text-muted-foreground">{{ t('phone.rc.fps') }}</label>
            <NativeSelect v-model.number="selectedFps" class="h-9 w-full" @update:model-value="onSettingChange">
              <NativeSelectOption v-for="f in fpsOptions" :key="f" :value="f">
                {{ f }}fps
              </NativeSelectOption>
            </NativeSelect>
          </div>
          <Button variant="outline" class="w-full justify-start gap-2" @click="fullscreen">
            <Maximize class="size-4" />
            {{ t('phone.rc.fullscreen') }}
          </Button>
        </div>
      </Transition>

      <!-- 展开面板：按键（圆形按钮 + 下方文字，按音量 / 电源 / 导航分行） -->
      <Transition name="panel">
        <div
          v-if="activePanel === 'keys'"
          class="flex w-56 shrink-0 flex-col content-start gap-5 overflow-y-auto rounded-md border p-4"
        >
          <!-- 音量：静音 / 音量+ / 音量- 一行 -->
          <div class="flex items-start justify-around gap-2">
            <div class="flex flex-col items-center gap-1.5">
              <Button
                :variant="isMuted ? 'secondary' : 'outline'"
                :disabled="!connected"
                class="size-12 rounded-full p-0"
                @click="toggleMute"
              >
                <component :is="isMuted ? VolumeX : Volume2" class="size-5" />
              </Button>
              <span class="text-xs">{{ isMuted ? t('phone.rc.unmute') : t('phone.rc.mute') }}</span>
            </div>
            <div class="flex flex-col items-center gap-1.5">
              <Button variant="outline" :disabled="!connected" class="size-12 rounded-full p-0" @click="input.sendButton('volume_up')">
                <Volume2 class="size-5" />
              </Button>
              <span class="text-xs">{{ t('phone.rc.btnVolUp') }}</span>
            </div>
            <div class="flex flex-col items-center gap-1.5">
              <Button variant="outline" :disabled="!connected" class="size-12 rounded-full p-0" @click="input.sendButton('volume_down')">
                <Volume1 class="size-5" />
              </Button>
              <span class="text-xs">{{ t('phone.rc.btnVolDown') }}</span>
            </div>
          </div>

          <!-- 电源：单独一行 -->
          <div class="flex items-start justify-around gap-2">
            <div class="flex flex-col items-center gap-1.5">
              <Button variant="outline" :disabled="!connected" class="size-12 rounded-full p-0" @click="input.sendButton('power')">
                <Power class="size-5" />
              </Button>
              <span class="text-xs">{{ t('phone.rc.btnPower') }}</span>
            </div>
          </div>

          <!-- 导航：返回 / 主屏 / 菜单 一行 -->
          <div class="flex items-start justify-around gap-2">
            <div class="flex flex-col items-center gap-1.5">
              <Button variant="outline" :disabled="!connected" class="size-12 rounded-full p-0" @click="input.sendButton('back')">
                <ChevronLeft class="size-5" />
              </Button>
              <span class="text-xs">{{ t('phone.rc.btnBack') }}</span>
            </div>
            <div class="flex flex-col items-center gap-1.5">
              <Button variant="outline" :disabled="!connected" class="size-12 rounded-full p-0" @click="input.sendButton('home')">
                <Circle class="size-5" />
              </Button>
              <span class="text-xs">{{ t('phone.rc.btnHome') }}</span>
            </div>
            <div class="flex flex-col items-center gap-1.5">
              <Button variant="outline" :disabled="!connected" class="size-12 rounded-full p-0" @click="input.sendButton('recent')">
                <Square class="size-5" />
              </Button>
              <span class="text-xs">{{ t('phone.rc.btnRecent') }}</span>
            </div>
          </div>
        </div>
      </Transition>

      <!-- 展开面板：文件管理（浏览 + 下载 + 批量多选下载） -->
      <Transition name="panel">
        <div
          v-if="activePanel === 'files'"
          class="flex w-[380px] shrink-0 flex-col overflow-hidden rounded-md border"
        >
          <RemoteFilePanel :phone-id="id" />
        </div>
      </Transition>

      <!-- 展开面板：应用（我的应用 / 应用市场 / 已安装，可安装/卸载到本机） -->
      <Transition name="panel">
        <div
          v-if="activePanel === 'apps'"
          class="flex w-[380px] shrink-0 flex-col overflow-hidden rounded-md border"
        >
          <RemoteAppPanel :phone-ids="[id]" :master-id="id" />
        </div>
      </Transition>

      <!-- 展开面板：上传（固定上传到 /sdcard/Download） -->
      <Transition name="panel">
        <div
          v-if="activePanel === 'upload'"
          class="flex w-[380px] shrink-0 flex-col overflow-hidden rounded-md border"
        >
          <RemoteUploadPanel :phone-id="id" />
        </div>
      </Transition>

      <!-- 展开面板：摄像头/麦克风注入（直播）—— 把本机摄像头+麦克风推给云手机 -->
      <Transition name="panel">
        <div
          v-if="activePanel === 'camera'"
          class="flex w-[280px] shrink-0 flex-col gap-4 overflow-y-auto rounded-md border p-4"
        >
          <div class="flex flex-col gap-1">
            <span class="text-sm font-medium">{{ t('phone.rc.cameraTitle') }}</span>
            <span class="text-xs text-muted-foreground">{{ t('phone.rc.cameraDesc') }}</span>
          </div>

          <!-- ===== 摄像头 ===== -->
          <div class="flex flex-col gap-2 rounded-md border p-3">
            <div class="flex items-center gap-1.5 text-xs font-medium">
              <Video class="size-3.5" /> {{ t('phone.rc.cameraSection') }}
            </div>

            <!-- 自拍预览 -->
            <div class="relative aspect-[9/16] w-full overflow-hidden rounded-md border bg-black">
              <video
                ref="cameraPreviewRef"
                class="size-full object-contain"
                :class="mirrorEnabled ? 'scale-x-[-1]' : ''"
                autoplay
                playsinline
                muted
              />
              <div v-if="!videoInjecting" class="absolute inset-0 flex items-center justify-center text-xs text-muted-foreground">
                {{ t('phone.rc.cameraPreviewOff') }}
              </div>
            </div>

            <!-- 摄像头设备 -->
            <NativeSelect v-if="videoInputs.length" v-model="videoDeviceModel" class="h-9 w-full">
              <NativeSelectOption v-for="(c, i) in videoInputs" :key="c.deviceId" :value="c.deviceId">
                {{ c.label || t('phone.rc.cameraDeviceN', { n: i + 1 }) }}
              </NativeSelectOption>
            </NativeSelect>
            <div v-else class="flex h-9 items-center rounded-md border bg-muted/30 px-3 text-xs text-muted-foreground">
              {{ t('phone.rc.cameraNoDevice') }}
            </div>

            <!-- 镜像开关 -->
            <div class="flex items-center justify-between">
              <label class="text-xs text-muted-foreground">{{ t('phone.rc.cameraMirror') }}</label>
              <Switch :model-value="mirrorEnabled" @update:model-value="(v) => onToggleMirror(v === true)" />
            </div>

            <Button
              size="sm"
              class="gap-1"
              :variant="videoInjecting ? 'destructive' : 'default'"
              :disabled="!canInject || videoInjectBusy || (!videoInjecting && !videoInputs.length)"
              @click="onToggleVideo"
            >
              <RefreshCw v-if="videoInjectBusy" class="size-4 animate-spin" />
              <Video v-else class="size-4" />
              {{ videoInjecting ? t('phone.rc.cameraStop') : t('phone.rc.cameraStart') }}
            </Button>
          </div>

          <!-- ===== 麦克风 ===== -->
          <div class="flex flex-col gap-2 rounded-md border p-3">
            <div class="flex items-center gap-1.5 text-xs font-medium">
              <Mic class="size-3.5" /> {{ t('phone.rc.micSection') }}
            </div>

            <NativeSelect v-if="audioInputs.length" v-model="audioDeviceModel" class="h-9 w-full">
              <NativeSelectOption v-for="(c, i) in audioInputs" :key="c.deviceId" :value="c.deviceId">
                {{ c.label || t('phone.rc.micDeviceN', { n: i + 1 }) }}
              </NativeSelectOption>
            </NativeSelect>
            <div v-else class="flex h-9 items-center rounded-md border bg-muted/30 px-3 text-xs text-muted-foreground">
              {{ t('phone.rc.micNoDevice') }}
            </div>

            <Button
              size="sm"
              class="gap-1"
              :variant="audioInjecting ? 'destructive' : 'default'"
              :disabled="!canInject || audioInjectBusy || (!audioInjecting && !audioInputs.length)"
              @click="onToggleAudio"
            >
              <RefreshCw v-if="audioInjectBusy" class="size-4 animate-spin" />
              <Mic v-else class="size-4" />
              {{ audioInjecting ? t('phone.rc.micStop') : t('phone.rc.micStart') }}
            </Button>
          </div>

          <p v-if="!canInject" class="text-xs text-muted-foreground">{{ t('phone.rc.cameraNeedConn') }}</p>
        </div>
      </Transition>
    </div>

    <!-- 「更多」里清理 / 加速的二次确认弹框 -->
    <AlertDialog v-model:open="confirmState.open">
      <AlertDialogContent class="sm:max-w-xs">
        <AlertDialogHeader class="text-left">
          <AlertDialogTitle>{{ confirmState.title }}</AlertDialogTitle>
          <AlertDialogDescription class="text-left">
            {{ confirmState.desc }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <!-- 控制窗较窄，强制按钮成行靠右下、用小按钮，避免在窄宽下竖排撑满 -->
        <AlertDialogFooter class="flex-row justify-end">
          <AlertDialogCancel class="mt-0 h-8 rounded-md px-3 text-xs">
            {{ t('crud.cancel') }}
          </AlertDialogCancel>
          <AlertDialogAction class="h-8 rounded-md px-3 text-xs" @click="confirmState.run()">
            {{ t('crud.confirm') }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>

<style scoped>
.panel-enter-active,
.panel-leave-active {
  transition: transform 0.2s ease, opacity 0.2s ease;
}
.panel-enter-from,
.panel-leave-to {
  opacity: 0;
  transform: translateX(100%);
}
</style>
