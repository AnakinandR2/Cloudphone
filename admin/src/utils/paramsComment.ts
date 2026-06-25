import type { ParamSpec, ParamType } from '@/types/automation'

// 脚本参数 schema 的"真源"是脚本顶部的 --[[ ... ]] 注释（中台 object 格式 + 我们的扩展字段）。
// 本工具负责：从注释提取/解析 schema、由 schema 生成注释、把注释插入/替换进 luaContent。
// 替换机制是纯 ${} 文本替换（见 docs/external_params_example.lua）。

const COMMENT_RE = /^\s*--\[\[([\s\S]*?)\]\]/
const OUR_TYPES: ParamType[] = ['string', 'number', 'boolean', 'enum', 'table']

// 中台/通用类型词 → 我们的内部类型（旧 array/object 并入 table）。
function mapMidType(t: string): ParamType {
  switch ((t || '').toLowerCase().trim()) {
    case 'int': case 'integer': case 'number': case 'float': case 'double': case 'long': return 'number'
    case 'bool': case 'boolean': return 'boolean'
    case 'array': case 'table': case 'list': case 'object': case 'map': return 'table'
    case 'enum': return 'enum'
    default: return 'string'
  }
}

// 我们的类型 → 注释里写的中台词表（同时另存 uiType 以无损回环）。
function toMidType(t: ParamType): string {
  switch (t) {
    case 'number': return 'int'
    case 'boolean': return 'bool'
    case 'enum': return 'string'
    case 'table': return 'table'
    default: return 'string'
  }
}

// 提取脚本顶部 --[[ ]] 注释里的 JSON 文本（不是 JSON 则返回 null，避免误伤普通注释）。
export function extractSchemaComment(lua: string): string | null {
  const m = COMMENT_RE.exec(lua || '')
  if (!m)
    return null
  const inner = m[1].trim()
  if (!inner.startsWith('{') && !inner.startsWith('['))
    return null
  try {
    JSON.parse(inner)
    return inner
  }
  catch {
    return null
  }
}

function normalizeSpec(s: Record<string, unknown>): ParamSpec {
  const t = s.type as ParamType
  const type: ParamType = OUR_TYPES.includes(t) ? t : mapMidType(String(s.type ?? ''))
  const spec: ParamSpec = { key: String(s.key ?? ''), type }
  if (s.required)
    spec.required = true
  if (s.default !== undefined && s.default !== null)
    spec.default = s.default
  if (s.description)
    spec.description = String(s.description)
  if (type === 'enum' && Array.isArray(s.options))
    spec.options = (s.options as unknown[]).map(String)
  return spec
}

// 解析 schema JSON：数组=我们格式；对象=中台格式（+扩展，按声明顺序）。
export function parseSchema(jsonText: string): ParamSpec[] {
  const text = (jsonText || '').trim()
  if (!text)
    return []
  let data: unknown
  try {
    data = JSON.parse(text)
  }
  catch {
    return []
  }
  if (Array.isArray(data)) {
    return data.filter((x): x is Record<string, unknown> => !!x && !!(x as Record<string, unknown>).key).map(normalizeSpec)
  }
  if (data && typeof data === 'object') {
    const obj = data as Record<string, Record<string, unknown>>
    return Object.keys(obj).map((key) => {
      const e = obj[key] || {}
      const ui = e.uiType as ParamType
      const type = (ui && OUR_TYPES.includes(ui)) ? ui : mapMidType(String(e.type ?? ''))
      return normalizeSpec({
        key,
        type,
        required: !!e.required,
        default: e.default,
        description: e.description || e.desc || '',
        options: e.options,
      })
    })
  }
  return []
}

// 由 specs 生成中台 object 格式的注释块（带 uiType/options 扩展，便于无损回环）。
export function buildSchemaComment(specs: ParamSpec[]): string {
  const obj: Record<string, Record<string, unknown>> = {}
  for (const s of specs) {
    if (!s.key)
      continue
    const entry: Record<string, unknown> = {
      desc: s.description || s.key,
      type: toMidType(s.type),
      required: !!s.required,
      uiType: s.type,
    }
    if (s.default !== undefined && s.default !== null)
      entry.default = s.default
    if (s.description)
      entry.description = s.description
    if (s.type === 'enum' && s.options?.length)
      entry.options = s.options
    obj[s.key] = entry
  }
  return `--[[\n${JSON.stringify(obj, null, 2)}\n]]`
}

// 把 schema 注释插入/替换到 luaContent 顶部；specs 为空则移除已有 schema 注释。
export function upsertSchemaComment(lua: string, specs: ParamSpec[]): string {
  let body = lua || ''
  if (extractSchemaComment(body) !== null)
    body = body.replace(COMMENT_RE, '')
  body = body.replace(/^\s+/, '')
  if (!specs.length)
    return body
  return `${buildSchemaComment(specs)}\n${body}`
}
