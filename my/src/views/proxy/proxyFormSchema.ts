import { z } from 'zod'

/** 翻译函数签名，测试时可传桩函数（返回 key 本身）。 */
export type Translator = (key: string) => string

/** 端口取值上限（TCP）。 */
const PORT_MAX = 65535

/**
 * 代理「新增/编辑」表单校验 schema。
 * 必填字段：name、host、port（对应 UI 上的 * 标记）。
 * 抽成工厂函数：1) 文案走 i18n；2) 单测可注入桩 t。
 */
export function buildProxyFormSchema(t: Translator) {
  return z.object({
    name: z.string().trim().min(1, t('proxy.errNameRequired')),
    host: z.string().trim().min(1, t('proxy.errHostRequired')),
    // port 在表单里是 '' | number | string；空/非数字→必填，越界→范围错误。
    port: z.preprocess(
      (v) => {
        if (v === '' || v == null)
          return undefined
        const n = Number(v)
        return Number.isNaN(n) ? undefined : n
      },
      z
        .number({
          required_error: t('proxy.errPortRequired'),
          invalid_type_error: t('proxy.errPortRequired'),
        })
        .int(t('proxy.errPortRange'))
        .min(1, t('proxy.errPortRequired'))
        .max(PORT_MAX, t('proxy.errPortRange')),
    ),
  })
}
