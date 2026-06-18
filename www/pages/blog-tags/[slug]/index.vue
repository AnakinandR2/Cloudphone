<script setup lang="ts">
// 博客列表 · 按标签 · 第 1 页（/blog-tags/<slug>）
const route = useRoute()
const slug = computed(() => String(route.params.slug))
</script>

<template>
  <BlogList mode="tag" :slug="slug" :page="1" />
</template>
