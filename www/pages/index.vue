<script setup lang="ts">
const { t } = useGp()

useHead({
  title: () => `Gloryphone — ${t.value.hero.title1} ${t.value.hero.titleHL}`,
})
</script>

<template>
  <div>
    <HeroSection />
    <FeaturesSection />
    <UseCasesSection />
    <CompareSection />
    <PricingSection />
    <TestimonialsSection />
    <DownloadsSection />
    <BlogSection />
    <FaqSection />
    <CtaSection />
  </div>
</template>
