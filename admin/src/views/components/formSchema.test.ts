import { describe, expect, it } from 'vitest'
import { buildProfileSchema } from './formSchema'

// 桩翻译：返回 key 本身，便于断言命中了哪条文案。
const t = (k: string) => k
const schema = buildProfileSchema(t)

const validInput = {
  name: '张三',
  email: 'zhang@example.com',
  role: 'editor',
  agree: true,
  bio: '你好',
  plan: 'pro',
  notify: true,
}

/** 取某字段的第一条错误信息。 */
function errorFor(input: unknown, field: string) {
  const r = schema.safeParse(input)
  if (r.success)
    return undefined
  return r.error.issues.find(i => i.path[0] === field)?.message
}

describe('profileSchema', () => {
  it('合法数据通过', () => {
    expect(schema.safeParse(validInput).success).toBe(true)
  })

  it('date/bio 可省略仍通过', () => {
    const { bio, ...rest } = validInput
    expect(schema.safeParse(rest).success).toBe(true)
  })

  it('name 为空报必填', () => {
    expect(errorFor({ ...validInput, name: '' }, 'name')).toBe('valid.nameRequired')
  })

  it('name 少于 2 字报最小长度', () => {
    expect(errorFor({ ...validInput, name: '甲' }, 'name')).toBe('valid.nameMin')
  })

  it('email 为空报必填', () => {
    expect(errorFor({ ...validInput, email: '' }, 'email')).toBe('valid.emailRequired')
  })

  it('email 格式错误报无效', () => {
    expect(errorFor({ ...validInput, email: 'not-an-email' }, 'email')).toBe('valid.emailInvalid')
  })

  it('role 未选（空/非法）报必选', () => {
    expect(errorFor({ ...validInput, role: '' }, 'role')).toBe('valid.roleRequired')
    expect(errorFor({ ...validInput, role: 'guest' }, 'role')).toBe('valid.roleRequired')
  })

  it('agree 未勾选报必勾', () => {
    expect(errorFor({ ...validInput, agree: false }, 'agree')).toBe('valid.agreeRequired')
  })

  it('bio 超 200 字报超长', () => {
    expect(errorFor({ ...validInput, bio: 'a'.repeat(201) }, 'bio')).toBe('valid.bioMax')
  })
})
