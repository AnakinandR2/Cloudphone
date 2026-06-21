// 价格营销页取数 + 兜底 + 展示 helper。
// 数字来自后端公开接口（经 /_api/* SSR 代理）；后端不可用时用与后端 seed 一致的默认数字兜底。
// 营销文案一律走 GP_CONTENT（useGp），本组合式只产出「数字」与纯展示 helper。
import type { PublicPricing, TrialPolicyOverview, TrialOverview } from '~/types/pricing'

// 默认数字（镜像后端 defaultPricingConfig，保证后端不可用时页面仍可渲染）。
export const DEFAULT_PRICING: PublicPricing = {
  kinds: {
    seat: {
      unit_price_cents: 3000,
      unit_label: '',
      duration_unit: 'month',
      qty_tiers: [{ min_quantity: 10, discount_bps: 9000 }, { min_quantity: 100, discount_bps: 8000 }],
      duration_options: [{ value: 1, discount_bps: 10000 }, { value: 3, discount_bps: 8500 }, { value: 12, discount_bps: 7000 }],
    },
    boot_slot: {
      unit_price_cents: 2000,
      unit_label: '',
      duration_unit: 'day',
      qty_tiers: [{ min_quantity: 10, discount_bps: 9000 }],
      duration_options: [{ value: 7, discount_bps: 10000 }, { value: 30, discount_bps: 9000 }],
    },
  },
  runtime_pack: {
    unit_price_cents_per_minute: 20,
    min_minutes: 60,
    packs: [{ minutes: 600, discount_bps: 10000 }, { minutes: 3000, discount_bps: 9000 }],
    daily_cap_minutes: 200,
    gift_minutes_per_seat_month: 200,
  },
  recharge_presets_cents: [1000, 5000, 10000, 50000],
}

// 分 → 元（去掉无意义小数）。
export function yuan(cents: number): string {
  const v = cents / 100
  return Number.isInteger(v) ? String(v) : v.toFixed(2)
}

// 折扣展示：bps=10000(无折扣) 返回 ''（调用方据此隐藏）。zh→「8.5折」；en→「15% off」。
export function discountLabel(bps: number, lang: string): string {
  if (!bps || bps >= 10000) return ''
  if (lang === 'zh') {
    const zhe = bps / 1000
    return `${Number.isInteger(zhe) ? zhe : zhe.toFixed(1)}折`
  }
  return `${Math.round((10000 - bps) / 100)}% off`
}

// 模板插值：把 "{min}" 之类占位符替换为给定值（GP_CONTENT 文案带数字用）。
export function fill(tpl: string, vars: Record<string, string | number>): string {
  return tpl.replace(/\{(\w+)\}/g, (_, k) => String(vars[k] ?? ''))
}

// 一组档位里的最大折扣 bps（值最小 = 折扣最深）；空数组返回 10000（无折扣）。
export function minBps(tiers: { discount_bps: number }[]): number {
  return tiers.length ? tiers.reduce((m, x) => Math.min(m, x.discount_bps), 10000) : 10000
}

// 叠加多组档位的最深折扣后的单价（分）：base × ∏(最大折扣)。用于资源卡「最低」单价。
function lowestCents(base: number, ...tierGroups: { discount_bps: number }[][]): number {
  const factor = tierGroups.reduce((f, g) => (f * minBps(g)) / 10000, 1)
  return Math.round(base * factor)
}

export function usePricing() {
  const { locale } = useI18n()
  const { data: pricingRaw } = useFetch<PublicPricing | null>('/_api/pricing', { key: 'pricing' })
  const { data: trialRaw } = useFetch<TrialOverview>('/_api/trial-overview', {
    key: 'trial-overview',
    default: () => ({ policy: null }),
  })

  const pricing = computed<PublicPricing>(() => pricingRaw.value ?? DEFAULT_PRICING)
  const trial = computed<TrialPolicyOverview | null>(() => trialRaw.value?.policy ?? null)
  const gift = computed(() => pricing.value.runtime_pack.gift_minutes_per_seat_month)

  // 各资源折后「最低」单价（分）：席位/包月数叠加数量×时长最深折扣；临时时长叠加时长包最深折扣。
  const lowest = computed(() => {
    const seat = pricing.value.kinds.seat
    const boot = pricing.value.kinds.boot_slot
    const rt = pricing.value.runtime_pack
    return {
      seat: seat ? lowestCents(seat.unit_price_cents, seat.qty_tiers, seat.duration_options) : 0,
      boot_slot: boot ? lowestCents(boot.unit_price_cents, boot.qty_tiers, boot.duration_options) : 0,
      runtime: lowestCents(rt.unit_price_cents_per_minute, rt.packs),
    }
  })

  return {
    pricing,
    trial,
    gift,
    lowest,
    yuan,
    fill,
    discountLabel: (bps: number) => discountLabel(bps, locale.value),
  }
}
