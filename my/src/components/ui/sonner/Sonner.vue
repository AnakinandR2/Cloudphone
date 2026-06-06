<script lang="ts" setup>
import type { ToasterProps } from "vue-sonner"
import { CircleCheckIcon, InfoIcon, Loader2Icon, OctagonXIcon, TriangleAlertIcon, XIcon } from "lucide-vue-next"
import { Toaster as Sonner } from "vue-sonner"
import { cn } from "@/lib/utils"

const props = defineProps<ToasterProps>()

// vue-sonner 默认会在 toast 上做「拖动滑走关闭」（pointerdown 时 setPointerCapture + 监听拖动），
// 这会导致用户想框选/复制提示文案时一拖就把弹窗关掉。
// 这里在 capture 阶段拦掉非按钮区域的 pointerdown：阻止其冒泡到 toast 自身的拖动逻辑，
// 既禁用了拖动关闭，也恢复了文本可选中（可复制）。关闭改由右上角关闭按钮完成。
function onPointerDownCapture(e: PointerEvent) {
  const target = e.target as HTMLElement | null
  // 按钮（关闭按钮 / action / cancel）保持原生行为，不拦截。
  if (target?.closest('button'))
    return
  e.stopPropagation()
}
</script>

<template>
  <Sonner
    :class="cn('toaster group', props.class)"
    :style="{
      '--normal-bg': 'var(--popover)',
      '--normal-text': 'var(--popover-foreground)',
      '--normal-border': 'var(--border)',
      '--border-radius': 'var(--radius)',
    }"
    v-bind="props"
    @pointerdown.capture="onPointerDownCapture"
  >
    <template #success-icon>
      <CircleCheckIcon class="size-4" />
    </template>
    <template #info-icon>
      <InfoIcon class="size-4" />
    </template>
    <template #warning-icon>
      <TriangleAlertIcon class="size-4" />
    </template>
    <template #error-icon>
      <OctagonXIcon class="size-4" />
    </template>
    <template #loading-icon>
      <div>
        <Loader2Icon class="size-4 animate-spin" />
      </div>
    </template>
    <template #close-icon>
      <XIcon class="size-4" />
    </template>
  </Sonner>
</template>

<style>
/* 关闭按钮放到 toast 内部右侧（vue-sonner 默认在左上角外侧），并常显 */
.toaster [data-close-button] {
  top: 50% !important;
  right: 8px !important;
  left: auto !important;
  transform: translateY(-50%) !important;
  opacity: 1 !important;
  border: none !important;
  background: transparent !important;
  box-shadow: none !important;
}
/* 留出右侧空间，避免关闭按钮压住文案 */
.toaster [data-sonner-toast] {
  padding-right: 1.75rem;
}
</style>
