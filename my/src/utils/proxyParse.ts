import type { ProxyCreate } from '@/types/proxy'

/**
 * 解析一行代理文本为结构化条目，协议固定 socks5。支持的格式（可选在最前加「名称,」列）：
 *   - host:port
 *   - host:port:user:pass        （密码可含冒号，取第 4 段及之后）
 *   - socks5://user:pass@host:port（scheme 可选，socks/socks5 均可）
 *   - user:pass@host:port
 *   - 名称,<以上任意格式>        （第一个逗号前为名称；缺省时名称按规则取 host:port）
 * 解析失败（空行 / 缺 host / 端口非法）返回 null。
 * 注意：不支持带方括号的 IPv6（SOCKS5 代理一般为 IPv4 或域名）。
 */
export function parseProxyLine(line: string): ProxyCreate | null {
  let s = line.trim()
  if (!s)
    return null
  // 可选名称列：`名称,<代理地址>`——第一个逗号前为自定义名称，缺省则回落 host:port。
  let customName = ''
  const comma = s.indexOf(',')
  if (comma >= 0) {
    customName = s.slice(0, comma).trim()
    s = s.slice(comma + 1).trim()
  }
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
    // 名称导入规则：优先自定义名称，缺省回落 host:port（后端亦按同规则兜底并做唯一性校验）。
    name: customName || `${host}:${port}`,
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

/** 逐行解析结果：有效条目 + 无法识别的非空行行号（1-based），供导入前向用户反馈。 */
export interface ProxyImportResult {
  items: ProxyCreate[]
  invalidRows: number[]
}

/**
 * 解析多行导入文本，保留「哪些行无法识别」的行号（空行不计入错误）。
 * 供批量导入弹窗提示用户，避免非法行被静默丢弃。
 */
export function parseProxyImport(raw: string): ProxyImportResult {
  const items: ProxyCreate[] = []
  const invalidRows: number[] = []
  raw.split('\n').forEach((line, i) => {
    if (!line.trim())
      return // 空行跳过，不算错误
    const p = parseProxyLine(line)
    if (p)
      items.push(p)
    else
      invalidRows.push(i + 1)
  })
  return { items, invalidRows }
}
