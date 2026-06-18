<script setup lang="ts">
// /faq —— 常见问题，按分类分组（数据来自内容中台 faq 空间）。
const { t } = useGp()
const { groups, pending, error, refresh } = useFaq()
useHead({ title: () => `${t.value.nav.faq} — Gloryphone` })
</script>

<template>
  <div style="padding-top: 24px">
    <section class="section">
      <div class="container">
        <header style="text-align: center">
          <span class="eyebrow">{{ t.faq.eyebrow }}</span>
          <h2 class="section-title">{{ t.faq.title }}</h2>
        </header>

        <div v-if="error" class="faq-state">
          <p>{{ t.faq.error }}</p>
          <button class="btn btn-quiet btn-sm" @click="refresh()">{{ t.faq.retry }}</button>
        </div>
        <div v-else-if="pending && !groups.length" class="faq-state"><p>…</p></div>
        <div v-else-if="!groups.length" class="faq-state"><p>{{ t.faq.empty }}</p></div>
        <template v-else>
          <div v-for="g in groups" :key="g.category" class="faq-group">
            <h3 v-if="g.category" class="faq-group__title">{{ g.category }}</h3>
            <FaqAccordion :items="g.items" />
          </div>
        </template>
      </div>
    </section>
    <CtaSection />
  </div>
</template>

<style scoped>
.faq-group + .faq-group { margin-top: 28px; }
.faq-group__title {
  max-width: 820px; margin: 0 auto; font-size: 13px; font-weight: 700;
  text-transform: uppercase; letter-spacing: 0.08em; color: var(--accent-color);
  font-family: var(--font-mono);
}
.faq-state { text-align: center; padding: 56px 0; color: rgb(var(--fg-muted)); display: flex; flex-direction: column; align-items: center; gap: 14px; }
</style>
