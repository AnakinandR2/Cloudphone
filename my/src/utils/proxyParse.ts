import type { ProxyCreate } from '@/types/proxy'

/**
 * 解析一行代理文本为结构化条目，协议固定 socks5。支持的格式：
 *   - host:port
 *   - host:port:user:pass        （密码可含冒号，取第 4 段及之后）
 *   - socks5://user:pass@host:port（scheme 可选，socks/socks5 均可）
 *   - user:pass@host:port
 * 解析失败（空行 / 缺 host / 端口非法）返回 null。
 * 注意：不支持带方括号的 IPv6（SOCKS5 代理一般为 IPv4 或域名）。
 */
export function parseProxyLine(line: string): ProxyCreate | null {
  let s = line.trim()
  if (!s)
    return null
  // 去掉协议前缀（socks:// 或 socks5://）
  s = s.replace(/^socks5?:\/\//i, '')

  let username = ''
  let password = ''
  // user:pass@host:port —— 凭证在 @ 前
  const at = s.lastIndexOf('@')
  if (at >= 0) {
    const cred = s.slice(0, at)
    s = s.slice(at + 1)
    const ci = cred.indexOf(':')
    if (ci >= 0) {
      username = cred.slice(0, ci)
      password = cred.slice(ci + 1)
    }
    else {
      username = cred
    }
  }

  const parts = s.split(':')
  if (parts.length < 2)
    return null
  const host = parts[0].trim()
  const port = Number(parts[1])
  if (!host || !port || !Number.isInteger(port) || port < 1 || port > 65535)
    return null

  // host:port:user:pass —— 凭证在冒号分段里（仅当 @ 形式未提供时采用）
  if (parts.length >= 4 && !username) {
    username = parts[2]
    password = parts.slice(3).join(':')
  }

  return {
    name: `${host}:${port}`,
    protocol: 'socks5',
    host,
    port,
    username,
    password,
    region: '',
    remark: '',
  }
}

/** 解析多行文本，过滤掉无法解析的行。 */
export function parseProxyLines(raw: string): ProxyCreate[] {
  return raw
    .split('\n')
    .map(parseProxyLine)
    .filter((x): x is ProxyCreate => x !== null)
}
