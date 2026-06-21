<script setup lang="ts">
// 阶梯优惠：用 tab 切换三种购买项（实例席位 / 包月开机数 / 临时开机时长），各自展示折扣梯度。
//  - 席位 / 包月数：数量阶梯 + 时长折扣两列；临时时长：时长包数量阶梯一列。
type TabKey = 'seat' | 'boot_slot' | 'runtime'

const { t } = useGp()
const { pricing, discountLabel } = usePricing()
const p = computed(() => t.value.pricing)

const active = ref<TabKey>('seat')
const tabs = computed(() => [
  { key: 'seat' as TabKey, label: p.value.subjSeat },
  { key: 'boot_slot' as TabKey, label: p.value.subjBoot },
  { key: 'runtime' as TabKey, label: p.value.subjRuntime },
])

interface DiscItem { cond: string, bps: number }
interface DiscCol { head: string, items: DiscItem[] }

const cols = computed<DiscCol[]>(() => {
  if (active.value === 'runtime') {
    const packs = (pricing.value.runtime_pack.packs ?? [])
      .filter(x => x.discount_bps < 10000)
      .map(x => ({ cond: `≥ ${x.minutes} ${p.value.unitMinute}`, bps: x.discount_bps }))
    return [{ head: p.value.discPackHead, items: packs }].filter(c => c.items.length)
  }
  const kp = pricing.value.kinds[active.value]
  const unit = active.value === 'seat' ? p.value.perSeat : p.value.perSlot
  const durUnit = active.value === 'seat' ? p.value.unitMonth : p.value.unitDay
  const qty = (kp?.qty_tiers ?? [])
    .filter(x => x.discount_bps < 10000)
    .map(x => ({ cond: `≥ ${x.min_quantity} ${unit}`, bps: x.discount_bps }))
  const dur = (kp?.duration_options ?? [])
    .filter(x => x.discount_bps < 10000)
    .map(x => ({ cond: `${x.value} ${durUnit}`, bps: x.discount_bps }))
  return [
    { head: p.value.discQtyHead, items: qty },
    { head: p.value.discDurHead, items: dur },
  ].filter(c => c.items.length)
})

const hasDiscount = (arr?: { discount_bps: number }[]) => (arr ?? []).some(x => x.discount_bps < 10000)
const anyDiscount = computed(() => {
  const k = pricing.value.kinds
  return hasDiscount(k.seat?.qty_tiers) || hasDiscount(k.seat?.duration_options)
    || hasDiscount(k.boot_slot?.qty_tiers) || hasDiscount(k.boot_slot?.duration_options)
    || hasDiscount(pricing.value.runtime_pack.packs)
})
</script>

<template>
  <section v-if="anyDiscount" class="section" style="background: rgb(var(--bg-sunken))">
    <div class="container" style="text-align: center">
      <header>
        <span class="eyebrow">{{ p.discEyebrow }}</span>
        <h2 class="section-title">{{ p.discTitle }}</h2>
        <p class="section-sub" style="margin-left: auto; margin-right: auto">{{ p.discSub }}</p>
      </header>

      <div class="disc-tabs">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          :class="['disc-tab', { active: active === tab.key }]"
          @click="active = tab.key"
        >
          {{ tab.label }}
        </button>
      </div>

      <div class="disc-grid">
        <div v-for="(col, i) in cols" :key="i" class="card disc-card">
          <h4>{{ col.head }}</h4>
          <ul class="disc-list">
            <li v-for="(it, j) in col.items" :key="j">
              <span class="disc-cond">{{ it.cond }}</span>
              <span class="disc-badge">{{ discountLabel(it.bps) }}</span>
            </li>
          </ul>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.disc-tabs {
  display: inline-flex;
  gap: 6px;
  margin-top: 30px;
  padding: 6px;
  border-radius: var(--radius-full);
  background: rgb(var(--bg-inset));
}
.disc-tab {
  padding: 8px 18px;
  border: none;
  border-radius: var(--radius-full);
  font-weight: 600;
  color: rgb(var(--fg-muted));
  background: transparent;
  cursor: pointer;
  transition: all 0.2s ease;
}
.disc-tab.active {
  background: var(--accent-color);
  color: rgb(var(--accent-fg));
}
.disc-grid {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 20px;
  margin-top: 28px;
  text-align: left;
}
.disc-card {
  flex: 1 1 300px;
  max-width: 420px;
}
.disc-list {
  list-style: none;
  padding: 0;
  margin: 18px 0 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.disc-list li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 14px;
  border-radius: var(--radius-sm);
  background: rgb(var(--bg-inset));
}
.disc-cond { font-weight: 600; }
.disc-badge { color: var(--accent-color); font-weight: 700; }
</style>
