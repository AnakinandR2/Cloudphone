<script setup lang="ts">
import { Crown, RefreshCw } from 'lucide-vue-next'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useWebRTC } from '@/composables/useWebRTC'

const props = defineProps<{ phoneId: number, name: string, master: boolean, selected: boolean }>()
const emit = defineEmits<{
  pointer: [{ kind: 'down' | 'move' | 'up', nx: number, ny: number, button: number }]
  selectMaster: []
  toggleSelected: []
}>()
// 群控中的单台云手机：自管一路 WebRTC；主控台(master)接收本地输入并把归一化坐标 emit 给父，
// 由父广播到所有单元格的 applyPointer（各自按设备尺寸还原像素），实现「主控操作同步全体」。
const { t } = useI18n()
const idRef = computed(() => props.phoneId)
const videoRef = ref<HTMLVideoElement | null>(null)

const {
  connState,
  connStatusText,
  connected,
  isMuted,
  deviceWidth,
  deviceHeight,
  connect,
  retry,
  cleanup,
  toggleMute,
  sendDC,
  selectedRes,
  selectedQuality,
  selectedFps,
} = useWebRTC({ id: idRef, videoRef })

const aspect = computed(() => `${deviceWidth.value}/${deviceHeight.value}`)

function genId(): string {
  const uuid = 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0
    return (c === 'x' ? r : (r & 0x3) | 0x8).toString(16)
  })
  const n = new Date()
  const p2 = (x: number) => String(x).padStart(2, '0')
  return `${uuid}-${n.getFullYear()}${p2(n.getMonth() + 1)}${p2(n.getDate())} ${p2(n.getHours())}:${p2(n.getMinutes())}:${p2(n.getSeconds())}:${String(n.getMilliseconds()).padStart(3, '0')}`
}

// ---- 被父调用：把归一化坐标(0..1)还原到本机设备像素并下发 ----
let downId: string | null = null
function applyPointer(kind: 'down' | 'move' | 'up', nx: number, ny: number, button: number) {
  if (connState.value !== 'connected')
    return
  const x = Math.round(nx * (deviceWidth.value - 1))
  const y = Math.round(ny * (deviceHeight.value - 1))
  const w = deviceWidth.value
  const h = deviceHeight.value
  if (kind === 'down') {
    downId = genId()
    sendDC({ type: 'mouse_down', x, y, button, rotation: 0, width: w, height: h, downEventId: downId, messageId: downId })
  }
  else if (kind === 'move') {
    if (!downId)
      return
    sendDC({ type: 'mouse_move', x, y, rotation: 0, width: w, height: h, downEventId: downId, messageId: genId() })
  }
  else {
    if (!downId)
      return
    sendDC({ type: 'mouse_up', x, y, button, rotation: 0, width: w, height: h, downEventId: downId, messageId: genId() })
    downId = null
  }
}

function sendButton(btn: string) {
  sendDC({ type: `button_${btn}` })
}

function setMute(want: boolean) {
  if (isMuted.value !== want)
    toggleMute()
}

// 截当前帧为 PNG（群控批量截屏用）。
function captureShot(): Promise<Blob | null> {
  return new Promise((resolve) => {
    const v = videoRef.value
    if (!v || !v.videoWidth || !v.videoHeight) {
      resolve(null)
      return
    }
    const canvas = document.createElement('canvas')
    canvas.width = v.videoWidth
    canvas.height = v.videoHeight
    const ctx = canvas.getContext('2d')
    if (!ctx) {
      resolve(null)
      return
    }
    ctx.drawImage(v, 0, 0, canvas.width, canvas.height)
    canvas.toBlob(b => resolve(b), 'image/png')
  })
}

// 改串流参数（群控「显示」统一下发）：改后需重连生效。
function setStream(res: string, quality: number, fps: number) {
  selectedRes.value = res
  selectedQuality.value = quality
  selectedFps.value = fps
  retry()
}

defineExpose({ applyPointer, sendButton, setMute, retry, captureShot, setStream })

// ---- 主控台本地输入 → 归一化 → 通知父 ----
function localNorm(clientX: number, clientY: number): { nx: number, ny: number } | null {
  const v = videoRef.value
  if (!v)
    return null
  const rect = v.getBoundingClientRect()
  const dx = clientX - rect.left
  const dy = clientY - rect.top
  if (dx < 0 || dx > rect.width || dy < 0 || dy > rect.height)
    return null
  const vw = v.videoWidth || deviceWidth.value
  const vh = v.videoHeight || deviceHeight.value
  const scale = Math.max(rect.width / vw, rect.height / vh)
  const ox = (rect.width - vw * scale) / 2
  const oy = (rect.height - vh * scale) / 2
  const nx = Math.min(1, Math.max(0, (dx - ox) / scale / vw))
  const ny = Math.min(1, Math.max(0, (dy - oy) / scale / vh))
  return { nx, ny }
}

