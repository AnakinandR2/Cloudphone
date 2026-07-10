import type { BadgeVariants } from '@/components/ui/badge'

/** 状态徽章统一基底：浅色底 + 彩色字（outline 系列复用） */
export const STATUS_BADGE_BASE = 'border-transparent px-2.5 font-semibold shadow-none'
export const STATUS_BADGE_DOT = 'size-1.5 rounded-full bg-current opacity-70'

type BadgeVariant = NonNullable<BadgeVariants['variant']>

export interface StatusBadgeProps {
  variant: BadgeVariant
  class: string
}

function outline(cls: string): StatusBadgeProps {
  return { variant: 'outline', class: `${STATUS_BADGE_BASE} ${cls}` }
}

/** 数据展示页成员列表同款：半透明底 + 彩色字 + 圆点（见 ComponentsDataView statusStyle） */
function dotBadge(cls: string): StatusBadgeProps {
  return { variant: 'default', class: `gap-1 border-transparent ${cls}` }
}

const emerald = 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-300'
const slate = 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300'
const slateMuted = 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400'
const red = 'bg-red-50 text-red-700 dark:bg-red-950/60 dark:text-red-300'
const amber = 'bg-amber-50 text-amber-700 dark:bg-amber-950/60 dark:text-amber-300'

const dotBlue = 'bg-blue-500/15 text-blue-600 dark:text-blue-400'
const dotEmerald = 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400'
const dotSlate = 'bg-slate-500/15 text-slate-600 dark:text-slate-400'
const dotRed = 'bg-red-500/15 text-red-600 dark:text-red-400'
const dotAmber = 'bg-amber-500/15 text-amber-600 dark:text-amber-400'

/** 云手机实例状态（RUNNING / STOPPED / CREATING …） */
export function phoneStatusBadge(status: string): StatusBadgeProps {
  if (status === 'RUNNING')
    return dotBadge(dotBlue)
  if (status === 'CREATED')
    return dotBadge(dotEmerald)
  if (status === 'STOPPED')
    return dotBadge(dotSlate)
  if (status === 'CREATE_FAILED' || status === 'ERROR')
    return dotBadge(dotRed)
  if (status === 'CREATING' || status === 'STARTING' || status === 'STOPPING')
    return dotBadge(`${dotAmber} animate-pulse`)
  if (status === 'DESTROYING')
    return dotBadge(`${dotRed} animate-pulse`)
  if (status === 'UNKNOWN')
    return dotBadge(`${dotSlate} animate-pulse`)
  return dotBadge(dotSlate)
}

/** 代理连通性状态（ok / fail / unknown） */
export function proxyStatusBadge(status: string): StatusBadgeProps {
  if (status === 'ok')
    return outline(emerald)
  if (status === 'fail')
    return outline(red)
  return outline(slate)
}

/** 订单支付状态（paid / unpaid / expired） */
export function orderStatusBadge(status: string): StatusBadgeProps {
  if (status === 'paid')
    return outline(emerald)
  if (status === 'unpaid')
    return outline(amber)
  if (status === 'expired')
    return outline(slateMuted)
  return outline(slate)
}

/** 应用解析状态（parsing / failed / ready） */
export function parseStatusBadge(parseStatus: string): StatusBadgeProps {
  if (parseStatus === 'parsing')
    return outline(`${amber} animate-pulse`)
  if (parseStatus === 'failed')
    return outline(red)
  return outline(emerald)
}

/** 脚本/功能启用态 */
export function enabledStatusBadge(enabled: boolean): StatusBadgeProps {
  return enabled ? outline(emerald) : outline(slate)
}

/** 定时任务计划状态 */
export function planStatusBadge(status: string): StatusBadgeProps {
  if (status === 'ENABLING')
    return outline(emerald)
  return outline(slate)
}

/** 任务日志聚合态 */
export function taskLogStatusBadge(kind: 'success' | 'failed' | 'running'): StatusBadgeProps {
  if (kind === 'success')
    return outline(emerald)
  if (kind === 'failed')
    return outline(red)
  return outline(amber)
}

/** 任务详情弹框状态 */
export function taskReportStatusBadge(status: string | undefined): StatusBadgeProps {
  if (status === 'COMPLETED')
    return outline(emerald)
  if (status === 'FAILED' || status === 'CANCELLED')
    return outline(red)
  return outline(amber)
}

/** API Key 活跃/吊销 */
export function activeStatusBadge(active: boolean): StatusBadgeProps {
  return active ? outline(emerald) : outline(slateMuted)
}

/** 云手机运行日志会话状态 */
export function runSessionStatusBadge(sessionStatus: string): StatusBadgeProps {
  if (sessionStatus === '运行中')
    return outline(emerald)
  return outline(slateMuted)
}

/** 代理探测结果 */
export function probeStatusBadge(ok: boolean): StatusBadgeProps {
  return ok ? outline(emerald) : outline(red)
}
