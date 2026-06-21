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
function iso(d: Date) {
  return d.toISOString()
}
function addDays(base: Date, days: number) {
  return new Date(base.getTime() + days * 86400000)
}

// ============================================================================
// 购买与费用重构（2026-06-21）mock 状态
// 契约：docs/superpowers/specs/2026-06-21-购买与费用重构-接口契约.md §1
// 旧端点（account/topup/ledger/skus/entitlements/trials）保留供旧视图；
// quote/orders/* 改为新契约形状；新增 overview/purchase-config/license-units/
// runtime/log。
// ============================================================================

// ---------- 钱包/容量内存状态 ----------
const wallet = { balance_cents: 1234500, runtime_minutes_remaining: 860 }

// 授权单元（seat / boot_slot），续费 tab 与容量统计共用
interface LU {
  id: number
  kind: 'seat' | 'boot_slot'
  created_at: string
  expire_at: string
  instance: { cp_id: string, name: string, status: string } | null
}
let luSeq = 100
const today = new Date('2026-06-21T08:00:00+08:00')
const licenseUnits: LU[] = [
  { id: ++luSeq, kind: 'seat', created_at: iso(addDays(today, -120)), expire_at: iso(addDays(today, 8)), instance: { cp_id: 'cp-a1', name: '手机A', status: 'RUNNING' } },
  { id: ++luSeq, kind: 'seat', created_at: iso(addDays(today, -90)), expire_at: iso(addDays(today, 40)), instance: { cp_id: 'cp-a2', name: '手机B', status: 'STOPPED' } },
  { id: ++luSeq, kind: 'seat', created_at: iso(addDays(today, -60)), expire_at: iso(addDays(today, 3)), instance: { cp_id: 'cp-a3', name: '采集机03', status: 'RUNNING' } },
  { id: ++luSeq, kind: 'seat', created_at: iso(addDays(today, -30)), expire_at: iso(addDays(today, 200)), instance: null },
  { id: ++luSeq, kind: 'seat', created_at: iso(addDays(today, -10)), expire_at: iso(addDays(today, 350)), instance: null },
  { id: ++luSeq, kind: 'boot_slot', created_at: iso(addDays(today, -50)), expire_at: iso(addDays(today, 5)), instance: { cp_id: 'cp-a1', name: '手机A', status: 'RUNNING' } },
  { id: ++luSeq, kind: 'boot_slot', created_at: iso(addDays(today, -20)), expire_at: iso(addDays(today, 25)), instance: { cp_id: 'cp-a3', name: '采集机03', status: 'RUNNING' } },
  { id: ++luSeq, kind: 'boot_slot', created_at: iso(addDays(today, -5)), expire_at: iso(addDays(today, 60)), instance: null },
]

function capacity(kind: 'seat' | 'boot_slot') {
  return licenseUnits.filter(u => u.kind === kind).length
}
function inUse(kind: 'seat' | 'boot_slot') {
  return licenseUnits.filter(u => u.kind === kind && u.instance).length
}

// ---------- 定价配置 ----------
const purchaseConfig = {
  payment_methods: [
    { code: 'balance', name: '余额支付', enabled: true, sort: 0 },
    { code: 'wechat', name: '微信支付', enabled: true, sort: 1 },
    { code: 'alipay', name: '支付宝', enabled: true, sort: 2 },
  ],
  recharge_presets_cents: [1000, 5000, 10000, 50000, 100000],
  kinds: {
    seat: {
      unit_price_cents: 3000,
      unit_label: '台',
      qty_options: [1, 2, 5, 10, 50, 100, 500, 1000],
      qty_tiers: [
        { min_quantity: 10, discount_bps: 9000 },
        { min_quantity: 100, discount_bps: 8000 },
      ],
      duration_unit: 'month',
      duration_options: [
        { value: 1, discount_bps: 10000 },
        { value: 3, discount_bps: 8500 },
        { value: 12, discount_bps: 7000 },
      ],
      notice: '云手机实例席位决定你最多能拥有多少台实例。席位到期后若池中无可用余量，超额实例将进入回收站。',
      billing_note: '按「数量 × 月数」计价，享数量阶梯折扣 × 时长折扣（相乘）。',
    },
    boot_slot: {
      unit_price_cents: 2000,
      unit_label: '个',
      qty_options: [1, 2, 5, 10, 50, 100],
      qty_tiers: [
        { min_quantity: 10, discount_bps: 9000 },
        { min_quantity: 50, discount_bps: 8500 },
      ],
      duration_unit: 'day',
      duration_options: [
        { value: 7, discount_bps: 10000 },
        { value: 30, discount_bps: 9000 },
        { value: 90, discount_bps: 8000 },
      ],
      notice: '包月开机数决定可同时开机的实例数。开机时优先占用空闲包月名额，名额满后才扣临时开机时长。',
      billing_note: '按「数量 × 天数」计价，享数量阶梯折扣 × 时长折扣（相乘）。',
    },
  },
  runtime_pack: {
    unit_price_cents_per_minute: 20,
    min_minutes: 60,
    packs: [
      { minutes: 600, discount_bps: 10000 },
      { minutes: 3000, discount_bps: 9000 },
      { minutes: 9000, discount_bps: 8000 },
    ],
    notice: '临时开机时长无使用期限，用完为止。开机满 1 分钟才扣 1 分钟；每台手机每天最多扣 200 分钟，超过后当天继续运行不再扣。',
    daily_cap_minutes: 200,
  },
}

