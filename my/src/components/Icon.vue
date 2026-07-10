<script setup lang="ts">
import type { Component } from 'vue'
import {
  ArchiveBox,
  AlarmClock,
  BagShopping,
  Box3,
  Category22,
  ChartLine,
  CircleInfo,
  Component2,
  CreditCard,
  CpuSetting2,
  File,
  FileText,
  Gauge3,
  Gift,
  Image,
  Layout,
  ListCheck,
  Menu,
  MenuSquare,
  Mobile,
  MoreH,
  Plug,
  ReceiptText2,
  Settings,
  ShieldCheck,
  Sparkles,
  Users,
  Wallet,
  Window,
} from 'reicon-vue'
import { computed } from 'vue'

const props = defineProps<{ name?: string, class?: string }>()

/** BagShopping 内建 scale(1.333)，略降描边以贴近其他 1.5 线宽图标 */
const iconExtraProps = computed(() =>
  props.name === 'ShoppingCart' ? { strokeWidth: 1.125 } : {},
)

// Menu icon registry. Keep the existing route icon keys, but render them with Reicon.
const registry: Record<string, Component> = {
  LayoutDashboard: Layout,
  Users,
  ShieldCheck,
  Settings,
  Menu,
  MoreHorizontal: MoreH,
  Gauge: Gauge3,
  FileText,
  FolderArchive: ArchiveBox,
  Gift,
  MenuSquare,
  CircleDot: CircleInfo,
  Blocks: Component2,
  Component: Component2,
  Boxes: Box3,
  CreditCard,
  Sparkles,
  ScrollText: ReceiptText2,
  Smartphone: Mobile,
  Network: Category22,
  AppWindow: Window,
  Bot: CpuSetting2,
  CalendarClock: AlarmClock,
  FileCode: File,
  Images: Image,
  ListChecks: ListCheck,
  LineChart: ChartLine,
  Plug,
  ReceiptText: ReceiptText2,
  ShoppingCart: BagShopping,
  Wallet,
  ReiconPhone3: Mobile,
}

const comp = computed(() => (props.name && registry[props.name]) || CircleInfo)
</script>

<template>
  <component :is="comp" :class="props.class" v-bind="iconExtraProps" />
</template>
