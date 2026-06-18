<script setup lang="ts">
// 右侧 TOC：锚点链接（真实 <a>），滚动时高亮当前小节（仅客户端 IntersectionObserver）。
import type { TocItem } from '~/types/content'

const props = defineProps<{ toc: TocItem[] }>()
const { locale } = useI18n()
const tocTitle = computed(() => (locale.value === 'zh' ? '本页目录' : 'On this page'))

const activeId = ref('')
let observer: IntersectionObserver | null = null

function observe() {
  observer?.disconnect()
  if (!props.toc.length) return
  observer = new IntersectionObserver(
    (entries) => {
      for (const e of entries) {
        if (e.isIntersecting) activeId.value = e.target.id
      }
    },
    { rootMargin: '0px 0px -75% 0px', threshold: 0 },
  )
  for (const item of props.toc) {
    const el = document.getElementById(item.id)
    if (el) observer.observe(el)
  }
}

onMounted(observe)
watch(() => props.toc, () => nextTick(observe))
onBeforeUnmount(() => observer?.disconnect())
</script>

<template>
  <nav v-if="toc.length" class="docs-toc" aria-label="table of contents">
    <div class="docs-toc__title">{{ tocTitle }}</div>
    <a
      v-for="item in toc"
      :key="item.id"
      :href="'#' + item.id"
      class="docs-toc__link"
      :class="{ active: activeId === item.id, sub: item.level === 3 }"
    >
      {{ item.text }}
    </a>
  </nav>
</template>
