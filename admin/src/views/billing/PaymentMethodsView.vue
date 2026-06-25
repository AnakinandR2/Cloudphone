<script setup lang="ts">
import type { PaymentMethod } from '@/types/billing'
import { GripVertical, Loader2, Upload } from 'lucide-vue-next'
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)

// 行内编辑态：手续费以「%」与「¥」展示编辑，保存时再换算回 bps/cents。
interface MethodRow extends PaymentMethod {
  // 比例手续费（%）展示值，bps/100（200bps → 2）
  feePercent: number
  // 固定手续费（¥）展示值，cents/100（100cents → 1）
  feeFixed: number
  // 满额免手续费阈值（¥）展示值，cents/100；0=不免
  feeFreeThreshold: number
}

const methods = ref<MethodRow[]>([])
// 每行 logo 上传进行态，按 code 索引。
const uploading = reactive<Record<string, boolean>>({})
// 每行 logo 加载失败标记（按 code），仅用于回退首字方块——不要去改 logo_url（那是持久化字段）。
const logoError = reactive<Record<string, boolean>>({})

function toRow(m: PaymentMethod): MethodRow {
  // 余额方式手续费与阈值恒为 0，编辑态固定显示 0（输入框另在模板内 disable）。
  const isBalance = m.code === 'balance'
  return {
    ...m,
    logo_url: m.logo_url ?? '',
    feePercent: isBalance ? 0 : (m.fee_percent_bps ?? 0) / 100,
    feeFixed: isBalance ? 0 : (m.fee_fixed_cents ?? 0) / 100,
    feeFreeThreshold: isBalance ? 0 : (m.fee_free_threshold_cents ?? 0) / 100,
  }
}

// 选图即传到 S3，存返回 URL 回填到该行 logo_url。
async function pickLogo(e: Event, m: MethodRow) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file)
    return
  uploading[m.code] = true
  try {
    const res = await billingApi.uploadPayLogo(file)
    m.logo_url = res.data.url
    logoError[m.code] = false // 新 logo，清掉旧的失败标记
  }
  catch {
    toast.error(t('billing.updateFail'))
  }
  finally {
    uploading[m.code] = false
    input.value = '' // 允许重复选同一文件
  }
}

async function load() {
  loading.value = true
  try {
    const { data } = await billingApi.getPaymentMethods()
    methods.value = [...(data.payment_methods ?? [])].sort((a, b) => a.sort - b.sort).map(toRow)
  }
  catch {
    toast.error(t('billing.loadFail'))
  }
  finally {
    loading.value = false
  }
}
onMounted(load)

// ── 原生 HTML5 拖拽排序（无第三方依赖）。
const dragIndex = ref<number | null>(null)
const overIndex = ref<number | null>(null)

function onDragStart(idx: number) {
  dragIndex.value = idx
}
function onDragOver(idx: number) {
  if (dragIndex.value === null) return
  overIndex.value = idx
}
function onDrop(idx: number) {
  const from = dragIndex.value
  dragIndex.value = null
  overIndex.value = null
  if (from === null || from === idx) return
  const arr = methods.value
  const [moved] = arr.splice(from, 1)
  arr.splice(idx, 0, moved)
}
function onDragEnd() {
  dragIndex.value = null
  overIndex.value = null
}

async function save() {
  saving.value = true
  try {
    // 以当前展示顺序重写 sort（0..n-1）；手续费与满额阈值换算回 bps/cents。
    // 余额方式（balance）手续费与阈值恒为 0（前端兜底，后端校验为最终保证）。
    const payload: PaymentMethod[] = methods.value.map((m, i) => {
      const isBalance = m.code === 'balance'
      const { feePercent, feeFixed, feeFreeThreshold, ...base } = m
      return {
        ...base,
        sort: i,
        logo_url: m.logo_url ?? '',
        fee_percent_bps: isBalance ? 0 : Math.round((feePercent || 0) * 100),
        fee_fixed_cents: isBalance ? 0 : Math.round((feeFixed || 0) * 100),
        fee_free_threshold_cents: isBalance ? 0 : Math.round((feeFreeThreshold || 0) * 100),
      }
    })
    await billingApi.savePaymentMethods(payload)
    toast.success(t('billing.savedOk'))
    load()
  }
  catch {
    toast.error(t('billing.updateFail'))
  }
  finally {
    saving.value = false
  }
}
</script>

