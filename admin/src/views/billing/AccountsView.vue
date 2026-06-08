<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { AccountView, LedgerEntry } from '@/types/billing'
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
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
import { fmtCents } from '@/utils/money'

const { t } = useI18n()

const userIdInput = ref('')
const currentUserId = ref<number | null>(null)
const accountView = ref<AccountView | null>(null)
const loading = ref(false)

// ── 余额调整 dialog ──────────────────────────────────────────
const balanceDialog = ref(false)
const balanceForm = reactive({ yuan: '', reason: '' })
const balanceSubmitting = ref(false)

// ── 资源调整 dialog ──────────────────────────────────────────
const resourceDialog = ref(false)
const resourceForm = reactive({ subject: 'instance_seat', delta: '', reason: '' })
const resourceSubmitting = ref(false)

// ── Ledger 列定义 ─────────────────────────────────────────────
const ledgerColumns = computed<ColumnDef<LedgerEntry>[]>(() => [
  {
    accessorKey: 'created_at',
    id: 'created_at',
    header: t('billing.colCreatedAt'),
    meta: { label: 'billing.colCreatedAt' },
  },
  {
    accessorKey: 'subject',
    id: 'subject',
    header: t('billing.colSubject'),
    meta: { label: 'billing.colSubject' },
  },
  {
    accessorKey: 'type',
    id: 'type',
    header: t('billing.colLedgerType'),
    meta: { label: 'billing.colLedgerType' },
  },
  {
    accessorKey: 'delta',
    id: 'delta',
    header: t('billing.colDelta'),
    meta: { label: 'billing.colDelta' },
  },
  {
    accessorKey: 'balance_after',
    id: 'balance_after',
    header: t('billing.colBalanceAfter'),
    meta: { label: 'billing.colBalanceAfter' },
  },
  {
    accessorKey: 'reason',
    id: 'reason',
    header: t('billing.colReason'),
    meta: { label: 'billing.colReason' },
  },
  {
    accessorKey: 'operator',
    id: 'operator',
    header: t('billing.colOperator'),
    meta: { label: 'billing.colOperator' },
  },
])

// 判断科目是否为余额（单位：分）
function isBalanceSubject(subject: string) {
  return subject === 'balance'
}

function fmtDelta(entry: LedgerEntry): string {
  const { subject, delta } = entry
  const sign = delta >= 0 ? '+' : ''
  if (isBalanceSubject(subject)) {
    return `${sign}¥${fmtCents(delta)}`
  }
  // 资源类：instance_seat / boot_seat → 台；runtime_minute → 分钟
  const unit = subject === 'runtime_minute' ? t('billing.unitMinute') : t('billing.unitSeat')
  return `${sign}${delta} ${unit}`
}

function fmtBalanceAfter(entry: LedgerEntry): string {
  const { subject, balance_after } = entry
  if (isBalanceSubject(subject)) {
    return `¥${fmtCents(balance_after)}`
  }
  const unit = subject === 'runtime_minute' ? t('billing.unitMinute') : t('billing.unitSeat')
  return `${balance_after} ${unit}`
}

function deltaClass(delta: number): string {
  return delta >= 0 ? 'text-emerald-600 tabular-nums' : 'text-destructive tabular-nums'
}

