<script setup lang="ts">
// 首页 FAQ 板块：从内容中台 faq 空间取前 6 条问答。出错/空 → 仅渲染标题。
const { t } = useGp()
const { flat } = useFaq()
const items = computed(() => flat.value.slice(0, 6))
</script>

<template>
  <section class="section" id="help">
    <div class="container">
      <header style="text-align: center">
        <span class="eyebrow">{{ t.faq.eyebrow }}</span>
        <h2 class="section-title">{{ t.faq.title }}</h2>
      </header>
      <FaqAccordion v-if="items.length" :items="items" />
    </div>
  </section>
</template>
