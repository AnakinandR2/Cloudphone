<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { AccountView, LedgerEntry } from '@/types/billing'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
import userApi from '@/api/modules/user'
import DataTable from '@/components/DataTable.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { formatDateTime } from '@/utils/date'
import { fmtCents, validateAdjustYuan } from '@/utils/money'

const { t } = useI18n()
const route = useRoute()

// 资源科目：调整入口用 seat/boot_slot/runtime_minute（对齐 adjust-resource V2 subject）。
// 容量展示读后端新模型字段 capacities_v2.{seat,boot_slot,runtime_minute}（与科目同名）。
type ResourceSubject = 'seat' | 'boot_slot' | 'runtime_minute'
const RESOURCE_SUBJECTS: ResourceSubject[] = ['seat', 'boot_slot', 'runtime_minute']

const userIdInput = ref('')
const currentUserId = ref<number | null>(null)
const accountView = ref<AccountView | null>(null)
const loading = ref(false)

const balanceDialog = ref(false)
const balanceForm = reactive({ yuan: '', reason: '' })
const balanceSubmitting = ref(false)

const resourceDialog = ref(false)
// quantity=台数（seat/boot_slot）或分钟数（runtime_minute）；duration_value=有效期(月/天，仅 seat/boot_slot)。
const resourceForm = reactive({ subject: 'seat' as ResourceSubject, quantity: '', durationValue: '30', reason: '' })
const resourceSubmitting = ref(false)

const ledgerColumns = computed<ColumnDef<LedgerEntry>[]>(() => [
  { accessorKey: 'created_at', id: 'created_at', header: t('billing.colCreatedAt'), meta: { label: 'billing.colCreatedAt' } },
  { accessorKey: 'subject', id: 'subject', header: t('billing.colSubject'), meta: { label: 'billing.colSubject' } },
  { accessorKey: 'type', id: 'type', header: t('billing.colLedgerType'), meta: { label: 'billing.colLedgerType' } },
  { accessorKey: 'delta', id: 'delta', header: t('billing.colDelta'), meta: { label: 'billing.colDelta' } },
  { accessorKey: 'balance_after', id: 'balance_after', header: t('billing.colBalanceAfter'), meta: { label: 'billing.colBalanceAfter' } },
  { accessorKey: 'reason', id: 'reason', header: t('billing.colReason'), meta: { label: 'billing.colReason' } },
  { accessorKey: 'operator', id: 'operator', header: t('billing.colOperator'), meta: { label: 'billing.colOperator' } },
])

function isBalanceSubject(subject: string) {
  return subject === 'balance'
}
function subjectUnit(s: string): string {
  return s === 'runtime_minute' ? t('billing.unitMinute') : t('billing.unitSeat')
}
function subjectLabel(subject: string): string {
  return t(`billing.subject_${subject}`)
}

function fmtDelta(entry: LedgerEntry): string {
  const { subject, delta } = entry
  const sign = delta >= 0 ? '+' : ''
  if (isBalanceSubject(subject)) return `${sign}¥${fmtCents(delta)}`
  return `${sign}${delta} ${subjectUnit(subject)}`
}
function fmtBalanceAfter(entry: LedgerEntry): string {
  const { subject, balance_after } = entry
  if (isBalanceSubject(subject)) return `¥${fmtCents(balance_after)}`
  return `${balance_after} ${subjectUnit(subject)}`
}
function deltaClass(delta: number): string {
  return delta >= 0 ? 'text-emerald-600 tabular-nums' : 'text-destructive tabular-nums'
}

function capacityValue(subject: ResourceSubject): number {
  const caps = accountView.value?.capacities_v2
  if (!caps) return 0
  return caps[subject] ?? 0
}

async function fetchAccount(uid: number) {
  loading.value = true
  try {
    const res = await billingApi.account(uid)
    accountView.value = res.data
  }
  catch {
    toast.error(t('billing.loadFail'))
  }
  finally {
    loading.value = false
  }
}

async function queryAccount() {
  const s = userIdInput.value.trim()
  if (!s) {
    toast.error(t('billing.errUserIdRequired'))
    return
  }
  let uid: number
  if (/^1\d{10}$/.test(s)) {
    // 11 位手机号：查用户列表精确匹配取 id
    try {
      const res = await userApi.list({ page: 1, size: 5, phone: s })
      const hit = res.data.list.find(u => u.phone === s)
      if (!hit) {
        toast.error(t('billing.errUserNotFound'))
        return
      }
      uid = hit.id
    }
    catch {
      toast.error(t('billing.loadFail'))
      return
    }
  }
  else {
    uid = Number(s)
    if (!uid || uid <= 0) {
      toast.error(t('billing.errUserIdRequired'))
      return
    }
  }
  currentUserId.value = uid
  accountView.value = null
  await fetchAccount(uid)
}

function reloadAccount() {
  if (currentUserId.value) fetchAccount(currentUserId.value)
}

onMounted(() => {
  // 从用户管理跳转携带 userId：自动填入并查询。
  const qid = route.query.userId
  const uid = Array.isArray(qid) ? qid[0] : qid
  if (uid) {
    userIdInput.value = String(uid)
    queryAccount()
  }
})

