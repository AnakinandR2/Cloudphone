import { defineFakeRoute } from 'vite-plugin-fake-server/client'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}
function fail(message: string) {
  return { code: 1, message, data: null }
}
function now() {
  return new Date().toISOString().slice(0, 19).replace('T', ' ')
}

// ---------- 内存状态 ----------

const account = { id: 1, user_id: 1, balance_cents: 1000000, created_at: '2026-01-01 00:00:00', updated_at: now() }

// SKU 定义
const skus = [
  {
    sku: { id: 1, code: 'instance_fee', category: 'instance_fee', name: '云手机实例费', description: '按台月计费，购买后增加实例席位', unit_price_cents: 3000, unit: '台月', listed: true, sort: 1 },
    tiers: [
      { id: 1, sku_id: 1, cycle_months: 1,  min_quantity: 1, discount_bps: 10000 },
      { id: 2, sku_id: 1, cycle_months: 3,  min_quantity: 1, discount_bps: 8500 },
      { id: 3, sku_id: 1, cycle_months: 12, min_quantity: 1, discount_bps: 7000 },
    ],
  },
  {
    sku: { id: 2, code: 'boot_pack', category: 'boot_pack', name: '开机包', description: '按台月购买开机席位，允许同时开机的实例数', unit_price_cents: 2000, unit: '台月', listed: true, sort: 2 },
    tiers: [
      { id: 4, sku_id: 2, cycle_months: 1,  min_quantity: 1, discount_bps: 10000 },
      { id: 5, sku_id: 2, cycle_months: 3,  min_quantity: 1, discount_bps: 8500 },
      { id: 6, sku_id: 2, cycle_months: 12, min_quantity: 1, discount_bps: 7000 },
    ],
  },
  {
    sku: { id: 3, code: 'time_pack', category: 'time_pack', name: '时长包', description: '按开机运行时长（分钟）计费，批量购买有折扣', unit_price_cents: 20, unit: '小时', listed: true, sort: 3 },
    tiers: [
      { id: 7,  sku_id: 3, cycle_months: 0, min_quantity: 1,    discount_bps: 10000 },
      { id: 8,  sku_id: 3, cycle_months: 0, min_quantity: 500,  discount_bps: 9000 },
      { id: 9,  sku_id: 3, cycle_months: 0, min_quantity: 1000, discount_bps: 8000 },
    ],
  },
]

// 权益批次
const entitlementBatches = [
  { id: 1, user_id: 1, subject: 'instance_seat', quantity: 3, used: 1, expire_at: null, source: 'purchase', source_ref: 'ORD-0001', created_at: '2026-01-15 10:00:00' },
  { id: 2, user_id: 1, subject: 'instance_seat', quantity: 2, used: 0, expire_at: '2026-12-31 23:59:59', source: 'trial', source_ref: 'newbie', created_at: '2026-03-01 09:00:00' },
  { id: 3, user_id: 1, subject: 'boot_seat',     quantity: 2, used: 1, expire_at: null, source: 'purchase', source_ref: 'ORD-0001', created_at: '2026-01-15 10:00:00' },
  { id: 4, user_id: 1, subject: 'runtime_minute', quantity: 6000, used: 320, expire_at: '2027-01-15 10:00:00', source: 'purchase', source_ref: 'ORD-0002', created_at: '2026-02-01 08:00:00' },
]

// 试用策略
const trialPolicies = [
  {
    policy: { id: 1, code: 'newbie', name: '新用户试用', enabled: true, grant_subject: 'instance_seat', grant_quantity: 1, grant_expire_days: 7, per_user_limit: 1, allow_new_user: true, invite_code: '' },
    claimable: true,
    need_invite: false,
    claimed_count: 0,
    reason: '',
  },
  {
    policy: { id: 2, code: 'promo', name: '邀请码专属', enabled: true, grant_subject: 'runtime_minute', grant_quantity: 1000, grant_expire_days: 30, per_user_limit: 1, allow_new_user: false, invite_code: '' },
    claimable: false,
    need_invite: true,
    claimed_count: 0,
    reason: '需要邀请码',
  },
]

