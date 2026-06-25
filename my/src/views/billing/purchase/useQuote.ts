import type { BizType, PaymentMethod, QuoteReq2, QuoteResult2 } from '@/types/billing'
import { ref, watch } from 'vue'
import billingApi from '@/api/modules/billing'

// 按选中 code 从 payment_methods 反查选中的支付方式；命中即返回整个 PaymentMethod
// （上层据此拿到 fee 费率/满额免阈值/logo_url），未命中（未选/已删）返回 null
// （OrderSummary 视为无手续费、无 logo）。
export function feeOf(
  methods: PaymentMethod[],
  code: string,
): PaymentMethod | null {
  return methods.find(m => m.code === code) ?? null
}

// 报价一律走 POST /billing/quote（服务端权威计价，前端不自算价）。
// 入参变化时防抖请求；返回 quote / loading 供面板与 OrderSummary 使用。
export function useQuote(buildReq: () => QuoteReq2 | null) {
  const quote = ref<QuoteResult2 | null>(null)
  const loading = ref(false)
  let timer: ReturnType<typeof setTimeout> | null = null
  let seq = 0

  async function run() {
    const req = buildReq()
    if (!req) {
      quote.value = null
      loading.value = false
      return
    }
    const my = ++seq
    loading.value = true
    try {
      const res = await billingApi.quote2(req)
      if (my !== seq) return // 丢弃过期响应
      quote.value = res.data
    }
    catch {
      if (my === seq) quote.value = null
    }
    finally {
      if (my === seq) loading.value = false
    }
  }

  function schedule() {
    if (timer) clearTimeout(timer)
    timer = setTimeout(run, 250)
  }

  // 依赖由 buildReq 内的响应式读取驱动
  watch(buildReq as (typeof buildReq), schedule, { immediate: true, deep: true })

  return { quote, loading, refresh: run }
}

// biz_type 推导：kind + 新购/续费。
export function bizTypeOf(kind: 'seat' | 'boot_slot', mode: 'new' | 'renew'): BizType {
  if (kind === 'seat') return mode === 'new' ? 'seat_new' : 'seat_renew'
  return mode === 'new' ? 'boot_slot_new' : 'boot_slot_renew'
}
