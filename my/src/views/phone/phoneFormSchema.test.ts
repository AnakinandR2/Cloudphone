import { describe, expect, it } from 'vitest'
import { buildPhoneFormSchema } from './phoneFormSchema'

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
})
