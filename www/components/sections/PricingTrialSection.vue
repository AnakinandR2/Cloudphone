<script setup lang="ts">
// 新用户免费试用：CTA 横幅样式——左侧为赠送内容清单，右侧为「注册领取」。
// 展示被 admin 标记「营销展示」且启用的单条试用策略（无则整段隐藏）。
import type { TrialItem } from '~/types/pricing'

const { t } = useGp()
const { trial } = usePricing()
const cfg = useRuntimeConfig()
const p = computed(() => t.value.pricing)
const registerUrl = computed(() => (cfg.public.myAppPath as string) || '/my/')

function subjLabel(s: string): string {
  if (s === 'seat') return p.value.subjSeat
  if (s === 'boot_slot') return p.value.subjBoot
  return p.value.subjRuntime
}
function unitWord(s: string): string {
  if (s === 'runtime_minute') return p.value.minuteWord
  if (s === 'boot_slot') return p.value.perSlot
  return p.value.perSeat
}
function itemText(it: TrialItem): string {
  const base = `${subjLabel(it.subject)} × ${it.quantity} ${unitWord(it.subject)}`
  if (it.subject !== 'runtime_minute' && it.expire_days > 0) {
    return `${base}（${it.expire_days} ${p.value.daysWord}）`
  }
  return base
}
</script>

<template>
  <section v-if="trial" class="section" style="padding-top: 0; padding-bottom: 0">
    <div class="container">
      <div class="cta-banner">
        <div>
          <h2>{{ p.trialTitle }}</h2>
          <p>{{ p.trialSub }}</p>
          <ul class="trial-cta-items">
            <li v-for="(it, i) in trial.items" :key="i">
              <GpIcon name="check-bold" style="width: 16px; height: 16px" /><span>{{ itemText(it) }}</span>
            </li>
          </ul>
        </div>
        <div class="actions">
          <a :href="registerUrl" class="btn btn-primary btn-lg">{{ p.trialCta }} <GpIcon name="arrow" /></a>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.trial-cta-items {
  list-style: none;
  padding: 0;
  margin: 22px 0 0;
  display: flex;
  flex-wrap: wrap;
  gap: 10px 22px;
}
.trial-cta-items li {
  display: flex;
  align-items: center;
  gap: 8px;
  color: rgb(255 255 255 / 0.92);
  font-weight: 600;
}
.trial-cta-items :deep(svg) { color: var(--accent-color); }
</style>
