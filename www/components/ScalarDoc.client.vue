<script setup lang="ts">
// Scalar API 参考（仅客户端）。standalone 挂载 + theme:'none'，把 Scalar 的 CSS 变量
// 映射到站点设计 token（--fg/--bg-*/--border/--accent…），主题色与明暗全部跟随站点。
// specUrl 指向本站 /_content 代理，Key 不进浏览器。
import { createApiReference } from '@scalar/api-reference'
// npm 版 createApiReference 不像 CDN standalone 那样自动注入样式，需手动引入结构样式；
// theme:'none' 只关掉内置配色预设，下面 customCss 再把配色覆盖成站点 token。
import '@scalar/api-reference/style.css'

const props = defineProps<{ specUrl: string }>()
const colorMode = useColorMode()
const el = ref<HTMLElement | null>(null)
let instance: { destroy?: () => void; app?: { unmount: () => void } } | null = null

// 把 Scalar 变量映射到站点 token。token 随主题色 + .dark 变化，故一套映射即可联动明暗。
const customCss = `
.scalar-app, .scalar-api-reference, .light-mode, .dark-mode {
  --scalar-font: var(--font-body);
  --scalar-font-code: var(--font-mono);
  --scalar-radius: 8px;
  --scalar-radius-lg: 12px;

  --scalar-color-1: rgb(var(--fg));
  --scalar-color-2: rgb(var(--fg-muted));
  --scalar-color-3: rgb(var(--fg-subtle));
  --scalar-color-accent: var(--accent-color);

  --scalar-background-1: rgb(var(--bg));
  --scalar-background-2: rgb(var(--bg-sunken));
  --scalar-background-3: rgb(var(--bg-inset));
  --scalar-border-color: rgb(var(--border));

  --scalar-button-1: var(--accent-color);
  --scalar-button-1-hover: var(--accent-strong-color);
  --scalar-button-1-color: rgb(var(--accent-fg));

  --scalar-color-green: var(--accent-color);

  --scalar-sidebar-background-1: rgb(var(--bg-sunken));
  --scalar-sidebar-border-color: rgb(var(--border));
  --scalar-sidebar-color-1: rgb(var(--fg));
  --scalar-sidebar-color-2: rgb(var(--fg-muted));
  --scalar-sidebar-color-active: var(--accent-color);
  --scalar-sidebar-item-hover-background: rgb(var(--bg-inset));
  --scalar-sidebar-item-hover-color: rgb(var(--fg));
  --scalar-sidebar-item-active-background: rgb(var(--accent) / 0.1);
  --scalar-sidebar-search-background: rgb(var(--bg));
  --scalar-sidebar-search-border-color: rgb(var(--border));
  --scalar-sidebar-search-color: rgb(var(--fg-muted));
  --scalar-sidebar-indent-border: transparent;
  --scalar-sidebar-indent-border-hover: rgb(var(--border));
  --scalar-sidebar-indent-border-active: var(--accent-color);
}
/* 兜底：个别版本搜索框变量名不一致，直接套 token，避免黑底 */
.scalar-api-reference .sidebar-search,
.scalar-api-reference .scalar-search-input,
.scalar-api-reference .t-doc__sidebar input {
  background: rgb(var(--bg)) !important;
  border-color: rgb(var(--border)) !important;
  color: rgb(var(--fg)) !important;
}
/* 隐藏 Scalar 自带品牌：Powered by Scalar / Open API Client / 侧栏底部块 */
.scalar-api-reference .open-api-client-button,
.scalar-api-reference .sidebar-footer,
.scalar-api-reference [class*="powered-by"],
.scalar-api-reference [class*="open-api-client"],
.scalar-api-reference a[href*="scalar.com"] {
  display: none !important;
}
`

function configuration() {
  return {
    url: props.specUrl,
    theme: 'none' as const,
    darkMode: colorMode.value === 'dark',
    hideDarkModeToggle: true,
    hideClientButton: true,
    // 进来落在文档概览（Introduction），而非第一个接口
    defaultOpenAllTags: false,
    customCss,
  }
}

function mount() {
  if (!el.value) return
  instance = createApiReference(el.value, configuration())
}
function unmount() {
  if (instance?.destroy) instance.destroy()
  else instance?.app?.unmount?.()
  instance = null
  if (el.value) el.value.innerHTML = '' // 清空宿主，避免重建时走 Scalar 的 SSR 水合分支
}

onMounted(mount)
// 站点切换明暗 → 重建实例，让代码块高亮等也跟着换配色（customCss token 已随 .dark 自动联动）
watch(() => colorMode.value, () => {
  unmount()
  nextTick(mount)
})
onBeforeUnmount(unmount)
</script>

<template>
  <div ref="el" class="scalar-doc-host" />
</template>

<style scoped>
.scalar-doc-host { min-height: 70vh; }
</style>