type Kind = 'seat' | 'boot_slot'
const KIND_BY_BIZ: Record<string, Kind> = {
  seat_new: 'seat',
  seat_renew: 'seat',
  boot_slot_new: 'boot_slot',
  boot_slot_renew: 'boot_slot',
}

// ---------- 计价（与契约 §1.3 一致：数量阶梯 × 时长折扣，相乘）----------
function pickQtyBps(kind: Kind, quantity: number): number {
  const tiers = purchaseConfig.kinds[kind].qty_tiers.filter(t => t.min_quantity <= quantity)
  if (!tiers.length) return 10000
  return tiers.reduce((best, t) => (t.min_quantity > best.min_quantity ? t : best)).discount_bps
}
function pickDurationBps(kind: Kind, durationValue: number): number {
  const opt = purchaseConfig.kinds[kind].duration_options.find(o => o.value === durationValue)
  return opt?.discount_bps ?? 10000
}
function pickRuntimeBps(minutes: number): number {
  const packs = purchaseConfig.runtime_pack.packs.filter(p => p.minutes <= minutes)
  if (!packs.length) return 10000
  return packs.reduce((best, p) => (p.minutes > best.minutes ? p : best)).discount_bps
}

interface Quote {
  quantity: number
  duration_value: number
  unit_price_cents: number
  billing_units: number
  original_cents: number
  qty_discount_bps: number
  duration_discount_bps: number
  payable_cents: number
}
function computeQuote(body: any): Quote | null {
  const bizType: string = body?.biz_type
  if (bizType === 'runtime_pack') {
    const minutes = Number(body?.minutes) || 0
    const unit = purchaseConfig.runtime_pack.unit_price_cents_per_minute
    const bps = pickRuntimeBps(minutes)
    const original = unit * minutes
    return {
      quantity: minutes,
      duration_value: 0,
      unit_price_cents: unit,
      billing_units: minutes,
      original_cents: original,
      qty_discount_bps: bps,
      duration_discount_bps: 10000,
      payable_cents: Math.round((original * bps) / 10000),
    }
  }
  if (bizType === 'recharge') {
    const cents = Number(body?.amount_cents) || 0
    return {
      quantity: 1,
      duration_value: 0,
      unit_price_cents: cents,
      billing_units: 1,
      original_cents: cents,
      qty_discount_bps: 10000,
      duration_discount_bps: 10000,
      payable_cents: cents,
    }
  }
  const kind = KIND_BY_BIZ[bizType]
  if (!kind) return null
  const quantity = Number(body?.quantity) || 1
  const durationValue = Number(body?.duration_value) || 1
  const unit = purchaseConfig.kinds[kind].unit_price_cents
  const qtyBps = pickQtyBps(kind, quantity)
  const durBps = pickDurationBps(kind, durationValue)
  const billingUnits = quantity * durationValue
  const original = unit * billingUnits
  const payable = Math.round((Math.round((original * qtyBps) / 10000) * durBps) / 10000)
  return {
    quantity,
    duration_value: durationValue,
    unit_price_cents: unit,
    billing_units: billingUnits,
    original_cents: original,
    qty_discount_bps: qtyBps,
    duration_discount_bps: durBps,
    payable_cents: payable,
  }
}