let pressing = false
let lastNx = 0
let lastNy = 0
function onDown(e: MouseEvent) {
  if (!props.master)
    return
  e.preventDefault()
  const c = localNorm(e.clientX, e.clientY)
  if (!c)
    return
  pressing = true
  lastNx = c.nx
  lastNy = c.ny
  emit('pointer', { kind: 'down', nx: c.nx, ny: c.ny, button: e.button })
}
function onMove(e: MouseEvent) {
  if (!props.master || !pressing)
    return
  const c = localNorm(e.clientX, e.clientY)
  if (!c)
    return
  lastNx = c.nx
  lastNy = c.ny
  emit('pointer', { kind: 'move', nx: c.nx, ny: c.ny, button: 0 })
}
function onUp(e: MouseEvent) {
  if (!props.master || !pressing)
    return
  pressing = false
  const c = localNorm(e.clientX, e.clientY) ?? { nx: 0, ny: 0 }
  emit('pointer', { kind: 'up', nx: c.nx, ny: c.ny, button: e.button })
}
function onTouchStart(e: TouchEvent) {
  if (!props.master || !e.touches[0])
    return
  const c = localNorm(e.touches[0].clientX, e.touches[0].clientY)
  if (!c)
    return
  pressing = true
  emit('pointer', { kind: 'down', nx: c.nx, ny: c.ny, button: 0 })
}
function onTouchMove(e: TouchEvent) {
  if (!props.master || !pressing || !e.touches[0])
    return
  const c = localNorm(e.touches[0].clientX, e.touches[0].clientY)
  if (!c)
    return
  emit('pointer', { kind: 'move', nx: c.nx, ny: c.ny, button: 0 })
}
function onTouchEnd(e: TouchEvent) {
  if (!props.master || !pressing)
    return
  pressing = false
  const tt = e.changedTouches[0]
  const c = (tt && localNorm(tt.clientX, tt.clientY)) || { nx: 0, ny: 0 }
  emit('pointer', { kind: 'up', nx: c.nx, ny: c.ny, button: 0 })
}

// 鼠标拖出画面后在窗口级补一个 up，避免设备侧卡在「按下」。
function onWinUp() {
  if (!pressing)
    return
  pressing = false
  emit('pointer', { kind: 'up', nx: lastNx, ny: lastNy, button: 0 })
}

onMounted(() => {
  connect()
  window.addEventListener('mouseup', onWinUp)
})
onBeforeUnmount(() => {
  window.removeEventListener('mouseup', onWinUp)
  cleanup()
})
</script>

<template>
  <div
    class="group relative flex flex-col overflow-hidden rounded-md border bg-black"
    :class="master ? 'ring-2 ring-primary' : 'ring-1 ring-transparent'"
  >
    <!-- 标题条 -->
    <div class="absolute inset-x-0 top-0 z-10 flex items-center gap-1 bg-gradient-to-b from-black/70 to-transparent px-2 py-1 text-xs text-white">
      <input
        type="checkbox"
        class="size-3.5 shrink-0 accent-primary"
        :checked="selected"
        :title="t('phone.group.cellSelect')"
        @click.stop
        @change="emit('toggleSelected')"
      >
      <Crown v-if="master" class="size-3.5 text-amber-400" />
      <span class="truncate">{{ name }}</span>
    </div>

    <div class="relative w-full" :style="{ aspectRatio: aspect }">
      <video
        ref="videoRef"
        class="absolute inset-0 size-full touch-none select-none bg-black object-contain"
        :class="master ? '' : 'pointer-events-none'"
        autoplay
        playsinline
        muted
        @mousedown="onDown"
        @mousemove="onMove"
        @mouseup="onUp"
        @touchstart.prevent="onTouchStart"
        @touchmove.prevent="onTouchMove"
        @touchend.prevent="onTouchEnd"
      />
      <!-- 非主控：点击整片选为主控 -->
      <button
        v-if="!master"
        type="button"
        class="absolute inset-0 z-[5] cursor-pointer"
        @click="emit('selectMaster')"
      />
      <!-- 未连接遮罩 -->
      <div
        v-if="!connected"
        class="absolute inset-0 z-[6] flex flex-col items-center justify-center gap-2 bg-black/70 text-center text-xs text-white"
      >
        <p>{{ connStatusText }}</p>
        <RefreshCw v-if="connState === 'connecting'" class="size-5 animate-spin opacity-80" />
      </div>
    </div>
  </div>
</template>
