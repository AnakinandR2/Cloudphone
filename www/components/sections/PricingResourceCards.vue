<script setup lang="ts">
// 三种可购买资源的价格卡（席位 / 包月开机数 / 临时时长）。数字来自 usePricing，文案来自 GP_CONTENT。
const { t } = useGp()
const { lowest, yuan } = usePricing()
const p = computed(() => t.value.pricing)

// 单价取「折后最低单价」（叠加当前最深折扣方式），更有吸引力。
const cards = computed(() => [
  { name: p.value.seat.name, desc: p.value.seat.desc, feats: p.value.seat.feats, cents: lowest.value.seat, per: `/${p.value.perSeat}/${p.value.unitMonth}` },
  { name: p.value.boot.name, desc: p.value.boot.desc, feats: p.value.boot.feats, cents: lowest.value.boot_slot, per: `/${p.value.perSlot}/${p.value.unitDay}` },
  { name: p.value.rt.name, desc: p.value.rt.desc, feats: p.value.rt.feats, cents: lowest.value.runtime, per: `/${p.value.unitMinute}` },
])
</script>

<template>
  <div class="pricing-grid" style="text-align: left; grid-template-columns: repeat(3, 1fr)">
    <div v-for="(c, i) in cards" :key="i" class="price-card">
      <h4>{{ c.name }}</h4>
      <p class="desc">{{ c.desc }}</p>
      <div class="price">
        <span class="per">{{ p.fromLabel }}</span>
        <span class="amt">{{ p.currency }}{{ yuan(c.cents) }}</span>
        <span class="per">{{ c.per }}</span>
      </div>
      <ul>
        <li v-for="(f, j) in c.feats" :key="j">
          <GpIcon name="check" style="width: 14px; height: 14px" /><span>{{ f }}</span>
        </li>
      </ul>
    </div>
  </div>
</template>