// ---------- 订单 ----------
interface OrderRec {
  order: {
    id: number
    biz_type: string
    status: string
    total_cents: number
    pay_method: string
    created_at: string
    paid_at: string | null
    expired_at: string | null
  }
  items: Array<{
    id: number
    order_id: number
    target_kind: string
    quantity: number
    duration_value: number
    duration_unit: string
    unit_price_cents: number
    qty_discount_bps: number
    duration_discount_bps: number
    amount_cents: number
  }>
}
let orderSeq = 9000
const orders: OrderRec[] = [
  {
    order: { id: ++orderSeq, biz_type: 'seat_new', status: 'paid', total_cents: 226800, pay_method: 'balance', created_at: iso(addDays(today, -30)), paid_at: iso(addDays(today, -30)), expired_at: null },
    items: [{ id: 1, order_id: orderSeq, target_kind: 'seat', quantity: 10, duration_value: 12, duration_unit: 'month', unit_price_cents: 3000, qty_discount_bps: 9000, duration_discount_bps: 7000, amount_cents: 226800 }],
  },
  {
    order: { id: ++orderSeq, biz_type: 'runtime_pack', status: 'paid', total_cents: 54000, pay_method: 'wechat', created_at: iso(addDays(today, -10)), paid_at: iso(addDays(today, -10)), expired_at: null },
    items: [{ id: 2, order_id: orderSeq, target_kind: 'runtime_minute', quantity: 3000, duration_value: 0, duration_unit: '', unit_price_cents: 20, qty_discount_bps: 9000, duration_discount_bps: 10000, amount_cents: 54000 }],
  },
  {
    order: { id: ++orderSeq, biz_type: 'boot_slot_new', status: 'unpaid', total_cents: 5400, pay_method: 'wechat', created_at: iso(addDays(today, -1)), paid_at: null, expired_at: iso(addDays(today, 1)) },
    items: [{ id: 3, order_id: orderSeq, target_kind: 'boot_slot', quantity: 3, duration_value: 30, duration_unit: 'day', unit_price_cents: 2000, qty_discount_bps: 10000, duration_discount_bps: 9000, amount_cents: 5400 }],
  },
]

// 履约：新购授权单元 / 续费 / 加临时时长（mock 简化版）
function fulfill(rec: OrderRec) {
  for (const it of rec.items) {
    if (it.target_kind === 'seat' || it.target_kind === 'boot_slot') {
      if (rec.order.biz_type.endsWith('_new')) {
        const unit = it.duration_unit === 'month' ? it.duration_value * 30 : it.duration_value
        for (let i = 0; i < it.quantity; i++) {
          licenseUnits.push({
            id: ++luSeq,
            kind: it.target_kind,
            created_at: now(),
            expire_at: iso(addDays(new Date(), unit)),
            instance: null,
          })
        }
      }
    }
    else if (it.target_kind === 'runtime_minute') {
      wallet.runtime_minutes_remaining += it.quantity
    }
  }
}

function settlePay(rec: OrderRec): { ok: boolean, msg?: string } {
  if (rec.order.status === 'paid') return { ok: false, msg: '订单已支付' }
  if (rec.order.pay_method === 'balance') {
    if (wallet.balance_cents < rec.order.total_cents) return { ok: false, msg: '余额不足' }
    wallet.balance_cents -= rec.order.total_cents
  }
  rec.order.status = 'paid'
  rec.order.paid_at = now()
  if (rec.order.biz_type === 'recharge') {
    wallet.balance_cents += rec.order.total_cents
  }
  else {
    fulfill(rec)
  }
  return { ok: true }
}

// ---------- 费用日志（聚合开机会话）----------
const runtimeLog = [
  {
    cp_id: 'cp-a1',
    instance_name: '手机A',
    power_on_at: iso(addDays(today, -1)),
    power_off_at: null,
    quota_type: 'temp',
    temp_minutes_charged: 45,
    running: true,
    segments: [
      { quota_type: 'boot_slot', minutes: 0, from: iso(addDays(today, -1)), to: iso(new Date(addDays(today, -1).getTime() + 90 * 60000)) },
      { quota_type: 'temp', minutes: 45, from: iso(new Date(addDays(today, -1).getTime() + 90 * 60000)), to: iso(today) },
    ],
  },
  {
    cp_id: 'cp-a3',
    instance_name: '采集机03',
    power_on_at: iso(addDays(today, -2)),
    power_off_at: iso(new Date(addDays(today, -2).getTime() + 30 * 60000)),
    quota_type: 'boot_slot',
    temp_minutes_charged: 0,
    running: false,
    segments: [
      { quota_type: 'boot_slot', minutes: 0, from: iso(addDays(today, -2)), to: iso(new Date(addDays(today, -2).getTime() + 30 * 60000)) },
    ],
  },
  {
    cp_id: 'cp-a2',
    instance_name: '手机B',
    power_on_at: iso(addDays(today, -3)),
    power_off_at: iso(new Date(addDays(today, -3).getTime() + 320 * 60000)),
    quota_type: 'mixed',
    temp_minutes_charged: 200,
    running: false,
    segments: [
      { quota_type: 'temp', minutes: 200, from: iso(addDays(today, -3)), to: iso(new Date(addDays(today, -3).getTime() + 200 * 60000)) },
      { quota_type: 'capped_free', minutes: 0, from: iso(new Date(addDays(today, -3).getTime() + 200 * 60000)), to: iso(new Date(addDays(today, -3).getTime() + 320 * 60000)) },
    ],
  },
]

