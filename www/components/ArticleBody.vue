<script setup lang="ts">
// 文章正文渲染。集中 v-html 的唯一入口 —— 内容来自受信内部 CMS；
// 若未来开放第三方撰写，可在此统一接入 HTML 净化。
defineProps<{ html: string }>()
</script>

<template>
  <div class="article-html" v-html="html" />
</template>
