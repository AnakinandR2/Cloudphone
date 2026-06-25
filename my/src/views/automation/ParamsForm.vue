<script setup lang="ts">
import type { ParamSpec } from '@/types/automation'
import { computed, ref, watch } from 'vue'
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

// v-model：参数值 { key: value }。table 存的是 Lua table 字面量文本（string）。
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

// schema 变化时按默认值初始化（enum 无默认）。
watch(specs, (list) => {
  const next: Record<string, unknown> = {}
  for (const s of list) {
    const cur = model.value[s.key]
    if (cur !== undefined)
      next[s.key] = cur
    else if (props.seedDefaults && s.type !== 'enum' && s.default !== undefined)
      next[s.key] = s.default
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

// 暴露校验：返回错误信息（空=通过）。
const lastError = ref('')
function validate(): string {
  for (const s of specs.value) {
    if (s.required) {
      const v = model.value[s.key]
      const empty = v === undefined || v === null || v === ''
      if (empty) {
        lastError.value = t('script.params.requiredMsg', { name: s.key })
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
        <span class="font-mono">{{ s.key }}</span>
        <span v-if="s.required" class="ml-0.5 text-destructive">*</span>
        <span class="ml-1 text-xs text-muted-foreground">{{ s.type }}</span>
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

      <!-- table：直接写 Lua table 字面量文本 -->
      <Textarea
        v-else-if="s.type === 'table'"
        :model-value="(model[s.key] as string) ?? ''"
        :rows="2"
        class="font-mono text-xs"
        placeholder="{1, 2, 3}"
        @update:model-value="(v) => setVal(s.key, String(v))"
      />

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
