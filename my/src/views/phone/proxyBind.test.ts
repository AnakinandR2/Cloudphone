import { describe, expect, it } from 'vitest'
import type { CloudPhone } from '@/types/phone'
import type { Proxy } from '@/types/proxy'
import { countBoundProxies, formatProxyAddress, formatProxyIp } from './proxyBind'

function phone(proxy_id: number, status = 'STOPPED'): Pick<CloudPhone, 'proxy_id' | 'status'> {
  return { proxy_id, status }
}

describe('countBoundProxies', () => {
  it('空列表返回空 Map', () => {
    expect(countBoundProxies([]).size).toBe(0)
  })

  it('跳过未绑定（proxy_id<=0）', () => {
    const m = countBoundProxies([phone(0), phone(-1), phone(5)])
    expect(m.get(5)).toBe(1)
    expect(m.has(0)).toBe(false)
    expect(m.size).toBe(1)
  })

  it('同一代理被多台绑定时累加', () => {
    const m = countBoundProxies([phone(7), phone(7), phone(7), phone(9)])
    expect(m.get(7)).toBe(3)
    expect(m.get(9)).toBe(1)
  })

  it('跳过回收态（RECYCLED）云手机', () => {
    const m = countBoundProxies([phone(7), phone(7, 'RECYCLED')])
    expect(m.get(7)).toBe(1)
  })
})

describe('formatProxyAddress', () => {
  it('拼出 protocol://host:port', () => {
    expect(formatProxyAddress({ protocol: 'socks5', host: '1.2.3.4', port: 1080 })).toBe('socks5://1.2.3.4:1080')
  })

  it('协议缺省按 socks5', () => {
    expect(formatProxyAddress({ protocol: '', host: 'h', port: 1 })).toBe('socks5://h:1')
  })
})

describe('formatProxyIp', () => {
  const base: Pick<Proxy, 'egress_ip' | 'country' | 'city'> = { egress_ip: '', country: '', city: '' }

  it('有出口 IP + 国家城市', () => {
    expect(formatProxyIp({ ...base, egress_ip: '1.2.3.4', country: 'US', city: 'Los Angeles' }))
      .toBe('1.2.3.4 · US Los Angeles')
  })

  it('仅有出口 IP', () => {
    expect(formatProxyIp({ ...base, egress_ip: '1.2.3.4' })).toBe('1.2.3.4')
  })

  it('无出口 IP 返回空串', () => {
    expect(formatProxyIp(base)).toBe('')
  })
})
