import { z } from 'zod'

/** 翻译函数签名，测试时可传桩函数（返回 key 本身）。 */
export type Translator = (key: string) => string

/** 名称 / 标签 / 备注统一上限（按 Unicode 码点计） */
export const PHONE_TEXT_MAX = 50

/**
 * 云手机「新增/编辑」表单校验 schema。
 * 必填字段：name（对应 UI 上的 * 标记）。remark 及代理绑定为可选。
 */
export function buildPhoneFormSchema(t: Translator) {
  return z.object({
    name: z.string().trim().min(1, t('phone.errNameRequired')).max(PHONE_TEXT_MAX, t('phone.errNameMax')),
    remark: z.string().max(PHONE_TEXT_MAX, t('phone.errRemarkMax')).optional(),
  })
}
