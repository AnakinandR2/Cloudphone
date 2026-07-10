<script setup lang="ts">
import type { ParamSpec, ParamType } from '@/types/automation'
import { Plus, Trash2 } from 'lucide-vue-next'
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

// v-model 是参数定义的 JSON 字符串（我们的数组格式）。
const model = defineModel<string>({ default: '' })
const { t } = useI18n()

interface Row {
  rid: number
  key: string
  type: ParamType
  required: boolean
  // 「值」列=默认值：string/number/table → 文本/数字/Lua 文本；bool 用 valueBool
  valueText: string
  valueBool: boolean
  description: string
}

// 编辑器只暴露这四种直观类型（int/array 等中台词只在下发 scriptParams 时映射，用户不可见）。
const types: { value: ParamType, label: string }[] = [
  { value: 'string', label: 'string' },
  { value: 'number', label: 'number' },
  { value: 'boolean', label: 'bool' },
  { value: 'table', label: 'table' },
]

let seq = 0
const rows = ref<Row[]>([])
let lastEmitted = '' // 自己发出去的最近一次 model 值，用于跳过回环 re-parse

function specToRow(s: ParamSpec): Row {
  const row: Row = {
    rid: ++seq,
    key: s.key ?? '',
    type: s.type ?? 'string',
    required: !!s.required,
    valueText: '',
    valueBool: false,
    description: s.description ?? '',
  }
  if (s.type === 'boolean')
    row.valueBool = s.default === true
  else if (s.default !== undefined && s.default !== null)
    row.valueText = String(s.default)
  return row
}

function parse(raw: string) {
  rows.value = []
  if (!raw.trim())
    return
  try {
    const specs = JSON.parse(raw) as ParamSpec[]
    if (Array.isArray(specs))
      rows.value = specs.map(specToRow)
  }
  catch {
    rows.value = []
  }
}

watch(model, (v) => {
  // 只在外部变化时 re-parse；自己 emit 的值（与 lastEmitted 相同）跳过，否则输入时整表会被重建。
  if ((v ?? '') === lastEmitted)
    return
  parse(v ?? '')
}, { immediate: true })

function rowToSpec(r: Row): ParamSpec {
  const spec: ParamSpec = { key: r.key.trim(), type: r.type }
  // bool 不支持「必填」（复选框无未填态），故只对非 bool 写 required。
  if (r.required && r.type !== 'boolean')
    spec.required = true
  const valTxt = String(r.valueText ?? '').trim() // 数字输入可能给到 number，统一转字符串
  if (r.type === 'boolean') {
    spec.default = r.valueBool
  }
  else if (valTxt) {
    if (r.type === 'number') {
      const n = Number(valTxt)
      if (!Number.isNaN(n))
        spec.default = n
    }
    else {
      spec.default = valTxt // string / table（Lua 文本）
    }
  }
  if (r.description.trim())
    spec.description = r.description.trim()
  return spec
}

function emitSchema() {
  const specs = rows.value.map(rowToSpec)
  lastEmitted = specs.length ? JSON.stringify(specs) : ''
  model.value = lastEmitted
}

watch(rows, emitSchema, { deep: true })

function addRow() {
  rows.value.push({ rid: ++seq, key: '', type: 'string', required: false, valueText: '', valueBool: false, description: '' })
}
function removeRow(rid: number) {
  rows.value = rows.value.filter(r => r.rid !== rid)
}
</script>

<template>
  <div class="space-y-2">
    <div v-if="!rows.length" class="rounded-md border border-dashed p-3 text-center text-xs text-muted-foreground">
      {{ t('script.params.empty') }}
    </div>

    <table v-else class="w-full border-separate border-spacing-x-2 border-spacing-y-1 text-sm">
      <thead>
        <tr class="text-left text-xs text-muted-foreground">
          <th class="font-medium">{{ t('script.params.key') }}</th>
          <th class="font-medium">{{ t('script.params.type') }}</th>
          <th class="font-medium">{{ t('script.params.value') }}</th>
          <th class="font-medium">{{ t('script.params.required') }}</th>
          <th class="font-medium">{{ t('script.params.desc') }}</th>
          <th />
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in rows" :key="r.rid" class="align-top">
          <td class="w-32">
            <Input v-model="r.key" class="h-8 font-mono" :placeholder="t('script.params.key')" />
          </td>
          <td>
            <Select v-model="r.type">
              <SelectTrigger class="h-8 w-28 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="ty in types" :key="ty.value" :value="ty.value">
                  {{ ty.label }}
                </SelectItem>
              </SelectContent>
            </Select>
          </td>
          <td>
            <label v-if="r.type === 'boolean'" class="flex h-8 items-center gap-2">
              <Checkbox :model-value="r.valueBool" @update:model-value="(v) => (r.valueBool = v === true)" />
              <span class="text-muted-foreground">{{ r.valueBool ? 'true' : 'false' }}</span>
            </label>
            <Input v-else-if="r.type === 'number'" v-model="r.valueText" type="number" class="h-8" />
            <Textarea
              v-else-if="r.type === 'table'"
              :model-value="r.valueText"
              :rows="2"
              class="font-mono text-xs"
              placeholder="{1, 2} 或 {[1]=2, [26]=5}"
              @update:model-value="(v) => (r.valueText = String(v))"
            />
            <Input v-else v-model="r.valueText" class="h-8" />
          </td>
          <td class="w-12 text-center">
            <!-- bool 无「必填」概念：复选框总有 true/false 值，故不渲染。 -->
            <Checkbox v-if="r.type !== 'boolean'" :model-value="r.required" class="mt-1.5" @update:model-value="(v) => (r.required = v === true)" />
            <span v-else class="text-muted-foreground">—</span>
          </td>
          <td>
            <Input v-model="r.description" class="h-8" />
          </td>
          <td class="w-8">
            <Button variant="ghost" size="icon" class="size-8 text-destructive" @click="removeRow(r.rid)">
              <Trash2 class="size-4" />
            </Button>
          </td>
        </tr>
      </tbody>
    </table>

    <Button variant="outline" size="sm" class="w-full" @click="addRow">
      <Plus class="size-4" /> {{ t('script.params.add') }}
    </Button>
  </div>
</template>
