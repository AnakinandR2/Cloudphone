<script setup lang="ts">
// 博客列表 · 全部 · 第 N 页（/blog/page/N）
const route = useRoute()
const page = computed(() => Number.parseInt(String(route.params.page ?? '1'), 10))
if (!Number.isFinite(page.value) || page.value < 1) {
  throw createError({ statusCode: 404, statusMessage: 'Page not found', fatal: true })
}
</script>

<template>
  <BlogList mode="all" :page="page" />
</template>
