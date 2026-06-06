<script setup lang="ts">
const { t } = useGp()
const yearly = ref(false)

function priceVal(price: number | string) {
  if (typeof price !== 'number') return price
  return yearly.value ? Math.round(price * 0.8) : price
}
</script>

<template>
  <section class="section" id="pricing" style="background: rgb(var(--bg-sunken))">
    <div class="container" style="text-align: center">
      <header>
        <span class="eyebrow">{{ t.pricing.eyebrow }}</span>
        <h2 class="section-title">{{ t.pricing.title }}</h2>
        <p class="section-sub" style="margin-left: auto; margin-right: auto">{{ t.pricing.sub }}</p>
      </header>

      <div class="pricing-toggle" style="display: inline-flex">
        <button :class="{ active: !yearly }" @click="yearly = false">{{ t.pricing.monthly }}</button>
        <button :class="{ active: yearly }" @click="yearly = true">{{ t.pricing.yearly }}<span class="save">{{ t.pricing.save }}</span></button>
      </div>

      <div class="pricing-grid" style="text-align: left">
        <div v-for="(p, i) in t.pricing.plans" :key="i" class="price-card" :class="{ featured: p.featured }">
          <span v-if="p.tag" class="tag">{{ p.tag }}</span>
          <h4>{{ p.name }}</h4>
          <p class="desc">{{ p.desc }}</p>
          <div class="price">
            <template v-if="typeof p.price === 'number'">
              <span class="amt">{{ p.price === 0 ? t.pricing.currency + '0' : t.pricing.currency + priceVal(p.price) }}</span>
              <span class="per">{{ p.per }}</span>
            </template>
            <span v-else class="amt">{{ p.price }}</span>
          </div>
          <span v-if="yearly && typeof p.price === 'number' && p.price > 0" class="strike">{{ t.pricing.currency }}{{ p.price }}{{ p.per }}</span>
          <span v-else style="height: 18px; display: block" />
          <ul>
            <li v-for="(f, j) in p.features" :key="j"><GpIcon name="check" style="width: 14px; height: 14px" /><span>{{ f }}</span></li>
          </ul>
          <a href="#" class="btn cta" :class="p.featured ? 'btn-primary' : 'btn-quiet'">{{ p.cta }}</a>
        </div>
      </div>
    </div>
  </section>
</template>
