<script setup lang="ts">
const { t } = useGp()
useHead({ title: () => `${t.value.pricing.title} — Gloryphone` })
</script>

<template>
  <div style="padding-top: 24px">
    <PricingSection />
    <CompareSection />
    <FaqSection />
    <CtaSection />
  </div>
</template>
