<script setup lang="ts">
// Scalar API 参考（仅客户端：.client.vue 不在 SSR 求值，避开重组件的 SSR 问题）。
// specUrl 指向本站 /_content 代理，Key 不进浏览器。
import { ApiReference } from '@scalar/api-reference'
import '@scalar/api-reference/style.css'

const props = defineProps<{ specUrl: string }>()
const config = computed(() => ({ url: props.specUrl }))
</script>

<template>
  <ApiReference :configuration="config" />
</template>