// ---------- 旧端点保留所需状态 ----------
const account = { id: 1, user_id: 1, balance_cents: wallet.balance_cents, created_at: '2026-01-01 00:00:00', updated_at: now() }
const skus = [
  { sku: { id: 1, code: 'instance_fee', category: 'instance_fee', name: '云手机实例费', description: '按台月计费', unit_price_cents: 3000, unit: '台月', listed: true, sort: 1 }, tiers: [{ id: 1, sku_id: 1, cycle_months: 1, min_quantity: 1, discount_bps: 10000 }] },
]
const entitlementBatches = [
  { id: 1, user_id: 1, subject: 'instance_seat', quantity: 3, used: 1, expire_at: null, source: 'purchase', source_ref: 'ORD-0001', created_at: '2026-01-15 10:00:00' },
]
const trialPolicies = [
  { policy: { id: 1, code: 'newbie', name: '新用户试用', enabled: true, per_user_limit: 1, allow_new_user: true, invite_code: '', items: [{ id: 1, policy_id: 1, subject: 'seat', quantity: 1, expire_days: 7 }, { id: 2, policy_id: 1, subject: 'runtime_minute', quantity: 600, expire_days: 0 }] }, claimable: true, need_invite: false, claimed_count: 0, reason: '' },
]
let ledgerSeq = 1
const ledger: any[] = []

