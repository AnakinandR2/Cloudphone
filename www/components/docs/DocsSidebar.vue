<script setup lang="ts">
// 文档左侧目录树：递归渲染分组（可折叠）+ 文章链接，当前篇高亮。支持任意层级嵌套。
import type { PubDirectoryNode } from '~/types/content'

defineProps<{
  tree: PubDirectoryNode[]
  currentSlug: string
}>()
</script>

<template>
  <nav class="docs-nav" aria-label="docs">
    <DocsTreeNode
      v-for="node in tree"
      :key="node.slug"
      :node="node"
      :current-slug="currentSlug"
      :depth="0"
    />
  </nav>
</template>
