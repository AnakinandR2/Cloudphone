import { describe, expect, it } from 'vitest'
import { md5Buffer } from './md5'

function bufOf(s: string): ArrayBuffer {
  return new TextEncoder().encode(s).buffer
}

describe('md5Buffer · RFC 1321 测试向量', () => {
  it('空串', () => {
    expect(md5Buffer(bufOf(''))).toBe('d41d8cd98f00b204e9800998ecf8427e')
  })
  it('"abc"', () => {
    expect(md5Buffer(bufOf('abc'))).toBe('900150983cd24fb0d6963f7d28e17f72')
  })
  it('"message digest"', () => {
    expect(md5Buffer(bufOf('message digest'))).toBe('f96b697d7cb7938d525a2f31aaf161d0')
  })
  it('a-z', () => {
    expect(md5Buffer(bufOf('abcdefghijklmnopqrstuvwxyz'))).toBe('c3fcd3d76192e4007dfb496cca67e13b')
  })
  it('长串跨多个 64 字节块', () => {
    const s = '12345678901234567890123456789012345678901234567890123456789012345678901234567890'
    expect(md5Buffer(bufOf(s))).toBe('57edf4a22be3c955ac49da2e2107b67a')
  })
})
