<script setup lang="ts">
// 文档三栏布局：左目录树 ｜ 中正文(slot) ｜ 右 TOC。
// 移动端：目录折叠为顶部「目录」开关；右 TOC 隐藏。
import type { PubDirectoryNode, TocItem } from '~/types/content'

defineProps<{
  tree: PubDirectoryNode[]
  currentSlug: string
  toc: TocItem[]
}>()

const { locale } = useI18n()
const navOpen = ref(false)
const tocLabel = computed(() => (locale.value === 'zh' ? '目录' : 'Menu'))
const route = useRoute()
watch(() => route.fullPath, () => (navOpen.value = false))
</script>

<template>
  <div class="docs-layout container">
    <!-- 移动端目录开关 -->
    <button class="docs-nav-toggle" @click="navOpen = !navOpen">
      <GpIcon name="burger" style="width: 16px; height: 16px" /> {{ tocLabel }}
    </button>

    <aside class="docs-aside" :class="{ open: navOpen }">
      <DocsSidebar :tree="tree" :current-slug="currentSlug" />
    </aside>

    <main class="docs-main">
      <slot />
    </main>

    <aside class="docs-toc-wrap">
      <DocsToc :toc="toc" />
    </aside>
  </div>
</template>
