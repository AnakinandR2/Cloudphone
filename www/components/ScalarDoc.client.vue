<script setup lang="ts">
// Scalar API 参考（仅客户端）。standalone 挂载 + theme:'none'，把 Scalar 的 CSS 变量
// 映射到站点设计 token（--fg/--bg-*/--border/--accent…），主题色与明暗全部跟随站点。
// specUrl 指向本站 /_content 代理，Key 不进浏览器。
// 用自托管的 Scalar standalone 浏览器构建（public/vendor/scalar-standalone.js，由 nuxt.config
// 从 node_modules 拷贝）。以 <script> 方式加载、走 window.Scalar.createApiReference 挂载，让 Scalar
// 完全脱离 Vite 模块图，规避 dev 下的 504 / Pre-transform 栈溢出等大依赖预处理问题。
// standalone 会自注入结构样式；theme:'none' 关配色预设，下面 customCss 覆盖成站点 token。
type ScalarInstance = { destroy?: () => void; app?: { unmount: () => void } }
type ScalarGlobal = { createApiReference: (el: Element, cfg: Record<string, unknown>) => ScalarInstance }

let scalarPromise: Promise<ScalarGlobal> | null = null
function loadScalar(): Promise<ScalarGlobal> {
  const g = window as unknown as { Scalar?: ScalarGlobal }
  if (g.Scalar?.createApiReference) return Promise.resolve(g.Scalar)
  if (scalarPromise) return scalarPromise
  scalarPromise = new Promise((resolve, reject) => {
    const s = document.createElement('script')
    s.src = '/vendor/scalar-standalone.js'
    s.async = true
    s.onload = () => (g.Scalar?.createApiReference ? resolve(g.Scalar) : reject(new Error('Scalar 全局缺失')))
    s.onerror = () => reject(new Error('加载 scalar standalone 失败'))
    document.head.appendChild(s)
  })
  return scalarPromise
}

