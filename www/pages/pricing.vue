<script setup lang="ts">
// 价格营销页（数据驱动）：Hero（试用/赠送徽章）+ 资源价格 + 阶梯优惠 + 赠送横幅 + 试用 + FAQ + CTA。
const { t } = useGp()
const { gift, trial } = usePricing()
const cfg = useRuntimeConfig()
const p = computed(() => t.value.pricing)
const startUrl = computed(() => (cfg.public.myAppPath as string) || '/my/')

// FAQ 仅展示计费相关问答（按分类/问题文本匹配，中英关键词覆盖）。
const faqKeywords = ['计费', '价格', '费用', '付费', '充值', '余额', '试用', '赠送', '退款', '折扣', '席位', '开机',
  'pric', 'pay', 'cost', 'bill', 'wallet', 'trial', 'refund', 'discount', 'seat', 'runtime']

useHead({ title: () => `${t.value.pricing.title} — Gloryphone` })
</script>

<template>
  <div style="padding-top: 24px">
    <!-- Hero -->
    <section class="section pricing-hero" style="padding-top: 56px; padding-bottom: 48px">
      <div class="container" style="text-align: center">
        <span class="eyebrow">{{ p.eyebrow }}</span>
        <h1 class="section-title" style="font-size: clamp(32px, 4vw, 52px)">{{ p.title }}</h1>
        <p class="section-sub" style="margin-left: auto; margin-right: auto">{{ p.sub }}</p>
        <div class="pricing-badges">
          <span v-if="trial" class="pill">{{ p.badgeTrial }}</span>
          <span v-if="gift > 0" class="pill">{{ p.badgeGift }}</span>
        </div>
        <div class="hero-ctas" style="justify-content: center; margin-top: 24px">
          <a :href="startUrl" class="btn btn-primary btn-lg">{{ p.ctaStart }} <GpIcon name="arrow" /></a>
          <a href="#discounts" class="btn btn-ghost btn-lg">{{ p.ctaMore }}</a>
        </div>
      </div>
    </section>

    <!-- 三种资源价格 -->
    <section class="section" style="padding-top: 16px">
      <div class="container" style="text-align: center">
        <header>
          <h2 class="section-title">{{ p.resTitle }}</h2>
          <p class="section-sub" style="margin-left: auto; margin-right: auto">{{ p.resSub }}</p>
        </header>
        <PricingResourceCards style="margin-top: 36px" />
      </div>
    </section>

    <div id="discounts">
      <PricingDiscountSection />
    </div>
    <PricingTrialSection />
    <FaqSection :keywords="faqKeywords" />
    <!-- 购买席位赠送时长：放到最后作为收尾 CTA -->
    <PricingGiftSection />
  </div>
</template>

<style scoped>
/* Hero 背景：双色 accent 光晕 + 渐隐点阵，避免大面积留白单调。 */
.pricing-hero {
  position: relative;
  overflow: hidden;
  background:
    radial-gradient(640px 300px at 15% -10%, rgb(var(--accent) / 0.16), transparent 60%),
    radial-gradient(640px 320px at 85% 0%, rgb(var(--accent-strong) / 0.12), transparent 62%);
}
.pricing-hero::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image: radial-gradient(rgb(var(--fg) / 0.05) 1px, transparent 1px);
  background-size: 22px 22px;
  -webkit-mask-image: radial-gradient(ellipse 70% 70% at 50% 0%, #000 25%, transparent 75%);
  mask-image: radial-gradient(ellipse 70% 70% at 50% 0%, #000 25%, transparent 75%);
  opacity: 0.7;
  pointer-events: none;
}
.pricing-hero > .container {
  position: relative;
  z-index: 1;
}
.pricing-badges {
  margin-top: 18px;
  display: flex;
  gap: 10px;
  justify-content: center;
  flex-wrap: wrap;
}
.pill {
  padding: 8px 16px;
  border-radius: 999px;
  background: rgb(var(--accent) / 0.10);
  color: var(--accent-color);
  font-weight: 600;
  font-size: 14px;
  border: 1px solid rgb(var(--accent) / 0.25);
}
</style>
