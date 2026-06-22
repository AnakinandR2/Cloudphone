import type { CloudPhone } from '@/types/phone'
import type { Proxy } from '@/types/proxy'

/**
 * 统计每个代理被多少台云手机绑定（仅本人云手机列表）。
 * 跳过未绑定（proxy_id<=0）与回收态（RECYCLED，等同已弃用）。
 * 返回 Map<代理ID, 已绑定台数>。
 */
export function countBoundProxies(
  phones: Pick<CloudPhone, 'proxy_id' | 'status'>[],
): Map<number, number> {
  const counts = new Map<number, number>()
  for (const p of phones) {
    if (!p.proxy_id || p.proxy_id <= 0)
      continue
    if (p.status === 'RECYCLED')
      continue
    counts.set(p.proxy_id, (counts.get(p.proxy_id) ?? 0) + 1)
  }
  return counts
}

/** 代理地址展示：protocol://host:port（协议缺省按 socks5）。 */
export function formatProxyAddress(
  p: Pick<Proxy, 'protocol' | 'host' | 'port'>,
): string {
  return `${p.protocol || 'socks5'}://${p.host}:${p.port}`
}

/**
 * 代理出口 IP 信息：出口 IP + 国家/城市（如 "1.2.3.4 · US Los Angeles"）。
 * 未测试（无出口 IP）返回空串，由调用方显示占位（如「未测试」）。
 */
export function formatProxyIp(
  p: Pick<Proxy, 'egress_ip' | 'country' | 'city'>,
): string {
  if (!p.egress_ip)
    return ''
  const loc = [p.country, p.city].filter(Boolean).join(' ')
  return loc ? `${p.egress_ip} · ${loc}` : p.egress_ip
}
