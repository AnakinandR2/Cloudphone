<script setup lang="ts">
// 数字分页器。全部是真实 <a>（NuxtLink），支持直接跳到任意页；
// 页数多时首尾页常驻、中间省略。上一页/下一页在边界时为禁用态（非链接）。
const props = defineProps<{
  page: number
  totalPages: number
  /** 给定页码 → 目标路径（已含 localePath） */
  hrefFor: (p: number) => string
}>()

const { t } = useGp()
const items = computed(() => pageWindow(props.page, props.totalPages))
</script>

<template>
  <nav v-if="totalPages > 1" class="blog-pager" aria-label="pagination">
    <NuxtLink v-if="page > 1" :to="hrefFor(page - 1)" class="blog-pager__btn" rel="prev">{{ t.blog.prev }}</NuxtLink>
    <span v-else class="blog-pager__btn is-disabled">{{ t.blog.prev }}</span>

    <template v-for="(it, i) in items" :key="i">
      <span v-if="it === '...'" class="blog-pager__gap">…</span>
      <NuxtLink v-else-if="it !== page" :to="hrefFor(it)" class="blog-pager__num">{{ it }}</NuxtLink>
      <span v-else class="blog-pager__num is-current" aria-current="page">{{ it }}</span>
    </template>

    <NuxtLink v-if="page < totalPages" :to="hrefFor(page + 1)" class="blog-pager__btn" rel="next">{{ t.blog.next }}</NuxtLink>
    <span v-else class="blog-pager__btn is-disabled">{{ t.blog.next }}</span>
  </nav>
</template>

<style scoped>
.blog-pager { display: flex; align-items: center; justify-content: center; flex-wrap: wrap; gap: 8px; margin-top: 48px; }
.blog-pager__btn,
.blog-pager__num {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 38px; height: 38px; padding: 0 12px;
  border-radius: 10px; border: 1px solid rgb(var(--border));
  font-size: 14px; font-weight: 600; color: rgb(var(--fg-muted));
  background: transparent; transition: all 0.15s ease;
}
.blog-pager__num { font-family: var(--font-mono); }
.blog-pager__btn:hover,
.blog-pager__num:hover { color: rgb(var(--fg)); border-color: rgb(var(--border-strong)); }
.blog-pager__num.is-current {
  background: var(--accent-color); color: rgb(var(--accent-fg)); border-color: var(--accent-color); cursor: default;
}
.blog-pager__btn.is-disabled { opacity: 0.4; pointer-events: none; }
.blog-pager__gap { color: rgb(var(--fg-muted)); padding: 0 2px; user-select: none; }
</style>
