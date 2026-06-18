<script setup lang="ts">
// FAQ 手风琴：标题=问题，答案=body_html（经 ArticleBody 渲染）。单开。
import type { FaqItem } from '~/types/content'

defineProps<{ items: FaqItem[] }>()
const open = ref(-1)
function toggle(i: number) {
  open.value = open.value === i ? -1 : i
}
</script>

<template>
  <div class="faq-list">
    <div v-for="(it, i) in items" :key="it.slug" class="faq-item" :class="{ open: open === i }">
      <button class="faq-q" @click="toggle(i)">
        <span>{{ it.question }}</span>
        <span class="chev"><GpIcon name="plus" style="width: 14px; height: 14px" /></span>
      </button>
      <div class="faq-a">
        <div class="article-html" v-html="it.answerHtml" />
      </div>
    </div>
  </div>
</template>
