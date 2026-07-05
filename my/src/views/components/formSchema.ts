import { z } from 'zod'

/** 翻译函数签名，测试时可传桩函数（返回 key 本身）。 */
export type Translator = (key: string) => string

const ROLES = ['admin', 'editor', 'viewer'] as const

/**
 * 组件库示例「个人资料表单」的校验 schema。
 * 抽成工厂函数以便：1) 校验文案走 i18n；2) 单测可注入桩 t。
 * 必填字段：name、email、role、agree（对应 UI 上的 * 标记）。
 */
export function buildProfileSchema(t: Translator) {
  return z.object({
    name: z
      .string()
      .min(1, t('valid.nameRequired'))
      .min(2, t('valid.nameMin')),
    email: z
      .string()
      .min(1, t('valid.emailRequired'))
      .email(t('valid.emailInvalid')),
    role: z
      .string()
      .refine((v): v is (typeof ROLES)[number] => (ROLES as readonly string[]).includes(v), {
        message: t('valid.roleRequired'),
      }),
    agree: z.boolean().refine(v => v === true, {
      message: t('valid.agreeRequired'),
    }),
    bio: z.string().max(200, t('valid.bioMax')).optional(),
    // 可选/有默认值，不做业务校验，仅纳入表单以便预览与提交。
    date: z.any().optional(),
    plan: z.string().optional(),
    notify: z.boolean().optional(),
  })
}
