<script setup lang="ts">
const { t } = useGp()
useHead({ title: () => `${t.value.faq.title} — Gloryphone` })
</script>

<template>
  <div style="padding-top: 24px">
    <FaqSection />
    <CtaSection />
  </div>
</template>