// ---------- 路由 ----------
export default defineFakeRoute([
  // ===== 新契约端点 =====
  {
    url: '/v1/billing/overview',
    method: 'get',
    response: () => ok({
      balance_cents: wallet.balance_cents,
      seat: { total: capacity('seat'), used: inUse('seat') },
      boot_slot: { total: capacity('boot_slot'), in_use: inUse('boot_slot') },
      runtime_minutes_remaining: wallet.runtime_minutes_remaining,
    }),
  },
  {
    url: '/v1/billing/purchase-config',
    method: 'get',
    response: () => ok(purchaseConfig),
  },
  {
    url: '/v1/billing/quote',
    method: 'post',
    response: ({ body }) => {
      const q = computeQuote(body)
      if (!q) return fail('参数有误')
      return ok(q)
    },
  },
  {
    url: '/v1/billing/license-units',
    method: 'get',
    response: ({ query }) => {
      const kind = query.kind as Kind
      let list = licenseUnits.filter(u => u.kind === kind)
      if (query.expiring_before)
        list = list.filter(u => new Date(u.expire_at) <= new Date(String(query.expiring_before)))
      if (query.keyword) {
        const kw = String(query.keyword)
        list = list.filter(u => u.instance && (u.instance.name.includes(kw) || u.instance.cp_id.includes(kw)))
      }
      return ok({ items: list, total: list.length })
    },
  },
  {
    url: '/v1/billing/orders',
    method: 'post',
    response: ({ body }) => {
      const bizType: string = body?.biz_type
      const payMethod: string = body?.pay_method || 'balance'
      const orderId = ++orderSeq
      let items: OrderRec['items'] = []
      let total = 0
      if (bizType === 'recharge') {
        total = Number(body?.amount_cents) || 0
        if (total <= 0) return fail('充值金额必须大于0')
      }
      else {
        const q = computeQuote(body)
        if (!q) return fail('参数有误')
        total = q.payable_cents
        const kind = bizType === 'runtime_pack' ? 'runtime_minute' : KIND_BY_BIZ[bizType]
        const durUnit = kind === 'seat' ? 'month' : kind === 'boot_slot' ? 'day' : ''
        items = [{
          id: 1,
          order_id: orderId,
          target_kind: kind,
          quantity: q.quantity,
          duration_value: q.duration_value,
          duration_unit: durUnit,
          unit_price_cents: q.unit_price_cents,
          qty_discount_bps: q.qty_discount_bps,
          duration_discount_bps: q.duration_discount_bps,
          amount_cents: q.payable_cents,
        }]
      }
      const rec: OrderRec = {
        order: { id: orderId, biz_type: bizType, status: 'unpaid', pay_method: payMethod, total_cents: total, created_at: now(), paid_at: null, expired_at: payMethod === 'balance' ? null : iso(addDays(new Date(), 1)) },
        items,
      }
      orders.unshift(rec)
      // 余额支付即时结算；第三方返回待支付（stub）。
      let pay: any = { status: 'unpaid', pay_url: 'https://pay.example.com/stub', qr_code: 'stub-qr' }
      if (payMethod === 'balance') {
        const r = settlePay(rec)
        if (!r.ok) {
          orders.shift()
          return fail(r.msg || '支付失败')
        }
        pay = { status: 'paid' }
      }
      return ok({ order: rec.order, pay })
    },
  },
  {
    url: '/v1/billing/orders',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 20
      let list = orders.map(o => o.order)
      if (query.status) list = list.filter(o => o.status === query.status)
      const total = list.length
      return ok({ items: list.slice((page - 1) * size, page * size), total })
    },
  },
  {
    url: '/v1/billing/orders/:id',
    method: 'get',
    response: ({ params }) => {
      const rec = orders.find(o => o.order.id === Number(params.id))
      if (!rec) return fail('订单不存在')
      return ok({ ...rec.order, items: rec.items })
    },
  },
  {
    url: '/v1/billing/orders/:id/pay',
    method: 'post',
    response: ({ params }) => {
      const rec = orders.find(o => o.order.id === Number(params.id))
      if (!rec) return fail('订单不存在')
      const r = settlePay(rec)
      if (!r.ok) return fail(r.msg || '支付失败')
      return ok({ order: rec.order, pay: { status: 'paid' } })
    },
  },
  {
    url: '/v1/billing/runtime/log',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 20
      const total = runtimeLog.length
      return ok({ items: runtimeLog.slice((page - 1) * size, page * size), total, daily_cap_minutes: purchaseConfig.runtime_pack.daily_cap_minutes })
    },
  },

  // ===== 旧端点（保留供旧视图：account/ledger/skus/entitlements/trials）=====
  {
    url: '/v1/billing/account',
    method: 'get',
    response: () => ok({ ...account, balance_cents: wallet.balance_cents, updated_at: now() }),
  },
  {
    url: '/v1/billing/topup',
    method: 'post',
    response: ({ body }) => {
      const amount = Number(body?.amount_cents) || 0
      if (amount <= 0) return fail('充值金额必须大于0')
      wallet.balance_cents += amount
      return ok({ ...account, balance_cents: wallet.balance_cents, updated_at: now() })
    },
  },
  {
    url: '/v1/billing/ledger',
    method: 'get',
    response: ({ query }) => {
      const page = Number(query.page) || 1
      const size = Number(query.size) || 20
      let list = [...ledger]
      if (query.subject) list = list.filter(e => e.subject === query.subject)
      if (query.type) list = list.filter(e => e.type === query.type)
      return ok({ list: list.slice((page - 1) * size, page * size), total: list.length })
    },
  },
  {
    url: '/v1/billing/skus',
    method: 'get',
    response: () => ok(skus),
  },
  {
    url: '/v1/billing/entitlements',
    method: 'get',
    response: () => ok({
      capacities: { instance_seat: capacity('seat'), boot_seat: capacity('boot_slot'), runtime_minute: wallet.runtime_minutes_remaining },
      batches: entitlementBatches,
    }),
  },
  {
    url: '/v1/billing/trials',
    method: 'get',
    response: () => ok(trialPolicies),
  },
  {
    url: '/v1/billing/trials/:code/claim',
    method: 'post',
    response: ({ params, body }) => {
      const item = trialPolicies.find(t => t.policy.code === params.code)
      if (!item) return fail('试用活动不存在')
      if (item.need_invite && !body?.invite_code) return fail('请填写邀请码')
      item.claimable = false
      item.claimed_count += 1
      // 静默用 ledgerSeq 防 lint 未用告警
      void ledgerSeq++
      return ok(null)
    },
  },
])
