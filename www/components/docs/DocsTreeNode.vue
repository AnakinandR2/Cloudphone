<script setup lang="ts">
// 文档目录树的递归节点。支持任意层级嵌套：
//  - 文章 → 真实 <a>（/help/<slug>），当前篇高亮；
//  - 分组 → 不是链接（无落地页，点了会 404）。顶层分组=静态小标题；子组=可点击折叠。
import type { PubDirectoryNode } from '~/types/content'

const props = defineProps<{ node: PubDirectoryNode; currentSlug: string; depth: number }>()
const localePath = useLocalePath()

const collapsible = props.node.kind === 'group' && props.depth >= 1

// 子树是否包含当前文章（用于默认展开到当前篇）。
function hasCurrent(node: PubDirectoryNode): boolean {
  if (node.kind === 'article') return node.slug === props.currentSlug
  return (node.children || []).some(hasCurrent)
}

// 顶层分组恒展开；子组按平台 collapsed 默认，含当前篇则强制展开。
const open = ref(!collapsible ? true : !props.node.collapsed || hasCurrent(props.node))
// 客户端导航切换文章时，确保通往当前篇的路径展开。
watch(
  () => props.currentSlug,
  () => {
    if (collapsible && hasCurrent(props.node)) open.value = true
  },
)
</script>

<template>
  <NuxtLink
    v-if="node.kind === 'article'"
    :to="localePath('/help/' + node.slug)"
    class="docs-nav__link"
    :class="{ active: node.slug === currentSlug }"
  >
    {{ node.title }}
  </NuxtLink>

  <div v-else class="docs-nav__group" :class="{ 'docs-nav__group--sub': depth >= 1 }">
    <button
      v-if="collapsible"
      type="button"
      class="docs-nav__group-toggle"
      :aria-expanded="open"
      @click="open = !open"
    >
      <svg class="docs-nav__chev" :class="{ open }" viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
        <path fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" d="M6 4l4 4-4 4" />
      </svg>
      <span>{{ node.title }}</span>
    </button>
    <div v-else class="docs-nav__group-title">{{ node.title }}</div>

    <div v-show="open" class="docs-nav__children">
      <DocsTreeNode
        v-for="c in node.children || []"
        :key="c.slug"
        :node="c"
        :current-slug="currentSlug"
        :depth="depth + 1"
      />
    </div>
  </div>
</template>