function openBalanceDialog() {
  balanceForm.yuan = ''
  balanceForm.reason = ''
  balanceDialog.value = true
}

// 余额调整金额校验（CP-0070 / #59）：超两位小数就地飘红，绝不静默截断/进位。
const balanceCheck = computed(() =>
  validateAdjustYuan(balanceForm.yuan === '' ? null : Number(balanceForm.yuan)),
)
// 仅对「已输入但小数位过多」就地飘红；空/0 沿用提交时 toast 提示。
const balanceAmountError = computed(() =>
  balanceForm.yuan !== '' && balanceCheck.value.error === 'precision'
    ? t('billing.fAdjustAmountPrecision')
    : '',
)

async function submitBalance() {
  if (!currentUserId.value) return
  if (!balanceForm.reason.trim()) {
    toast.error(t('billing.errReasonRequired'))
    return
  }
  const check = validateAdjustYuan(balanceForm.yuan === '' ? null : Number(balanceForm.yuan))
  if (check.error === 'precision') {
    toast.error(t('billing.fAdjustAmountPrecision')) // 超两位小数：拒绝提交，不再静默改写输入
    return
  }
  if (check.error) {
    toast.error(t('billing.errInvalidAmount'))
    return
  }
  const cents = check.cents
  balanceSubmitting.value = true
  try {
    await billingApi.adjustBalance(currentUserId.value, cents, balanceForm.reason.trim())
    toast.success(t('billing.adjustOk'))
    balanceDialog.value = false
    reloadAccount()
  }
  catch {
    toast.error(t('billing.adjustFail'))
  }
  finally {
    balanceSubmitting.value = false
  }
}

function openResourceDialog() {
  resourceForm.subject = 'seat'
  resourceForm.quantity = ''
  resourceForm.durationValue = '30'
  resourceForm.reason = ''
  resourceDialog.value = true
}

const isRuntimeSubject = computed(() => resourceForm.subject === 'runtime_minute')

