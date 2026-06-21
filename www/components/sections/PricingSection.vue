<script setup lang="ts">
// 首页价格引流区块：精简版——3 张资源卡 + 「查看完整价格」跳 /pricing。
const { t } = useGp()
const p = computed(() => t.value.pricing)
</script>

<template>
  <section id="pricing" class="section" style="background: rgb(var(--bg-sunken))">
    <div class="container" style="text-align: center">
      <header>
        <span class="eyebrow">{{ p.eyebrow }}</span>
        <h2 class="section-title">{{ p.title }}</h2>
        <p class="section-sub" style="margin-left: auto; margin-right: auto">{{ p.sub }}</p>
      </header>

      <PricingResourceCards style="margin-top: 36px" />

      <div style="margin-top: 32px">
        <NuxtLink to="/pricing" class="btn btn-primary btn-lg">
          {{ p.teaserMore }} <GpIcon name="arrow" />
        </NuxtLink>
      </div>
    </div>
  </section>
</template>