async function queryAccount() {
  const uid = Number(userIdInput.value.trim())
  if (!uid || uid <= 0) {
    toast.error(t('billing.errUserIdRequired'))
    return
  }
  currentUserId.value = uid
  loading.value = true
  accountView.value = null
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

async function reloadAccount() {
  if (!currentUserId.value) return
  loading.value = true
  try {
    const res = await billingApi.account(currentUserId.value)
    accountView.value = res.data
  }
  catch {
    toast.error(t('billing.loadFail'))
  }
  finally {
    loading.value = false
  }
}

// ── 余额调整 ─────────────────────────────────────────────────
function openBalanceDialog() {
  balanceForm.yuan = ''
  balanceForm.reason = ''
  balanceDialog.value = true
}

async function submitBalance() {
  if (!currentUserId.value) return
  if (!balanceForm.reason.trim()) {
    toast.error(t('billing.errReasonRequired'))
    return
  }
  const yuan = Number(balanceForm.yuan)
  const cents = Math.round(yuan * 100)
  if (Number.isNaN(yuan) || cents === 0) {
    toast.error(t('billing.errInvalidAmount'))
    return
  }
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

// ── 资源调整 ─────────────────────────────────────────────────
function openResourceDialog() {
  resourceForm.subject = 'instance_seat'
  resourceForm.delta = ''
  resourceForm.reason = ''
  resourceDialog.value = true
}

async function submitResource() {
  if (!currentUserId.value) return
  if (!resourceForm.reason.trim()) {
    toast.error(t('billing.errReasonRequired'))
    return
  }
  const delta = Number(resourceForm.delta)
  if (!Number.isInteger(delta) || delta === 0) {
    toast.error(t('billing.errInvalidDelta'))
    return
  }
  resourceSubmitting.value = true
  try {
    await billingApi.adjustResource(currentUserId.value, resourceForm.subject, delta, resourceForm.reason.trim())
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

function subjectLabel(subject: string): string {
  return t(`billing.subject_${subject}`)
}
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('billing.accountsTitle') }}</CardTitle>
      <CardDescription>{{ t('billing.accountsDesc') }}</CardDescription>
    </CardHeader>
    <CardContent class="space-y-6">
      <!-- 查询区 -->
      <div class="flex items-center gap-2">
        <Input
          v-model="userIdInput"
          type="number"
          class="h-9 w-40"
          :placeholder="t('billing.inputUserId')"
          @keyup.enter="queryAccount"
        />
        <Button size="sm" :disabled="loading" @click="queryAccount">
          {{ t('common.search') }}
        </Button>
      </div>

      <!-- 空态提示 -->
      <div v-if="!accountView && !loading" class="text-muted-foreground py-8 text-center text-sm">
        {{ t('billing.accountQueryHint') }}
      </div>

      <!-- 账户概览 -->
      <template v-if="accountView">
        <!-- 概览卡片 -->
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <!-- 余额 -->
          <Card class="bg-muted/30">
            <CardContent class="pt-4">
              <p class="text-muted-foreground text-xs">{{ t('billing.accountBalance') }}</p>
              <p class="mt-1 text-2xl font-semibold tabular-nums">¥{{ fmtCents(accountView.account.balance_cents) }}</p>
            </CardContent>
          </Card>
          <!-- instance_seat -->
          <Card class="bg-muted/30">
            <CardContent class="pt-4">
              <p class="text-muted-foreground text-xs">{{ t('billing.subject_instance_seat') }}</p>
              <p class="mt-1 text-2xl font-semibold tabular-nums">
                {{ accountView.capacities.instance_seat }} <span class="text-muted-foreground text-sm font-normal">{{ t('billing.unitSeat') }}</span>
              </p>
            </CardContent>
          </Card>
          <!-- boot_seat -->
          <Card class="bg-muted/30">
            <CardContent class="pt-4">
              <p class="text-muted-foreground text-xs">{{ t('billing.subject_boot_seat') }}</p>
              <p class="mt-1 text-2xl font-semibold tabular-nums">
                {{ accountView.capacities.boot_seat }} <span class="text-muted-foreground text-sm font-normal">{{ t('billing.unitSeat') }}</span>
              </p>
            </CardContent>
          </Card>
          <!-- runtime_minute -->
          <Card class="bg-muted/30">
            <CardContent class="pt-4">
              <p class="text-muted-foreground text-xs">{{ t('billing.subject_runtime_minute') }}</p>
              <p class="mt-1 text-2xl font-semibold tabular-nums">
                {{ accountView.capacities.runtime_minute }} <span class="text-muted-foreground text-sm font-normal">{{ t('billing.unitMinute') }}</span>
              </p>
            </CardContent>
          </Card>
        </div>

        <!-- 操作按钮 -->
        <div class="flex gap-2">
          <Button v-auth="'billing:manage'" variant="outline" size="sm" @click="openBalanceDialog">
            {{ t('billing.adjustBalance') }}
          </Button>
          <Button v-auth="'billing:manage'" variant="outline" size="sm" @click="openResourceDialog">
            {{ t('billing.adjustResource') }}
          </Button>
        </div>

        <!-- 流水表 -->
        <div>
          <p class="text-muted-foreground mb-2 text-xs">
            {{ t('billing.ledgerHint', { total: accountView.ledger_total }) }}
          </p>
          <DataTable
            :columns="ledgerColumns"
            :data="accountView.ledger"
            :loading="loading"
          >
            <template #cell-created_at="{ row }">
              <span class="tabular-nums text-muted-foreground text-xs">{{ formatDateTime(row.created_at) }}</span>
            </template>
            <template #cell-subject="{ row }">
              <Badge variant="outline" class="text-xs">{{ subjectLabel(row.subject) }}</Badge>
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
            v-model="balanceForm.yuan"
            type="number"
            step="0.01"
            :placeholder="t('billing.fAdjustAmountPlaceholder')"
          />
          <p class="text-muted-foreground text-xs">{{ t('billing.fAdjustAmountHint') }}</p>
        </div>
        <div class="space-y-1.5">
          <Label>{{ t('billing.fAdjustReason') }}<span class="text-destructive ml-1">*</span></Label>
          <Input
            v-model="balanceForm.reason"
            :placeholder="t('billing.fAdjustReasonPlaceholder')"
          />
        </div>
      </div>
      <DialogFooter>
        <Button variant="outline" @click="balanceDialog = false">{{ t('crud.cancel') }}</Button>
        <Button :disabled="balanceSubmitting" @click="submitBalance">
          {{ balanceSubmitting ? t('common.loading') : t('crud.confirm') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <!-- 资源调整 Dialog -->
  <Dialog v-model:open="resourceDialog">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ t('billing.adjustResourceTitle') }}</DialogTitle>
      </DialogHeader>
      <div class="space-y-4 py-2">
        <div class="space-y-1.5">
          <Label>{{ t('billing.fAdjustSubject') }}</Label>
          <Select v-model="resourceForm.subject">
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="instance_seat">{{ t('billing.subject_instance_seat') }}</SelectItem>
              <SelectItem value="boot_seat">{{ t('billing.subject_boot_seat') }}</SelectItem>
              <SelectItem value="runtime_minute">{{ t('billing.subject_runtime_minute') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="space-y-1.5">
          <Label>{{ t('billing.fAdjustDelta') }}</Label>
          <Input
            v-model="resourceForm.delta"
            type="number"
            step="1"
            :placeholder="t('billing.fAdjustDeltaPlaceholder')"
          />
          <p class="text-muted-foreground text-xs">{{ t('billing.fAdjustDeltaHint') }}</p>
        </div>
        <div class="space-y-1.5">
          <Label>{{ t('billing.fAdjustReason') }}<span class="text-destructive ml-1">*</span></Label>
          <Input
            v-model="resourceForm.reason"
            :placeholder="t('billing.fAdjustReasonPlaceholder')"
          />
        </div>
      </div>
      <DialogFooter>
        <Button variant="outline" @click="resourceDialog = false">{{ t('crud.cancel') }}</Button>
        <Button :disabled="resourceSubmitting" @click="submitResource">
          {{ resourceSubmitting ? t('common.loading') : t('crud.confirm') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
