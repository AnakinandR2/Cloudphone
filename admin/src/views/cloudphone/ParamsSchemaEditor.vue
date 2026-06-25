<script setup lang="ts">
import type { ParamSpec, ParamType } from '@/types/automation'
import { Plus, Trash2 } from 'lucide-vue-next'
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Textarea } from '@/components/ui/textarea'

// v-model 是参数定义的 JSON 字符串（我们的数组格式）。
const model = defineModel<string>({ default: '' })
const { t } = useI18n()

interface Row {
  rid: number
  key: string
  type: ParamType
  required: boolean
  optionsText: string // 仅 enum：候选项（逗号分隔）
  // 「值」列=默认值：string/number/table → 文本/数字/Lua 文本；enum → 选中的默认；bool 用 valueBool
  valueText: string
  valueBool: boolean
  description: string
}

// 解析某行 enum 的候选项。
function optionsOf(r: Row): string[] {
  return String(r.optionsText ?? '').split(/[,\n]/).map(s => s.trim()).filter(Boolean)
}

const types: { value: ParamType, label: string }[] = [
  { value: 'string', label: 'string' },
  { value: 'number', label: 'number' },
  { value: 'boolean', label: 'bool' },
  { value: 'enum', label: 'enum' },
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
    optionsText: (s.options ?? []).join(', '),
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
  // 只在外部变化时 re-parse；自己 emit 的值（与 lastEmitted 相同）跳过，
  // 否则输入候选项时整表会被重建、popover/输入框消失。
  if ((v ?? '') === lastEmitted)
    return
  parse(v ?? '')
}, { immediate: true })

function rowToSpec(r: Row): ParamSpec {
  const spec: ParamSpec = { key: r.key.trim(), type: r.type }
  if (r.required)
    spec.required = true
  const valTxt = String(r.valueText ?? '').trim() // 数字输入可能给到 number，统一转字符串
  if (r.type === 'enum') {
    const opts = optionsOf(r)
    spec.options = opts
    if (valTxt && opts.includes(valTxt))
      spec.default = valTxt
  }
  else if (r.type === 'boolean') {
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
  rows.value.push({ rid: ++seq, key: '', type: 'string', required: false, optionsText: '', valueText: '', valueBool: false, description: '' })
}
function removeRow(rid: number) {
  rows.value = rows.value.filter(r => r.rid !== rid)
}
</script>

<template>
  <div class="space-y-2">
    <div v-if="!rows.length" class="rounded-md border border-dashed p-3 text-center text-xs text-muted-foreground">
      {{ t('scriptStore.params.empty') }}
    </div>

    <table v-else class="w-full border-separate border-spacing-x-2 border-spacing-y-1 text-sm">
      <thead>
        <tr class="text-left text-xs text-muted-foreground">
          <th class="font-medium">{{ t('scriptStore.params.key') }}</th>
          <th class="font-medium">{{ t('scriptStore.params.type') }}</th>
          <th class="font-medium">{{ t('scriptStore.params.value') }}</th>
          <th class="font-medium">{{ t('scriptStore.params.required') }}</th>
          <th class="font-medium">{{ t('scriptStore.params.desc') }}</th>
          <th />
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in rows" :key="r.rid" class="align-top">
          <td class="w-32">
            <Input v-model="r.key" class="h-8 font-mono" :placeholder="t('scriptStore.params.key')" />
          </td>
          <td>
            <div class="flex items-center gap-1">
              <NativeSelect v-model="r.type" class="h-8 w-24">
                <NativeSelectOption v-for="ty in types" :key="ty.value" :value="ty.value">
                  {{ ty.label }}
                </NativeSelectOption>
              </NativeSelect>
              <!-- enum：候选项用 popover 编辑 -->
              <Popover v-if="r.type === 'enum'">
                <PopoverTrigger as-child>
                  <Button variant="outline" size="sm" class="h-8 px-2 text-xs">
                    {{ t('scriptStore.params.optionsCol') }}<span class="ml-1 text-muted-foreground">({{ optionsOf(r).length }})</span>
                  </Button>
                </PopoverTrigger>
                <PopoverContent class="w-64" align="start">
                  <div class="grid gap-1.5">
                    <Label class="text-xs">{{ t('scriptStore.params.optionsCol') }}</Label>
                    <Textarea v-model="r.optionsText" :rows="3" class="text-xs" :placeholder="t('scriptStore.params.enumPh')" />
                  </div>
                </PopoverContent>
              </Popover>
            </div>
          </td>
          <td>
            <label v-if="r.type === 'boolean'" class="flex h-8 items-center gap-2">
              <Checkbox :model-value="r.valueBool" @update:model-value="(v) => (r.valueBool = v === true)" />
              <span class="text-muted-foreground">{{ r.valueBool ? 'true' : 'false' }}</span>
            </label>
            <Input v-else-if="r.type === 'number'" v-model="r.valueText" type="number" class="h-8" />
            <NativeSelect v-else-if="r.type === 'enum'" :model-value="r.valueText" class="h-8" @update:model-value="(v?: unknown) => (r.valueText = String(v ?? ''))">
              <NativeSelectOption value="">
                {{ t('scriptStore.params.choose') }}
              </NativeSelectOption>
              <NativeSelectOption v-for="o in optionsOf(r)" :key="o" :value="o">
                {{ o }}
              </NativeSelectOption>
            </NativeSelect>
            <Textarea
              v-else-if="r.type === 'table'"
              :model-value="r.valueText"
              :rows="2"
              class="font-mono text-xs"
              placeholder="{1, 2, 3}"
              @update:model-value="(v) => (r.valueText = String(v))"
            />
            <Input v-else v-model="r.valueText" class="h-8" />
          </td>
          <td class="w-12 text-center">
            <Checkbox :model-value="r.required" class="mt-1.5" @update:model-value="(v) => (r.required = v === true)" />
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
      <Plus class="size-4" /> {{ t('scriptStore.params.add') }}
    </Button>
  </div>
</template>
