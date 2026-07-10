import { describe, expect, it } from 'vitest'
import { buildPhoneFormSchema, PHONE_TEXT_MAX } from './phoneFormSchema'

const t = (k: string) => k
const schema = buildPhoneFormSchema(t)

describe('phoneFormSchema', () => {
  it('name 非空通过', () => {
    expect(schema.safeParse({ name: '我的手机' }).success).toBe(true)
  })

  it('name 为空或纯空格报必填', () => {
    const r = schema.safeParse({ name: '   ' })
    expect(r.success).toBe(false)
    if (!r.success)
      expect(r.error.issues[0]?.message).toBe('phone.errNameRequired')
  })

  it('name 超过上限报错', () => {
    const r = schema.safeParse({ name: 'x'.repeat(PHONE_TEXT_MAX + 1) })
    expect(r.success).toBe(false)
    if (!r.success)
      expect(r.error.issues[0]?.message).toBe('phone.errNameMax')
  })

  it('remark 超过上限报错', () => {
    const r = schema.safeParse({ name: 'ok', remark: 'y'.repeat(PHONE_TEXT_MAX + 1) })
    expect(r.success).toBe(false)
    if (!r.success)
      expect(r.error.issues.some(i => i.message === 'phone.errRemarkMax')).toBe(true)
  })
})
