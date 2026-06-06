<script setup lang="ts">
const { t } = useGp()
const open = ref(0)
function toggle(i: number) {
  open.value = open.value === i ? -1 : i
}
</script>

<template>
  <section class="section" id="help">
    <div class="container">
      <header style="text-align: center">
        <span class="eyebrow">{{ t.faq.eyebrow }}</span>
        <h2 class="section-title">{{ t.faq.title }}</h2>
      </header>
      <div class="faq-list">
        <div v-for="([q, a], i) in t.faq.items" :key="i" class="faq-item" :class="{ open: open === i }">
          <button class="faq-q" @click="toggle(i)">
            <span>{{ q }}</span>
            <span class="chev"><GpIcon name="plus" style="width: 14px; height: 14px" /></span>
          </button>
          <div class="faq-a"><p style="max-width: 760px">{{ a }}</p></div>
        </div>
      </div>
    </div>
  </section>
</template>
