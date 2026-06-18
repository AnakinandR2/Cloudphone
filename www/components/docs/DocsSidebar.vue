<script setup lang="ts">
// 文档左侧目录树：分组标题 + 文章链接（真实 <a>），当前篇高亮。
// 目录数据为两级（组 → 文章），并兼容顶层散文章。
import type { PubDirectoryNode } from '~/types/content'

defineProps<{
  tree: PubDirectoryNode[]
  currentSlug: string
}>()

const localePath = useLocalePath()
</script>

<template>
  <nav class="docs-nav" aria-label="docs">
    <template v-for="node in tree" :key="node.slug">
      <div v-if="node.kind === 'group'" class="docs-nav__group">
        <div class="docs-nav__group-title">{{ node.title }}</div>
        <NuxtLink
          v-for="c in node.children || []"
          :key="c.slug"
          :to="localePath('/help/' + c.slug)"
          class="docs-nav__link"
          :class="{ active: c.slug === currentSlug }"
        >
          {{ c.title }}
        </NuxtLink>
      </div>
      <NuxtLink
        v-else
        :to="localePath('/help/' + node.slug)"
        class="docs-nav__link"
        :class="{ active: node.slug === currentSlug }"
      >
        {{ node.title }}
      </NuxtLink>
    </template>
  </nav>
</template>
