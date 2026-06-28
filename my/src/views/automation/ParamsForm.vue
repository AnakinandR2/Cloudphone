<script setup lang="ts">
import type { ParamSpec } from '@/types/automation'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
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

// schema 变化时按默认值初始化。
watch(specs, (list) => {
  const next: Record<string, unknown> = {}
  for (const s of list) {
    const cur = model.value[s.key]
    if (cur !== undefined)
      next[s.key] = cur
    else if (props.seedDefaults && s.default !== undefined)
      next[s.key] = s.default
    // bool 始终具体：无默认时 seed false（复选框显示与提交一致；规避必填误判）。
    else if (props.seedDefaults && s.type === 'boolean')
      next[s.key] = false
  }
  model.value = next
}, { immediate: true })

function setVal(key: string, v: unknown) {
  model.value = { ...model.value, [key]: v }
}

function onNumber(key: string, v: unknown) {
  const raw = String(v ?? '')
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
    // bool 跳过必填校验：false 是合法值，复选框无「未填」态。
    if (s.required && s.type !== 'boolean') {
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
  <table v-if="specs.length" class="w-full border-separate border-spacing-x-2 border-spacing-y-1 text-sm">
    <thead>
      <tr class="text-left text-xs text-muted-foreground">
        <th class="whitespace-nowrap font-medium">{{ t('script.params.key') }}</th>
        <th class="whitespace-nowrap font-medium">{{ t('script.params.type') }}</th>
        <th class="w-full font-medium">{{ t('script.params.value') }}</th>
        <th class="whitespace-nowrap font-medium">{{ t('script.params.desc') }}</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="s in specs" :key="s.key" class="align-top">
        <td class="whitespace-nowrap pt-2">
          <span class="font-mono">{{ s.key }}</span>
          <!-- bool 无必填概念（与 validate/编辑器一致），不显示必填星号。 -->
          <span v-if="s.required && s.type !== 'boolean'" class="ml-0.5 text-destructive">*</span>
        </td>
        <td class="whitespace-nowrap pt-2 text-xs text-muted-foreground">{{ s.type }}</td>
        <td>
          <!-- boolean -->
          <label v-if="s.type === 'boolean'" class="flex h-8 items-center gap-2">
            <Checkbox
              :model-value="model[s.key] === true"
              @update:model-value="(v) => setVal(s.key, v === true)"
            />
            <span class="text-muted-foreground">{{ model[s.key] === true ? 'true' : 'false' }}</span>
          </label>

          <!-- number -->
          <Input
            v-else-if="s.type === 'number'"
            type="number"
            class="h-8"
            :model-value="(model[s.key] as number) ?? ''"
            @update:model-value="(v) => onNumber(s.key, v)"
          />

          <!-- table：直接写 Lua table 字面量文本（数组或 {[k]=v} 均可） -->
          <Textarea
            v-else-if="s.type === 'table'"
            :model-value="(model[s.key] as string) ?? ''"
            :rows="1"
            class="min-h-8 font-mono text-xs"
            placeholder="{1, 2} 或 {[1]=2, [26]=5}"
            @update:model-value="(v) => setVal(s.key, String(v))"
          />

          <!-- string -->
          <Input
            v-else
            class="h-8"
            :model-value="(model[s.key] as string) ?? ''"
            @update:model-value="(v) => setVal(s.key, v)"
          />
        </td>
        <td class="pt-2 text-xs text-muted-foreground">{{ s.description }}</td>
      </tr>
    </tbody>
  </table>
</template>
