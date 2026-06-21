import { defineFakeRoute } from 'vite-plugin-fake-server/client'

function ok<T>(data: T, message = '成功') {
  return { code: 0, message, data }
}

// ---- Orders（购买与费用重构新形状：biz_type + status unpaid/paid/expired）----
const orders: any[] = [
  { id: 9001, user_id: 101, biz_type: 'seat_new', status: 'paid', pay_method: 'balance', total_cents: 226800, paid_at: '2026-06-01T10:00:00Z', created_at: '2026-06-01T09:55:00Z', expired_at: null },
  { id: 9002, user_id: 102, biz_type: 'recharge', status: 'unpaid', pay_method: 'wechat', total_cents: 10000, paid_at: null, created_at: '2026-06-01T11:00:00Z', expired_at: null },
  { id: 9003, user_id: 101, biz_type: 'runtime_pack', status: 'paid', pay_method: 'alipay', total_cents: 10800, paid_at: '2026-06-02T14:30:00Z', created_at: '2026-06-02T14:20:00Z', expired_at: null },
  { id: 9004, user_id: 103, biz_type: 'boot_slot_new', status: 'unpaid', pay_method: 'alipay', total_cents: 12600, paid_at: null, created_at: '2026-06-03T08:00:00Z', expired_at: null },
  { id: 9005, user_id: 104, biz_type: 'seat_renew', status: 'expired', pay_method: 'wechat', total_cents: 3000, paid_at: null, created_at: '2026-06-04T16:00:00Z', expired_at: '2026-06-04T17:00:00Z' },
]

// ---- Biz Order Items（订单项明细，按订单 id 索引；GET /biz-orders/:id 返回）----
const orderItems: Record<number, any[]> = {
  9001: [
    { target_kind: 'seat', quantity: 10, duration_value: 12, duration_unit: 'month', unit_price_cents: 3000, qty_discount_bps: 9000, duration_discount_bps: 7000, amount_cents: 226800 },
  ],
  9002: [], // recharge：无订单项
  9003: [
    { target_kind: 'runtime_minute', quantity: 600, duration_value: 0, duration_unit: '', unit_price_cents: 20, qty_discount_bps: 9000, duration_discount_bps: 10000, amount_cents: 10800 },
  ],
  9004: [
    { target_kind: 'boot_slot', quantity: 3, duration_value: 30, duration_unit: 'day', unit_price_cents: 2000, qty_discount_bps: 10000, duration_discount_bps: 9000, amount_cents: 12600 },
  ],
  9005: [
    { target_kind: 'seat', quantity: 1, duration_value: 1, duration_unit: 'month', unit_price_cents: 3000, qty_discount_bps: 10000, duration_discount_bps: 10000, amount_cents: 3000 },
  ],
}

// ---- Account (userId=101) ----
const accountStore: Record<number, any> = {
  101: {
    account: { id: 1, user_id: 101, balance_cents: 15000, created_at: '2026-01-01T00:00:00Z', updated_at: '2026-06-01T10:00:00Z' },
    ledger: [
      { id: 1, user_id: 101, subject: 'balance', type: 'topup', delta: 20000, balance_after: 20000, reason: '充值', order_id: 0, operator: 'user', created_at: '2026-05-01T09:00:00Z' },
      { id: 2, user_id: 101, subject: 'balance', type: 'consume', delta: -3000, balance_after: 17000, reason: '订单 ORD202606010001', order_id: 1, operator: 'system', created_at: '2026-06-01T10:00:00Z' },
      { id: 3, user_id: 101, subject: 'balance', type: 'consume', delta: -2000, balance_after: 15000, reason: '订单 ORD202606020001', order_id: 3, operator: 'system', created_at: '2026-06-02T14:30:00Z' },
    ],
    ledger_total: 3,
    capacities: { instance_seat: 5, boot_seat: 10, runtime_minute: 3600 },
    // 新模型容量快照（权威），前端容量卡片读这里。
    capacities_v2: { seat: 2, boot_slot: 1, runtime_minute: 600 },
  },
}