// 订单
let orderSeq = 100
const orders: Array<{
  order: {
    id: number, order_no: string, user_id: number, status: string, pay_method: string,
    total_cents: number, paid_at: string | null, created_at: string, updated_at: string
  }
  items: Array<{
    id: number, order_id: number, sku_code: string, sku_name: string, category: string,
    cycle_months: number, quantity: number, unit_price_cents: number,
    discount_bps: number, original_cents: number, payable_cents: number
  }>
}> = []

// ---------- 计价逻辑 ----------

function pickTier(skuCode: string, cycleMonths: number, quantity: number) {
  const entry = skus.find(s => s.sku.code === skuCode)
  if (!entry) return null
  const tiers = entry.tiers.filter(t => t.cycle_months === cycleMonths && t.min_quantity <= quantity)
  if (tiers.length === 0) {
    // fallback: any tier for this sku
    const fallback = entry.tiers.filter(t => t.min_quantity <= quantity)
    if (fallback.length === 0) return { discount_bps: 10000 }
    return fallback.reduce((best, t) => t.min_quantity > best.min_quantity ? t : best)
  }
  return tiers.reduce((best, t) => t.min_quantity > best.min_quantity ? t : best)
}

function computeQuote(skuCode: string, cycleMonths: number, quantity: number) {
  const entry = skus.find(s => s.sku.code === skuCode)
  if (!entry) return null
  const { sku } = entry
  const tier = pickTier(skuCode, cycleMonths, quantity)
  const discountBps = tier?.discount_bps ?? 10000
  // billing_units: subscription = cycle_months * quantity; time_pack = quantity
  const billingUnits = sku.category === 'time_pack' ? quantity : cycleMonths * quantity
  const originalCents = sku.unit_price_cents * billingUnits
  const payableCents = Math.round(originalCents * discountBps / 10000)
  return {
    sku_code: sku.code,
    sku_name: sku.name,
    category: sku.category,
    cycle_months: cycleMonths,
    quantity,
    unit_price_cents: sku.unit_price_cents,
    billing_units: billingUnits,
    original_cents: originalCents,
    discount_bps: discountBps,
    payable_cents: payableCents,
  }
}

// ---------- 账本流水 ----------
let ledgerSeq = 1
const ledger: Array<{
  id: number, user_id: number, subject: string, type: string,
  delta: number, balance_after: number, reason: string,
  order_id: number, operator: string, created_at: string
}> = []

function addLedger(subject: string, type: string, delta: number, reason: string, orderId = 0) {
  ledger.unshift({
    id: ledgerSeq++,
    user_id: 1,
    subject,
    type,
    delta,
    balance_after: subject === 'balance' ? account.balance_cents : 0,
    reason,
    order_id: orderId,
    operator: '',
    created_at: now(),
  })
}

// ---------- 路由定义 ----------

