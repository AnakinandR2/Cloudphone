<script setup lang="ts">
import type { ParamSpec, ParamType } from '@/types/automation'
import { Plus, Trash2 } from 'lucide-vue-next'
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

// v-model 是参数定义的 JSON 字符串（脚本 paramsSchema）。
const model = defineModel<string>({ default: '' })
const { t } = useI18n()

interface Row {
  rid: number
  key: string
  label: string
  type: ParamType
  required: boolean
  defaultText: string
  optionsText: string
  description: string
}

const types: ParamType[] = ['string', 'number', 'boolean', 'enum', 'array', 'object']
let seq = 0
const rows = ref<Row[]>([])
let syncing = false

function parse(raw: string) {
  rows.value = []
  if (!raw.trim())
    return
  try {
    const specs = JSON.parse(raw) as ParamSpec[]
    if (!Array.isArray(specs))
      return
    rows.value = specs.map(s => ({
      rid: ++seq,
      key: s.key ?? '',
      label: s.label ?? '',
      type: s.type ?? 'string',
      required: !!s.required,
      defaultText: s.default === undefined || s.default === null
        ? ''
        : (s.type === 'array' || s.type === 'object' ? JSON.stringify(s.default) : String(s.default)),
      optionsText: (s.options ?? []).join(', '),
      description: s.description ?? '',
    }))
  }
  catch {
    rows.value = []
  }
}

watch(model, (v) => {
  if (syncing)
    return
  parse(v ?? '')
}, { immediate: true })

function coerceDefault(r: Row): unknown {
  const txt = r.defaultText.trim()
  if (!txt)
    return undefined
  switch (r.type) {
    case 'number': {
      const n = Number(txt)
      return Number.isNaN(n) ? undefined : n
    }
    case 'boolean':
      return txt === 'true'
    case 'array':
    case 'object':
      try {
        return JSON.parse(txt)
      }
      catch {
        return undefined
      }
    default:
      return txt
  }
}

function parseOptions(r: Row): string[] {
  return r.optionsText.split(/[,\n]/).map(s => s.trim()).filter(Boolean)
}

function emitSchema() {
  const specs: ParamSpec[] = rows.value.map((r) => {
    const spec: ParamSpec = { key: r.key.trim(), type: r.type }
    if (r.label.trim())
      spec.label = r.label.trim()
    if (r.required)
      spec.required = true
    if (r.type === 'enum')
      spec.options = parseOptions(r)
    const def = coerceDefault(r)
    if (def !== undefined)
      spec.default = def
    if (r.description.trim())
      spec.description = r.description.trim()
    return spec
  })
  syncing = true
  model.value = specs.length ? JSON.stringify(specs) : ''
  syncing = false
}

watch(rows, emitSchema, { deep: true })

function addRow() {
  rows.value.push({ rid: ++seq, key: '', label: '', type: 'string', required: false, defaultText: '', optionsText: '', description: '' })
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

    <div v-for="r in rows" :key="r.rid" class="space-y-2 rounded-md border p-3">
      <div class="flex items-center gap-2">
        <Input v-model="r.key" class="h-8 flex-1 font-mono" :placeholder="t('scriptStore.params.key')" />
        <NativeSelect v-model="r.type" class="h-8 w-28">
          <NativeSelectOption v-for="ty in types" :key="ty" :value="ty">
            {{ ty }}
          </NativeSelectOption>
        </NativeSelect>
        <label class="flex items-center gap-1 text-xs text-muted-foreground whitespace-nowrap">
          <Checkbox :model-value="r.required" @update:model-value="(v) => (r.required = v === true)" />
          {{ t('scriptStore.params.required') }}
        </label>
        <Button variant="ghost" size="icon" class="size-8 text-destructive" @click="removeRow(r.rid)">
          <Trash2 class="size-4" />
        </Button>
      </div>
      <div class="grid grid-cols-2 gap-2">
        <Input v-model="r.label" class="h-8" :placeholder="t('scriptStore.params.label')" />
        <Input v-model="r.defaultText" class="h-8" :placeholder="t('scriptStore.params.default')" />
      </div>
      <Input v-if="r.type === 'enum'" v-model="r.optionsText" class="h-8" :placeholder="t('scriptStore.params.options')" />
      <Input v-model="r.description" class="h-8" :placeholder="t('scriptStore.params.desc')" />
    </div>

    <Button variant="outline" size="sm" class="w-full" @click="addRow">
      <Plus class="size-4" /> {{ t('scriptStore.params.add') }}
    </Button>
  </div>
</template>