const props = defineProps<{ specUrl: string }>()
const colorMode = useColorMode()
const el = ref<HTMLElement | null>(null)
const ready = ref(false)
let instance: ScalarInstance | null = null
let builtDark: boolean | null = null
let observer: MutationObserver | null = null
let readyTimer: ReturnType<typeof setTimeout> | null = null

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

  /* 内容区宽度与站点 .container 一致（侧栏仍贴左，靠 grid 的 auto 列） */
  --scalar-content-max-width: var(--container, 1240px);
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
/* 文档高度自适应内容：去掉 100dvh 强制最小高度，避免内容短时侧栏/正文与 footer 间出现大块空白 */
.scalar-api-reference .references-layout,
.scalar-api-reference.references-classic .references-layout {
  min-height: auto !important;
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

function clearReadyWatch() {
  observer?.disconnect()
  observer = null
  if (readyTimer) {
    clearTimeout(readyTimer)
    readyTimer = null
  }
}
// Scalar 布局出现即视为「就绪」，撤掉骨架屏；带 8s 兜底。
function watchReady() {
  clearReadyWatch()
  const done = () => {
    if (el.value?.querySelector('.scalar-api-reference, .references-layout')) {
      ready.value = true
      clearReadyWatch()
      return true
    }
    return false
  }
  if (done()) return
  observer = new MutationObserver(done)
  if (el.value) observer.observe(el.value, { childList: true, subtree: true })
  readyTimer = setTimeout(() => {
    ready.value = true
    clearReadyWatch()
  }, 8000)
}

async function mount() {
  if (!el.value || instance) return
  el.value.innerHTML = '' // 宿主置空，确保走全新挂载
  ready.value = false
  builtDark = colorMode.value === 'dark'
  try {
    const Scalar = await loadScalar()
    if (!el.value || instance) return // 等待脚本期间可能已卸载/已挂载
    instance = Scalar.createApiReference(el.value, configuration())
    watchReady()
  } catch (e) {
    console.error('[ScalarDoc] 加载/挂载 Scalar 失败', e)
    ready.value = true
  }
}
function unmount() {
  clearReadyWatch()
  try {
    if (instance?.destroy) instance.destroy()
    else instance?.app?.unmount?.()
  } catch {}
  instance = null
  if (el.value) el.value.innerHTML = ''
}
// 延迟到水合完成之后再挂载：刷新场景下页面数据已在 SSR payload 里，<ScalarDoc> 会在
// 父应用「水合期间」挂载，此时再去 createApp 启一个嵌套 Scalar 应用会与水合冲突导致空白。
// nextTick + rAF 把挂载推到水合结束后；客户端导航场景也照常工作。
function scheduleMount() {
  nextTick(() => requestAnimationFrame(mount))
}

onMounted(scheduleMount)
// 仅当明暗「真正翻转」且已有实例时才重建（避免初次颜色结算时误重建打断初始化）。
watch(
  () => colorMode.value === 'dark',
  (dark) => {
    if (!instance || dark === builtDark) return
    unmount()
    scheduleMount()
  },
)
onBeforeUnmount(unmount)
</script>

<template>
  <div class="scalar-doc" :class="{ 'is-loading': !ready }">
    <div ref="el" class="scalar-doc__host" />
    <Transition name="scalar-fade">
      <div v-if="!ready" class="scalar-skeleton" aria-hidden="true">
        <div class="scalar-skeleton__nav">
          <div class="sk-bar sk-search" />
          <div v-for="n in 9" :key="n" class="sk-bar" :style="{ width: `${55 + ((n * 13) % 40)}%` }" />
        </div>
        <div class="scalar-skeleton__main">
          <div class="sk-bar sk-title" />
          <div class="sk-bar" style="width: 88%" />
          <div class="sk-bar" style="width: 94%" />
          <div class="sk-bar" style="width: 72%" />
          <div class="sk-block" />
          <div class="sk-bar" style="width: 90%" />
          <div class="sk-bar" style="width: 64%" />
        </div>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.scalar-doc { position: relative; }
/* 仅加载期撑起高度给骨架屏；就绪后高度交给 Scalar 自身，避免底部与 footer 间留空白 */
.scalar-doc.is-loading { min-height: 70vh; }

.scalar-skeleton {
  position: absolute;
  inset: 0;
  display: flex;
  gap: 32px;
  background: rgb(var(--bg));
  overflow: hidden;
}
.scalar-skeleton__nav {
  width: 268px;
  flex-shrink: 0;
  padding: 24px 18px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  border-right: 1px solid rgb(var(--border));
  background: rgb(var(--bg-sunken));
}
.scalar-skeleton__main {
  flex: 1;
  max-width: var(--container, 1240px);
  padding: 28px 32px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.sk-bar {
  height: 13px;
  border-radius: 6px;
  background: rgb(var(--bg-inset));
  position: relative;
  overflow: hidden;
}
.sk-search { height: 34px; border-radius: 8px; margin-bottom: 8px; }
.sk-title { height: 30px; width: 42%; margin-bottom: 12px; }
.sk-block {
  height: 200px;
  border-radius: 12px;
  background: rgb(var(--bg-inset));
  position: relative;
  overflow: hidden;
  margin: 10px 0;
}
.sk-bar::after,
.sk-block::after {
  content: '';
  position: absolute;
  inset: 0;
  transform: translateX(-100%);
  background: linear-gradient(90deg, transparent, rgb(var(--bg-elev) / 0.65), transparent);
  animation: sk-shimmer 1.4s ease-in-out infinite;
}
@keyframes sk-shimmer {
  100% { transform: translateX(100%); }
}
@media (max-width: 768px) {
  .scalar-skeleton__nav { display: none; }
}

.scalar-fade-leave-active { transition: opacity 0.3s ease; }
.scalar-fade-leave-to { opacity: 0; }
</style>