export default defineFakeRoute([
  // 账户
  {
    url: '/v1/billing/account',
    method: 'get',
    response: () => ok({ ...account, updated_at: now() }),
  },

  // 充值
  {
    url: '/v1/billing/topup',
    method: 'post',
    response: ({ body }) => {
      const amount = Number(body?.amount_cents) || 0
      if (amount <= 0) return fail('充值金额必须大于0')
      account.balance_cents += amount
      addLedger('balance', 'topup', amount, '手动充值')
      return ok({ ...account, updated_at: now() })
    },
  },

  // 流水
  {
    url: '/v1/billing/ledger',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 20
      let list = [...ledger]
      if (query.subject) list = list.filter(e => e.subject === query.subject)
      if (query.type) list = list.filter(e => e.type === query.type)
      const total = list.length
      return ok({ list: list.slice((page - 1) * size, page * size), total })
    },
  },

  // SKU 目录
  {
    url: '/v1/billing/skus',
    method: 'get',
    response: () => ok(skus),
  },

  // 计价
  {
    url: '/v1/billing/quote',
    method: 'post',
    response: ({ body }) => {
      const result = computeQuote(body?.sku_code, Number(body?.cycle_months ?? 0), Number(body?.quantity ?? 1))
      if (!result) return fail('SKU 不存在')
      return ok(result)
    },
  },

  // 创建订单
  {
    url: '/v1/billing/orders',
    method: 'post',
    response: ({ body }) => {
      const reqItems: Array<{ sku_code: string, cycle_months: number, quantity: number }> = body?.items || []
      const payMethod: string = body?.pay_method || 'balance'
      let totalCents = 0
      const itemSeq = { n: 1 }
      const orderId = ++orderSeq
      const items = reqItems.map((req) => {
        const q = computeQuote(req.sku_code, Number(req.cycle_months), Number(req.quantity))
        if (!q) return null
        totalCents += q.payable_cents
        const entry = skus.find(s => s.sku.code === req.sku_code)
        return {
          id: itemSeq.n++,
          order_id: orderId,
          sku_code: q.sku_code,
          sku_name: q.sku_name,
          category: q.category,
          cycle_months: q.cycle_months,
          quantity: q.quantity,
          unit_price_cents: entry?.sku.unit_price_cents ?? 0,
          discount_bps: q.discount_bps,
          original_cents: q.original_cents,
          payable_cents: q.payable_cents,
        }
      }).filter(Boolean) as typeof orders[0]['items']

      const order = {
        id: orderId,
        order_no: `ORD-${String(orderId).padStart(6, '0')}`,
        user_id: 1,
        status: 'pending',
        pay_method: payMethod,
        total_cents: totalCents,
        paid_at: null as string | null,
        created_at: now(),
        updated_at: now(),
      }
      orders.unshift({ order, items })
      return ok({ order, items })
    },
  },

  // 订单列表
  {
    url: '/v1/billing/orders',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 20
      let list = orders.map(o => o.order)
      if (query.status) list = list.filter(o => o.status === query.status)
      const total = list.length
      return ok({ list: list.slice((page - 1) * size, page * size), total })
    },
  },

  // 订单详情
  {
    url: '/v1/billing/orders/:id',
    method: 'get',
    response: ({ params }) => {
      const entry = orders.find(o => o.order.id === Number(params.id))
      if (!entry) return fail('订单不存在')
      return ok(entry)
    },
  },

  // 支付订单
  {
    url: '/v1/billing/orders/:id/pay',
    method: 'post',
    response: ({ params }) => {
      const entry = orders.find(o => o.order.id === Number(params.id))
      if (!entry) return fail('订单不存在')
      if (entry.order.status === 'paid') return fail('订单已支付')
      // 余额支付：校验并扣余额 + 记流水；微信/支付宝：即时到账桩，不扣余额。
      if (entry.order.pay_method === 'balance') {
        if (account.balance_cents < entry.order.total_cents) return fail('余额不足')
        account.balance_cents -= entry.order.total_cents
        addLedger('balance', 'purchase', -entry.order.total_cents, `支付订单 ${entry.order.order_no}`, entry.order.id)
      }
      entry.order.status = 'paid'
      entry.order.paid_at = now()
      entry.order.updated_at = now()
      return ok({ ...entry })
    },
  },

  // 权益
  {
    url: '/v1/billing/entitlements',
    method: 'get',
    response: () => {
      const instance_seat = entitlementBatches.filter(b => b.subject === 'instance_seat').reduce((s, b) => s + b.quantity - b.used, 0)
      const boot_seat = entitlementBatches.filter(b => b.subject === 'boot_seat').reduce((s, b) => s + b.quantity - b.used, 0)
      const runtime_minute = entitlementBatches.filter(b => b.subject === 'runtime_minute').reduce((s, b) => s + b.quantity - b.used, 0)
      return ok({
        capacities: { instance_seat, boot_seat, runtime_minute },
        batches: entitlementBatches,
      })
    },
  },

  // 试用列表
  {
    url: '/v1/billing/trials',
    method: 'get',
    response: () => ok(trialPolicies),
  },

  // 领取试用
  {
    url: '/v1/billing/trials/:code/claim',
    method: 'post',
    response: ({ params, body }) => {
      const item = trialPolicies.find(t => t.policy.code === params.code)
      if (!item) return fail('试用活动不存在')
      if (item.need_invite && !body?.invite_code) return fail('请填写邀请码')
      item.claimable = false
      item.claimed_count += 1
      return ok(null)
    },
  },
])
