<script setup lang="ts">
import type { Component } from 'vue'
import {
  AppWindow,
  Blocks,
  Bot,
  Boxes,
  CalendarClock,
  CircleDot,
  Component as ComponentIcon,
  FileCode,
  FileText,
  FolderArchive,
  Gauge,
  Gift,
  Images,
  Layers,
  LayoutDashboard,
  LineChart,
  ListChecks,
  Plug,
  ReceiptText,
  ShoppingCart,
  Menu as MenuIcon,
  MoreHorizontal,
  Network,
  ScrollText,
  Smartphone,
  Settings,
  ShieldCheck,
  Sparkles,
  Users,
  Wallet,
} from 'lucide-vue-next'
import { computed } from 'vue'

const props = defineProps<{ name?: string, class?: string }>()

// 菜单用到的图标注册表（按名字取用，保持按需打包）
const registry: Record<string, Component> = {
  LayoutDashboard,
  Users,
  ShieldCheck,
  Settings,
  Menu: MenuIcon,
  MoreHorizontal,
  Gauge,
  FileText,
  FolderArchive,
  Gift,
  Layers,
  CircleDot,
  Blocks,
  Component: ComponentIcon,
  Boxes,
  Sparkles,
  ScrollText,
  Smartphone,
  Network,
  AppWindow,
  Bot,
  CalendarClock,
  FileCode,
  Images,
  ListChecks,
  LineChart,
  Plug,
  ReceiptText,
  ShoppingCart,
  Wallet,
}

const comp = computed(() => (props.name && registry[props.name]) || CircleDot)
</script>

<template>
  <component :is="comp" :class="props.class" />
</template>
