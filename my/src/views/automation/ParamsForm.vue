<script setup lang="ts">
import type { ParamSpec } from '@/types/automation'
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Textarea } from '@/components/ui/textarea'

const props = withDefaults(defineProps<{
  schema: string
  // seedDefaults=false 用于「逐台覆盖」：不预填默认值，留空=不覆盖。
  seedDefaults?: boolean
}>(), { seedDefaults: true })

// v-model：参数值 { key: value }。array/object 存的是已解析的实际值。
const model = defineModel<Record<string, unknown>>({ default: () => ({}) })
const { t } = useI18n()

const specs = computed<ParamSpec[]>(() => {
  if (!props.schema.trim())
    return []
  try {
    const arr = JSON.parse(props.schema)
    return Array.isArray(arr) ? arr : []
  }
  catch {
    return []
  }
})

const rawText = reactive<Record<string, string>>({}) // array/object 的文本态
const errors = reactive<Record<string, string>>({}) // array/object 的解析错误

// schema 变化时初始化值与文本。
watch(specs, (list) => {
  const next: Record<string, unknown> = {}
  for (const s of list) {
    const cur = model.value[s.key]
    if (cur !== undefined) {
      next[s.key] = cur
    }
    else if (props.seedDefaults && s.default !== undefined) {
      next[s.key] = s.default
    }
    if (s.type === 'array' || s.type === 'object') {
      const v = next[s.key]
      rawText[s.key] = v === undefined ? '' : JSON.stringify(v, null, 2)
      errors[s.key] = ''
    }
  }
  model.value = next
}, { immediate: true })

function setVal(key: string, v: unknown) {
  model.value = { ...model.value, [key]: v }
}

function onNumber(key: string, e: Event) {
  const raw = (e.target as HTMLInputElement).value
  if (raw === '') {
    const { [key]: _omit, ...rest } = model.value
    model.value = rest
    return
  }
  const n = Number(raw)
  if (!Number.isNaN(n))
    setVal(key, n)
}

function onJson(key: string, text: string) {
  rawText[key] = text
  if (!text.trim()) {
    errors[key] = ''
    const { [key]: _omit, ...rest } = model.value
    model.value = rest
    return
  }
  try {
    setVal(key, JSON.parse(text))
    errors[key] = ''
  }
  catch {
    errors[key] = t('script.params.badJson')
  }
}

function labelOf(s: ParamSpec) {
  return s.label || s.key
}

// 暴露校验：返回错误信息（空=通过）。
const lastError = ref('')
function validate(): string {
  for (const s of specs.value) {
    if (s.type === 'array' || s.type === 'object') {
      if (errors[s.key]) {
        lastError.value = `${labelOf(s)}: ${errors[s.key]}`
        return lastError.value
      }
    }
    if (s.required) {
      const v = model.value[s.key]
      const empty = v === undefined || v === null || v === ''
      if (empty) {
        lastError.value = t('script.params.requiredMsg', { name: labelOf(s) })
        return lastError.value
      }
    }
  }
  lastError.value = ''
  return ''
}

defineExpose({ validate })
</script>

<template>
  <div v-if="specs.length" class="space-y-3">
    <div v-for="s in specs" :key="s.key" class="grid gap-1.5">
      <Label class="text-sm">
        {{ labelOf(s) }}
        <span v-if="s.required" class="text-destructive">*</span>
        <span class="ml-1 font-mono text-xs text-muted-foreground">{{ s.key }}</span>
      </Label>

      <!-- boolean -->
      <label v-if="s.type === 'boolean'" class="flex items-center gap-2 text-sm">
        <Checkbox
          :model-value="model[s.key] === true"
          @update:model-value="(v) => setVal(s.key, v === true)"
        />
        <span class="text-muted-foreground">{{ model[s.key] === true ? 'true' : 'false' }}</span>
      </label>

      <!-- enum -->
      <NativeSelect
        v-else-if="s.type === 'enum'"
        :model-value="(model[s.key] as string) ?? ''"
        @update:model-value="(v?: unknown) => setVal(s.key, v)"
      >
        <NativeSelectOption value="" disabled>
          {{ t('script.params.choose') }}
        </NativeSelectOption>
        <NativeSelectOption v-for="o in (s.options ?? [])" :key="o" :value="o">
          {{ o }}
        </NativeSelectOption>
      </NativeSelect>

      <!-- number -->
      <Input
        v-else-if="s.type === 'number'"
        type="number"
        :value="(model[s.key] as number) ?? ''"
        @input="(e: Event) => onNumber(s.key, e)"
      />

      <!-- array / object -->
      <template v-else-if="s.type === 'array' || s.type === 'object'">
        <Textarea
          :model-value="rawText[s.key] ?? ''"
          :rows="3"
          class="font-mono text-xs"
          :placeholder="s.type === 'array' ? '[]' : '{}'"
          @update:model-value="(v) => onJson(s.key, String(v))"
        />
        <p v-if="errors[s.key]" class="text-xs text-destructive">
          {{ errors[s.key] }}
        </p>
      </template>

      <!-- string -->
      <Input
        v-else
        :model-value="(model[s.key] as string) ?? ''"
        @update:model-value="(v) => setVal(s.key, v)"
      />

      <p v-if="s.description" class="text-xs text-muted-foreground">
        {{ s.description }}
      </p>
    </div>
  </div>
</template>
