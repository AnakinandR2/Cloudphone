<script setup lang="ts">
const { t } = useGp()
const localePath = useLocalePath()
</script>

<template>
  <NuxtLink :to="localePath('/') + '#pricing'" class="promo-banner" :aria-label="`${t.promo.tag} — ${t.promo.title}`">
    <div class="container promo-banner__inner">
      <span class="promo-banner__tag">
        <span class="promo-banner__pulse" aria-hidden="true" />
        {{ t.promo.tag }}
      </span>
      <span class="promo-banner__title">{{ t.promo.title }}</span>
      <span class="promo-banner__body">{{ t.promo.body }}</span>
      <span class="promo-banner__countdown" aria-hidden="true">{{ t.promo.countdown }}</span>
      <span class="promo-banner__cta">
        {{ t.promo.cta }}
        <GpIcon name="arrow-sm" style="width: 14px; height: 14px" />
      </span>
    </div>
  </NuxtLink>
</template>
