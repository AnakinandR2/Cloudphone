<script setup lang="ts">
import type { Component } from 'vue'
import {
  Activity,
  AppWindow,
  Blocks,
  Bot,
  Boxes,
  CircleDot,
  Component as ComponentIcon,
  Cpu,
  CreditCard,
  Disc,
  FileCode,
  FileText,
  Gauge,
  Gift,
  Images,
  Layers,
  LayoutDashboard,
  ListChecks,
  Menu as MenuIcon,
  Network,
  Percent,
  Plug,
  ReceiptText,
  ScrollText,
  Server,
  Settings,
  ShieldCheck,
  Smartphone,
  Sparkles,
  Tags,
  UserRound,
  Users,
  Wallet,
} from 'lucide-vue-next'
import { computed } from 'vue'

const props = defineProps<{ name?: string, class?: string }>()

// 菜单用到的图标注册表（按名字取用，保持按需打包）
const registry: Record<string, Component> = {
  LayoutDashboard,
  Users,
  UserRound,
  ShieldCheck,
  Settings,
  Menu: MenuIcon,
  Gauge,
  FileText,
  Layers,
  CircleDot,
  Blocks,
  Component: ComponentIcon,
  Boxes,
  Sparkles,
  ScrollText,
  Smartphone,
  Cpu,
  Server,
  Disc,
  AppWindow,
  Activity,
  Network,
  Bot,
  CreditCard,
  FileCode,
  Gift,
  Images,
  ListChecks,
  Percent,
  Plug,
  ReceiptText,
  Tags,
  Wallet,
}

const comp = computed(() => (props.name && registry[props.name]) || CircleDot)
</script>

<template>
  <component :is="comp" :class="props.class" />
</template>
