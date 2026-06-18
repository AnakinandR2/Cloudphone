<script setup lang="ts">
// 博客列表 · 按分类 · 第 1 页（/blog-categories/<slug>）
const route = useRoute()
const slug = computed(() => String(route.params.slug))
</script>

<template>
  <BlogList mode="category" :slug="slug" :page="1" />
</template>