async function submitResource() {
  if (!currentUserId.value) return
  if (!resourceForm.reason.trim()) {
    toast.error(t('billing.errReasonRequired'))
    return
  }
  const qty = Number(resourceForm.quantity)
  if (!Number.isInteger(qty) || qty <= 0) {
    toast.error(t('billing.errInvalidDelta'))
    return
  }
  resourceSubmitting.value = true
  try {
    const reason = resourceForm.reason.trim()
    if (resourceForm.subject === 'runtime_minute') {
      // runtime_minute：用 minutes
      await billingApi.adjustResource(currentUserId.value, { subject: 'runtime_minute', minutes: qty, reason })
    }
    else {
      // seat/boot_slot：quantity=台数，duration_value=有效期
      const durationValue = Number(resourceForm.durationValue)
      if (!Number.isInteger(durationValue) || durationValue <= 0) {
        toast.error(t('billing.errInvalidDelta'))
        resourceSubmitting.value = false
        return
      }
      await billingApi.adjustResource(currentUserId.value, { subject: resourceForm.subject, quantity: qty, duration_value: durationValue, reason })
    }
    toast.success(t('billing.adjustOk'))
    resourceDialog.value = false
    reloadAccount()
  }
  catch {
    toast.error(t('billing.adjustFail'))
  }
  finally {
    resourceSubmitting.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <Card>
      <CardHeader>
        <CardTitle>{{ t('billing.accountsTitle') }}</CardTitle>
        <CardDescription>{{ t('billing.accountsDesc') }}</CardDescription>
      </CardHeader>
      <CardContent class="space-y-6">
        <div class="flex items-center gap-2">
          <Input
            v-model="userIdInput"
            class="h-9 w-48"
            :placeholder="t('billing.inputUserIdOrPhone')"
            @keyup.enter="queryAccount"
          />
          <Button size="sm" :disabled="loading" @click="queryAccount">
            {{ t('common.search') }}
          </Button>
        </div>

        <div v-if="!accountView && !loading" class="text-muted-foreground py-8 text-center text-sm">
          {{ t('billing.accountQueryHint') }}
        </div>

        <template v-if="accountView">
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <Card class="bg-muted/30">
              <CardContent class="pt-4">
                <p class="text-muted-foreground text-xs">
                  {{ t('billing.accountBalance') }}
                </p>
                <p class="mt-1 text-2xl font-semibold tabular-nums">
                  ¥{{ fmtCents(accountView.account.balance_cents) }}
                </p>
              </CardContent>
            </Card>
            <Card v-for="s in RESOURCE_SUBJECTS" :key="s" class="bg-muted/30">
              <CardContent class="pt-4">
                <p class="text-muted-foreground text-xs">
                  {{ subjectLabel(s) }}
                </p>
                <p class="mt-1 text-2xl font-semibold tabular-nums">
                  {{ capacityValue(s) }}
                  <span class="text-muted-foreground text-sm font-normal">{{ subjectUnit(s) }}</span>
                </p>
              </CardContent>
            </Card>
          </div>

          <div class="flex gap-2">
            <Button v-auth="'billing:manage'" variant="outline" size="sm" @click="openBalanceDialog">
              {{ t('billing.adjustBalance') }}
            </Button>
            <Button v-auth="'billing:manage'" variant="outline" size="sm" @click="openResourceDialog">
              {{ t('billing.adjustResource') }}
            </Button>
          </div>

          <div>
            <p class="text-muted-foreground mb-2 text-xs">
              {{ t('billing.ledgerHint', { total: accountView.ledger_total }) }}
            </p>
            <DataTable :columns="ledgerColumns" :data="accountView.ledger" :loading="loading">
              <template #cell-created_at="{ row }">
                <span class="tabular-nums text-muted-foreground text-xs">{{ formatDateTime(row.created_at) }}</span>
              </template>
              <template #cell-subject="{ row }">
                <Badge variant="outline" class="text-xs">
                  {{ subjectLabel(row.subject) }}
                </Badge>
              </template>
              <template #cell-type="{ row }">
                <span class="text-muted-foreground text-xs">{{ t(`billing.ledgerType_${row.type}`) }}</span>
              </template>
              <template #cell-delta="{ row }">
                <span :class="deltaClass(row.delta)">{{ fmtDelta(row) }}</span>
              </template>
              <template #cell-balance_after="{ row }">
                <span class="tabular-nums text-xs">{{ fmtBalanceAfter(row) }}</span>
              </template>
              <template #cell-reason="{ row }">
                <span class="text-muted-foreground text-xs">{{ row.reason || '-' }}</span>
              </template>
              <template #cell-operator="{ row }">
                <span class="text-muted-foreground text-xs">{{ row.operator || '-' }}</span>
              </template>
            </DataTable>
          </div>
        </template>
      </CardContent>
    </Card>

    <!-- 余额调整 Dialog -->
    <Dialog v-model:open="balanceDialog">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ t('billing.adjustBalanceTitle') }}</DialogTitle>
        </DialogHeader>
        <div class="space-y-4 py-2">
          <div class="space-y-1.5">
            <Label>{{ t('billing.fAdjustAmount') }}</Label>
            <Input
              v-model="balanceForm.yuan" type="number" step="0.01"
              :class="balanceAmountError ? 'border-destructive focus-visible:ring-destructive' : ''"
              :placeholder="t('billing.fAdjustAmountPlaceholder')"
            />
            <p v-if="balanceAmountError" class="text-destructive text-xs">
              {{ balanceAmountError }}
            </p>
            <p v-else class="text-muted-foreground text-xs">
              {{ t('billing.fAdjustAmountHint') }}
            </p>
          </div>
          <div class="space-y-1.5">
            <Label>{{ t('billing.fAdjustReason') }}<span class="text-destructive ml-1">*</span></Label>
            <Input v-model="balanceForm.reason" :placeholder="t('billing.fAdjustReasonPlaceholder')" />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="balanceDialog = false">
            {{ t('crud.cancel') }}
          </Button>
          <Button :disabled="balanceSubmitting || !!balanceAmountError" @click="submitBalance">
            {{ balanceSubmitting ? t('common.loading') : t('crud.confirm') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- 资源赠送/扣减 Dialog -->
    <Dialog v-model:open="resourceDialog">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ t('billing.adjustResourceTitle') }}</DialogTitle>
        </DialogHeader>
        <div class="space-y-4 py-2">
          <div class="space-y-1.5">
            <Label>{{ t('billing.fAdjustSubject') }}</Label>
            <Select v-model="resourceForm.subject">
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="s in RESOURCE_SUBJECTS" :key="s" :value="s">
                  {{ subjectLabel(s) }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="space-y-1.5">
            <Label>{{ isRuntimeSubject ? t('billing.fAdjustMinutes') : t('billing.fAdjustQuantity') }}</Label>
            <Input v-model="resourceForm.quantity" type="number" min="1" step="1" :placeholder="t('billing.fAdjustQuantityPlaceholder')" />
            <p class="text-muted-foreground text-xs">
              {{ t('billing.fAdjustResourceHint') }}
            </p>
          </div>
          <div v-if="!isRuntimeSubject" class="space-y-1.5">
            <Label>{{ t('billing.fAdjustDuration') }}</Label>
            <Input v-model="resourceForm.durationValue" type="number" min="1" step="1" :placeholder="t('billing.fAdjustDurationPlaceholder')" />
            <p class="text-muted-foreground text-xs">
              {{ t('billing.fAdjustDurationHint') }}
            </p>
          </div>
          <div class="space-y-1.5">
            <Label>{{ t('billing.fAdjustReason') }}<span class="text-destructive ml-1">*</span></Label>
            <Input v-model="resourceForm.reason" :placeholder="t('billing.fAdjustReasonPlaceholder')" />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="resourceDialog = false">
            {{ t('crud.cancel') }}
          </Button>
          <Button :disabled="resourceSubmitting" @click="submitResource">
            {{ resourceSubmitting ? t('common.loading') : t('crud.confirm') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
