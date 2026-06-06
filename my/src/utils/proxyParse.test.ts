import { describe, expect, it } from 'vitest'
import { parseProxyLine, parseProxyLines } from './proxyParse'

describe('parseProxyLine', () => {
  it('host:port', () => {
    expect(parseProxyLine('1.2.3.4:1080')).toEqual({
      name: '1.2.3.4:1080', protocol: 'socks5', host: '1.2.3.4', port: 1080,
      username: '', password: '', region: '', remark: '',
    })
  })

  it('host:port:user:pass', () => {
    const r = parseProxyLine('1.2.3.4:1080:alice:s3cret')
    expect(r).toMatchObject({ host: '1.2.3.4', port: 1080, username: 'alice', password: 's3cret' })
  })

  it('socks5://user:pass@host:port', () => {
    const r = parseProxyLine('socks5://bob:pw@example.com:1081')
    expect(r).toMatchObject({ host: 'example.com', port: 1081, username: 'bob', password: 'pw', protocol: 'socks5' })
  })

  it('user:pass@host:port (no scheme)', () => {
    const r = parseProxyLine('u1:p1@10.0.0.1:7890')
    expect(r).toMatchObject({ host: '10.0.0.1', port: 7890, username: 'u1', password: 'p1' })
  })

  it('socks:// scheme also stripped', () => {
    expect(parseProxyLine('socks://9.9.9.9:1080')).toMatchObject({ host: '9.9.9.9', port: 1080 })
  })

  it('password containing colons (4th segment and beyond)', () => {
    const r = parseProxyLine('h:1080:user:p:a:ss')
    expect(r).toMatchObject({ username: 'user', password: 'p:a:ss' })
  })

  it('@-form takes precedence over colon-form credentials', () => {
    // 既有 @ 凭证又有冒号段时，以 @ 前的为准（不覆盖）
    const r = parseProxyLine('admin:secret@host.io:1080')
    expect(r).toMatchObject({ host: 'host.io', port: 1080, username: 'admin', password: 'secret' })
  })

  it('username-only credentials (no password)', () => {
    const r = parseProxyLine('justuser@1.1.1.1:1080')
    expect(r).toMatchObject({ username: 'justuser', password: '' })
  })

  it('trims surrounding whitespace', () => {
    expect(parseProxyLine('  1.2.3.4:1080  ')).toMatchObject({ host: '1.2.3.4', port: 1080 })
  })

  it.each([
    ['', 'empty'],
    ['   ', 'blank'],
    ['just-a-host', 'no port'],
    ['1.2.3.4:abc', 'non-numeric port'],
    ['1.2.3.4:0', 'port 0'],
    ['1.2.3.4:70000', 'port out of range'],
    [':1080', 'empty host'],
  ])('returns null for invalid input %j (%s)', (input) => {
    expect(parseProxyLine(input)).toBeNull()
  })
})

describe('parseProxyLines', () => {
  it('parses multiple lines and drops invalid ones', () => {
    const raw = [
      '1.1.1.1:1080',
      '',
      'garbage-line',
      'socks5://u:p@2.2.2.2:1081',
      '3.3.3.3:abc', // invalid port → dropped
    ].join('\n')
    const out = parseProxyLines(raw)
    expect(out).toHaveLength(2)
    expect(out.map(p => p.host)).toEqual(['1.1.1.1', '2.2.2.2'])
  })

  it('returns empty array for all-invalid input', () => {
    expect(parseProxyLines('\n\nnonsense\n')).toEqual([])
  })
})
