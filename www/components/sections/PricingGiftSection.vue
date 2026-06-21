<script setup lang="ts">
// 赠送福利横幅：每席位每月赠送的临时开机时长（值为 0 时隐藏）。
const { t } = useGp()
const { gift, fill } = usePricing()
const p = computed(() => t.value.pricing)

const title = computed(() => fill(p.value.giftTitle, { min: gift.value }))
const example = computed(() => fill(p.value.giftExample, { total: gift.value * 6 }))
</script>

<template>
  <section v-if="gift > 0" class="section">
    <div class="container">
      <div class="gift-banner">
        <span class="eyebrow">{{ p.giftEyebrow }}</span>
        <h2 class="section-title" style="margin-top: 14px">{{ title }}</h2>
        <p class="section-sub" style="margin-left: auto; margin-right: auto">{{ p.giftDesc }}</p>
        <div class="gift-example">{{ example }}</div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.gift-banner {
  position: relative;
  text-align: center;
  padding: 48px 28px;
  border-radius: var(--radius-xl);
  background: linear-gradient(135deg, rgb(var(--accent) / 0.12), rgb(var(--accent) / 0.04));
  border: 1px solid rgb(var(--accent) / 0.25);
  box-shadow: var(--shadow-glow);
}
.gift-example {
  display: inline-block;
  margin-top: 22px;
  padding: 12px 20px;
  border-radius: var(--radius-full);
  background: var(--accent-color);
  color: rgb(var(--accent-fg));
  font-weight: 600;
}
</style>