<template>
  <Card>
    <CardContent class="pt-6">
      <div class="max-w-6xl space-y-5">
        <div class="flex items-start justify-between gap-3">
          <div class="space-y-1.5">
            <h2 class="text-lg font-semibold">
              {{ t('billing.payTitle') }}
            </h2>
            <p class="text-muted-foreground text-sm">
              {{ t('billing.payDesc') }}
            </p>
          </div>
          <Button v-auth="'billing:manage'" :disabled="saving || loading" @click="save">
            {{ saving ? t('common.loading') : t('crud.save') }}
          </Button>
        </div>

        <!-- 表格展示：仅左侧手柄可拖拽排序（行本身不 draggable，便于在输入框里框选文本）。 -->
        <div class="overflow-x-auto rounded-md border">
          <table class="w-full text-sm">
            <thead>
              <tr class="text-muted-foreground bg-muted/40 border-b text-left text-xs">
                <th class="w-8 px-2 py-2" />
                <th class="w-10 px-2 py-2 text-center">
                  #
                </th>
                <th class="px-3 py-2 font-medium">
                  {{ t('billing.payName') }}
                </th>
                <th class="px-3 py-2 font-medium">
                  {{ t('billing.payLogo') }}
                </th>
                <th class="px-3 py-2 font-medium">
                  {{ t('billing.payFeePercent') }}
                </th>
                <th class="px-3 py-2 font-medium">
                  {{ t('billing.payFeeFixed') }}
                </th>
                <th class="px-3 py-2 font-medium">
                  {{ t('billing.payFeeFreeThreshold') }}
                </th>
                <th class="w-16 px-3 py-2 text-center font-medium">
                  {{ t('billing.payEnabled') }}
                </th>
              </tr>
            </thead>
            <tbody class="divide-y">
              <tr
                v-for="(m, idx) in methods"
                :key="m.code"
                class="transition-colors"
                :class="[
                  dragIndex === idx ? 'opacity-50' : '',
                  overIndex === idx && dragIndex !== null && dragIndex !== idx ? 'bg-accent' : '',
                ]"
                @dragover.prevent="onDragOver(idx)"
                @drop="onDrop(idx)"
              >
                <!-- 拖拽手柄：只有它 draggable，避免在输入框框选时误触发拖拽 -->
                <td class="px-2 py-2 align-middle">
                  <span
                    class="text-muted-foreground inline-flex cursor-grab active:cursor-grabbing"
                    :title="t('billing.payDragHint')"
                    draggable="true"
                    @dragstart="onDragStart(idx)"
                    @dragend="onDragEnd"
                  >
                    <GripVertical class="size-4" />
                  </span>
                </td>
                <td class="text-muted-foreground px-2 py-2 text-center align-middle text-xs tabular-nums">
                  {{ idx + 1 }}
                </td>
                <td class="px-3 py-2 align-middle">
                  <div class="flex flex-col gap-1">
                    <span class="text-muted-foreground font-mono text-xs">{{ m.code }}</span>
                    <Input v-model="m.name" class="h-8 w-40" :placeholder="t('billing.payName')" />
                  </div>
                </td>
                <!-- 渠道 Logo：小预览（img，回退首字方块）+ 上传按钮 + 可编辑 URL 文本框 -->
                <td class="px-3 py-2 align-middle">
                  <div class="flex items-center gap-2">
                    <span class="bg-muted/40 flex size-8 shrink-0 items-center justify-center overflow-hidden rounded border">
                      <Loader2 v-if="uploading[m.code]" class="text-muted-foreground size-4 animate-spin" />
                      <img
                        v-else-if="m.logo_url && !logoError[m.code]"
                        :src="m.logo_url"
                        class="size-full object-contain"
                        :alt="m.name"
                        @error="logoError[m.code] = true"
                      >
                      <span v-else class="text-muted-foreground text-xs font-medium">{{ (m.name || m.code).slice(0, 1) }}</span>
                    </span>
                    <label class="inline-flex">
                      <Button as="span" variant="outline" size="sm" class="h-8 cursor-pointer gap-1">
                        <Upload class="size-3.5" />
                        {{ t('billing.payLogoUpload') }}
                      </Button>
                      <input type="file" accept="image/*" class="hidden" :disabled="uploading[m.code]" @change="(e) => pickLogo(e, m)">
                    </label>
                    <Input v-model="m.logo_url" class="h-8 w-44" :placeholder="t('billing.payLogoUrlPlaceholder')" @update:model-value="logoError[m.code] = false" />
                  </div>
                </td>
                <td class="px-3 py-2 align-middle">
                  <Input
                    v-model.number="m.feePercent"
                    type="number"
                    min="0"
                    max="100"
                    step="0.01"
                    class="h-8 w-24"
                    :disabled="m.code === 'balance'"
                  />
                </td>
                <td class="px-3 py-2 align-middle">
                  <Input
                    v-model.number="m.feeFixed"
                    type="number"
                    min="0"
                    step="0.01"
                    class="h-8 w-24"
                    :disabled="m.code === 'balance'"
                  />
                </td>
                <td class="px-3 py-2 align-middle">
                  <Input
                    v-model.number="m.feeFreeThreshold"
                    type="number"
                    min="0"
                    step="0.01"
                    class="h-8 w-24"
                    :disabled="m.code === 'balance'"
                  />
                </td>
                <td class="px-3 py-2 text-center align-middle">
                  <Switch v-model="m.enabled" />
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <p v-if="!methods.length && !loading" class="text-muted-foreground py-6 text-center text-sm">
          {{ t('billing.payEmpty') }}
        </p>
      </div>
    </CardContent>
  </Card>
</template>