// ---- Trial Policies ----
let trials: any[] = [
  { id: 1, code: 'new_user_trial', name: '新用户试用', enabled: true, per_user_limit: 1, allow_new_user: true, invite_code: '', items: [
    { id: 1, policy_id: 1, subject: 'seat', quantity: 1, expire_days: 7 },
    { id: 2, policy_id: 1, subject: 'runtime_minute', quantity: 600, expire_days: 0 },
  ], created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' },
  { id: 2, code: 'invite_trial', name: '邀请码大礼包', enabled: true, per_user_limit: 1, allow_new_user: false, invite_code: 'GLORY2026', items: [
    { id: 3, policy_id: 2, subject: 'seat', quantity: 2, expire_days: 30 },
    { id: 4, policy_id: 2, subject: 'runtime_minute', quantity: 1440, expire_days: 30 },
    { id: 5, policy_id: 2, subject: 'boot_slot', quantity: 1, expire_days: 30 },
  ], created_at: '2026-03-01T00:00:00Z', updated_at: '2026-03-01T00:00:00Z' },
]
let trialNextId = 3
let trialItemNextId = 6

const trialGrantsStore: Record<number, any[]> = {
  1: [
    { id: 1, policy_id: 1, user_id: 201, subject: 'seat', quantity: 1, created_at: '2026-05-10T08:00:00Z' },
    { id: 2, policy_id: 1, user_id: 202, subject: 'seat', quantity: 1, created_at: '2026-05-15T10:30:00Z' },
  ],
  2: [
    { id: 3, policy_id: 2, user_id: 203, subject: 'runtime_minute', quantity: 1440, created_at: '2026-06-01T12:00:00Z' },
  ],
}
let grantNextId = 4

// ── 购买与费用重构 · 配置（契约 §2）─────────────────────────────────────────
// 定价（按 kind）。后端形状为 { kinds: { seat, boot_slot } }，KindPricing 无 kind 字段。
const pricingKinds: any = {
  seat: {
    unit_price_cents: 3000,
    unit_label: '台',
    qty_options: [1, 2, 5, 10, 50, 100, 500, 1000],
    qty_tiers: [{ min_quantity: 10, discount_bps: 9000 }, { min_quantity: 100, discount_bps: 8000 }],
    duration_unit: 'month',
    duration_options: [{ value: 1, discount_bps: 10000 }, { value: 3, discount_bps: 8500 }, { value: 12, discount_bps: 7000 }],
    notice: '云手机为固定套餐，购买席位即获得「能存在」的权利。',
    billing_note: '席位按月计费，数量阶梯折扣 × 时长折扣相乘。',
  },
  boot_slot: {
    unit_price_cents: 2000,
    unit_label: '个',
    qty_options: [1, 2, 5, 10, 50, 100],
    qty_tiers: [{ min_quantity: 10, discount_bps: 9000 }],
    duration_unit: 'day',
    duration_options: [{ value: 7, discount_bps: 10000 }, { value: 30, discount_bps: 9000 }],
    notice: '包月开机数决定能同时开机的实例数，多实例轮流共享。',
    billing_note: '包月开机数按天计费，名额满后开机改扣临时时长。',
  },
}

// 临时时长配置（后端 RuntimePackCfg，扁平，含 notice）
let runtimeCfg: any = {
  unit_price_cents_per_minute: 20,
  packs: [{ minutes: 600, discount_bps: 10000 }, { minutes: 3000, discount_bps: 9000 }],
  min_minutes: 60,
  daily_cap_minutes: 200,
  recycle_retention_days: 30,
  notice: '临时开机时长无期限，用完为止；每台每天封顶 200 分钟。',
}

// 支付方式
let paymentMethods: any[] = [
  { code: 'balance', name: '余额支付', enabled: true, sort: 0 },
  { code: 'wechat', name: '微信支付', enabled: true, sort: 1 },
  { code: 'alipay', name: '支付宝', enabled: true, sort: 2 },
]

// 充值预设
let rechargePresets: number[] = [1000, 5000, 10000, 50000]

// 须知文案
let notices: any = {
  seat: { notice: '云手机为固定套餐，用完为止。', billing_note: '席位实例计费说明……' },
  boot_slot: { notice: '包月开机数说明……', billing_note: '包月开机数计费说明……' },
  runtime_pack: { notice: '临时开机时长无期限，用完为止；每台每天封顶 200 分钟。' },
}

export default defineFakeRoute([
  // ---- Biz Orders（新模型，无订单项随列表返回）----
  {
    url: '/v1/admin/billing/biz-orders',
    method: 'get',
    response: ({ query }) => {
      let list = [...orders]
      if (query.userId) list = list.filter(o => o.user_id === Number(query.userId))
      if (query.status) list = list.filter(o => o.status === query.status)
      const page = Number(query.page) || 1
      const size = Number(query.size) || 20
      const total = list.length
      const start = (page - 1) * size
      return ok({ list: list.slice(start, start + size), total })
    },
  },
  {
    url: '/v1/admin/billing/biz-orders/:id',
    method: 'get',
    response: ({ params }) => {
      const id = Number(params.id)
      const order = orders.find(o => o.id === id)
      if (!order) return { code: 404, message: 'not found', data: null }
      // 后端 AdminBizOrderDetail 返回扁平订单字段 + items（含 user_id）。
      return ok({ ...order, items: orderItems[id] ?? [] })
    },
  },
  {
    url: '/v1/admin/billing/biz-orders/:id/mark-paid',
    method: 'post',
    response: ({ params }) => {
      const id = Number(params.id)
      const idx = orders.findIndex(o => o.id === id)
      if (idx < 0) return { code: 404, message: 'not found', data: null }
      orders[idx] = { ...orders[idx], status: 'paid', paid_at: new Date().toISOString() }
      // 后端 AdminMarkBizOrderPaid 仅返回订单本身（无 items）。
      return ok(orders[idx])
    },
  },
  // ---- Accounts ----
  {
    url: '/v1/admin/billing/accounts/:userId',
    method: 'get',
    response: ({ params, query }) => {
      const userId = Number(params.userId)
      const view = accountStore[userId]
      if (!view) {
        // Return a default empty account for unknown users
        return ok({
          account: { id: 0, user_id: userId, balance_cents: 0, created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
          ledger: [],
          ledger_total: 0,
          capacities: { instance_seat: 0, boot_seat: 0, runtime_minute: 0 },
          capacities_v2: { seat: 0, boot_slot: 0, runtime_minute: 0 },
        })
      }
      const page = Number(query.page) || 1
      const size = Number(query.size) || 20
      const total = view.ledger.length
      const start = (page - 1) * size
      return ok({ ...view, ledger: view.ledger.slice(start, start + size), ledger_total: total })
    },
  },
  {
    url: '/v1/admin/billing/accounts/:userId/adjust',
    method: 'post',
    response: ({ params, body }) => {
      const userId = Number(params.userId)
      const d = body as any
      if (!accountStore[userId]) {
        accountStore[userId] = {
          account: { id: 0, user_id: userId, balance_cents: 0, created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
          ledger: [],
          ledger_total: 0,
          capacities: { instance_seat: 0, boot_seat: 0, runtime_minute: 0 },
          capacities_v2: { seat: 0, boot_slot: 0, runtime_minute: 0 },
        }
      }
      const acct = accountStore[userId].account
      acct.balance_cents += d.delta_cents
      acct.updated_at = new Date().toISOString()
      const entry = {
        id: accountStore[userId].ledger.length + 1,
        user_id: userId,
        subject: 'balance',
        type: d.delta_cents >= 0 ? 'adjust_grant' : 'adjust_deduct',
        delta: d.delta_cents,
        balance_after: acct.balance_cents,
        reason: d.reason || '',
        order_id: 0,
        operator: 'admin',
        created_at: new Date().toISOString(),
      }
      accountStore[userId].ledger.unshift(entry)
      accountStore[userId].ledger_total = accountStore[userId].ledger.length
      return ok(acct)
    },
  },
  {
    url: '/v1/admin/billing/accounts/:userId/adjust-resource',
    method: 'post',
    response: ({ params, body }) => {
      const userId = Number(params.userId)
      const d = body as any
      if (!accountStore[userId]) {
        accountStore[userId] = {
          account: { id: 0, user_id: userId, balance_cents: 0, created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
          ledger: [],
          ledger_total: 0,
          capacities: { instance_seat: 0, boot_seat: 0, runtime_minute: 0 },
          capacities_v2: { seat: 0, boot_slot: 0, runtime_minute: 0 },
        }
      }
      // V2：subject ∈ seat/boot_slot/runtime_minute。seat/boot_slot 用 quantity，runtime_minute 用 minutes。
      const caps = accountStore[userId].capacities
      const capsV2 = accountStore[userId].capacities_v2
      if (d.subject === 'seat') {
        caps.instance_seat += Number(d.quantity) || 0
        capsV2.seat += Number(d.quantity) || 0
      }
      else if (d.subject === 'boot_slot') {
        caps.boot_seat += Number(d.quantity) || 0
        capsV2.boot_slot += Number(d.quantity) || 0
      }
      else if (d.subject === 'runtime_minute') {
        const m = Number(d.minutes) || 0
        caps.runtime_minute += m
        capsV2.runtime_minute += m
      }
      return ok(null)
    },
  },
  // ---- Trial Policies ----
  {
    url: '/v1/admin/billing/trials',
    method: 'get',
    response: () => ok(trials),
  },
  {
    url: '/v1/admin/billing/trials',
    method: 'post',
    response: ({ body }) => {
      const d = body as any
      const id = trialNextId++
      const items = (d.items || []).map((it: any) => ({ id: trialItemNextId++, policy_id: id, subject: it.subject, quantity: it.quantity, expire_days: it.expire_days || 0 }))
      const policy = {
        id,
        code: d.code,
        name: d.name,
        enabled: d.enabled !== false,
        per_user_limit: d.per_user_limit || 1,
        allow_new_user: d.allow_new_user !== false,
        invite_code: d.invite_code || '',
        items,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      }
      trials.push(policy)
      return ok(policy)
    },
  },
  {
    url: '/v1/admin/billing/trials/:id',
    method: 'put',
    response: ({ params, body }) => {
      const id = Number(params.id)
      const idx = trials.findIndex(t => t.id === id)
      if (idx < 0) return { code: 404, message: 'not found', data: null }
      const d = body as any
      const next = { ...trials[idx], ...d, updated_at: new Date().toISOString() }
      if (d.items) {
        next.items = d.items.map((it: any) => ({ id: trialItemNextId++, policy_id: id, subject: it.subject, quantity: it.quantity, expire_days: it.expire_days || 0 }))
      }
      trials[idx] = next
      return ok(next)
    },
  },
  {
    url: '/v1/admin/billing/trials/:id',
    method: 'delete',
    response: ({ params }) => {
      const id = Number(params.id)
      trials = trials.filter(t => t.id !== id)
      return ok(null)
    },
  },
  {
    url: '/v1/admin/billing/trials/:id/eligibility',
    method: 'post',
    response: ({ params, body }) => {
      const id = Number(params.id)
      const d = body as any
      if (!trialGrantsStore[id]) trialGrantsStore[id] = []
      const policy = trials.find(t => t.id === id)
      const claimId = grantNextId++
      const items = policy?.items?.length ? policy.items : [{ subject: 'seat', quantity: 1 }]
      for (const it of items) {
        trialGrantsStore[id].push({
          id: grantNextId++,
          claim_id: claimId,
          policy_id: id,
          user_id: d.user_id,
          subject: it.subject,
          quantity: it.quantity,
          created_at: new Date().toISOString(),
        })
      }
      return ok(null)
    },
  },
  {
    url: '/v1/admin/billing/trials/:id/grants',
    method: 'get',
    response: ({ params }) => {
      const id = Number(params.id)
      return ok(trialGrantsStore[id] || [])
    },
  },
  // ── 配置：定价（{ kinds }）────────────────────────────────────
  {
    url: '/v1/admin/billing/pricing',
    method: 'get',
    response: () => ok({ kinds: pricingKinds }),
  },
  {
    url: '/v1/admin/billing/pricing',
    method: 'put',
    response: ({ body }) => {
      const d = body as any
      if (d?.kinds) Object.assign(pricingKinds, d.kinds)
      return ok({ kinds: pricingKinds })
    },
  },
  // ── 配置：临时时长 ───────────────────────────────────────────
  {
    url: '/v1/admin/billing/runtime-config',
    method: 'get',
    response: () => ok(runtimeCfg),
  },
  {
    url: '/v1/admin/billing/runtime-config',
    method: 'put',
    response: ({ body }) => {
      runtimeCfg = { ...runtimeCfg, ...(body as any) }
      return ok(runtimeCfg)
    },
  },
  // ── 配置：支付方式（{ payment_methods }）─────────────────────
  {
    url: '/v1/admin/billing/payment-methods',
    method: 'get',
    response: () => ok({ payment_methods: [...paymentMethods].sort((a, b) => a.sort - b.sort) }),
  },
  {
    url: '/v1/admin/billing/payment-methods',
    method: 'put',
    response: ({ body }) => {
      const d = body as any
      if (Array.isArray(d.payment_methods)) paymentMethods = d.payment_methods
      return ok({ payment_methods: [...paymentMethods].sort((a, b) => a.sort - b.sort) })
    },
  },
  // ── 配置：充值预设 ───────────────────────────────────────────
  {
    url: '/v1/admin/billing/recharge-presets',
    method: 'get',
    response: () => ok({ presets_cents: rechargePresets }),
  },
  {
    url: '/v1/admin/billing/recharge-presets',
    method: 'put',
    response: ({ body }) => {
      const d = body as any
      if (Array.isArray(d.presets_cents)) rechargePresets = d.presets_cents
      return ok({ presets_cents: rechargePresets })
    },
  },
  // ── 配置：须知文案 ───────────────────────────────────────────
  {
    url: '/v1/admin/billing/notices',
    method: 'get',
    response: () => ok(notices),
  },
  {
    url: '/v1/admin/billing/notices',
    method: 'put',
    response: ({ body }) => {
      notices = { ...notices, ...(body as any) }
      return ok(notices)
    },
  },
])
