<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'
import type { TrialGrant, TrialPolicy } from '@/types/billing'
import { Pencil, Plus, Trash2 } from 'lucide-vue-next'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import billingApi from '@/api/modules/billing'
import DataTable from '@/components/DataTable.vue'
import Popconfirm from '@/components/Popconfirm.vue'
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
import { Switch } from '@/components/ui/switch'
import { formatDateTime } from '@/utils/date'

const { t } = useI18n()

const policies = ref<TrialPolicy[]>([])
const loading = ref(false)
const q = ref('')

// ── 授予资格 userId inputs per row (keyed by policy id) ──────
const grantUserIds = reactive<Record<number, string>>({})
const granting = reactive<Record<number, boolean>>({})

// ── 发放记录 per row ─────────────────────────────────────────
const grants = reactive<Record<number, TrialGrant[]>>({})
const grantsLoading = reactive<Record<number, boolean>>({})

const SUBJECTS = ['instance_seat', 'boot_seat', 'runtime_minute'] as const

function subjectLabel(s: string): string {
  return t(`billing.subject_${s}`)
}

function subjectUnit(s: string): string {
  return s === 'runtime_minute' ? t('billing.unitMinute') : t('billing.unitSeat')
}

function grantDesc(p: TrialPolicy): string {
  const subj = subjectLabel(p.grant_subject)
  const unit = subjectUnit(p.grant_subject)
  let s = `${subj} × ${p.grant_quantity} ${unit}`
  if (p.grant_expire_days > 0) {
    s += `（${t('trial.nDaysValid', { n: p.grant_expire_days })}）`
  }
  else {
    s += `（${t('trial.permanent')}）`
  }
  return s
}

const columns = computed<ColumnDef<TrialPolicy>[]>(() => [
  { id: 'expander', header: '', enableHiding: false, meta: { label: '' } },
  { accessorKey: 'code', id: 'code', header: t('trial.fCode'), meta: { label: 'trial.fCode' } },
  { accessorKey: 'name', id: 'name', header: t('trial.fName'), meta: { label: 'trial.fName' } },
  { accessorKey: 'enabled', id: 'enabled', header: t('trial.fEnabled'), meta: { label: 'trial.fEnabled' } },
  { id: 'grant', header: t('trial.colGrant'), meta: { label: 'trial.colGrant' } },
  { accessorKey: 'per_user_limit', id: 'per_user_limit', header: t('trial.fPerUserLimit'), meta: { label: 'trial.fPerUserLimit' } },
  { accessorKey: 'allow_new_user', id: 'allow_new_user', header: t('trial.fAllowNewUser'), meta: { label: 'trial.fAllowNewUser' } },
  { accessorKey: 'invite_code', id: 'invite_code', header: t('trial.fInviteCode'), meta: { label: 'trial.fInviteCode' } },
  { id: 'actions', header: '', enableHiding: false, meta: { label: 'crud.actions', headClass: 'text-right', cellClass: 'text-right whitespace-nowrap' } },
])

async function load() {
  loading.value = true
  try {
    const res = await billingApi.listTrials()
    policies.value = res.data
  }
  catch {
    toast.error(t('billing.loadFail'))
  }
  finally {
    loading.value = false
  }
}

onMounted(load)

// ── Dialog ───────────────────────────────────────────────────
const dialogOpen = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)

interface FormState {
  code: string
  name: string
  grant_subject: string
  grant_quantity: number
  grant_expire_days: number
  per_user_limit: number
  allow_new_user: boolean
  invite_code: string
  enabled: boolean
}

const form = reactive<FormState>({
  code: '',
  name: '',
  grant_subject: 'instance_seat',
  grant_quantity: 1,
  grant_expire_days: 0,
  per_user_limit: 1,
  allow_new_user: false,
  invite_code: '',
  enabled: true,
})

function openCreate() {
  editingId.value = null
  Object.assign(form, {
    code: '',
    name: '',
    grant_subject: 'instance_seat',
    grant_quantity: 1,
    grant_expire_days: 0,
    per_user_limit: 1,
    allow_new_user: false,
    invite_code: '',
    enabled: true,
  })
  dialogOpen.value = true
}

function openEdit(p: TrialPolicy) {
  editingId.value = p.id
  Object.assign(form, {
    code: p.code,
    name: p.name,
    grant_subject: p.grant_subject,
    grant_quantity: p.grant_quantity,
    grant_expire_days: p.grant_expire_days,
    per_user_limit: p.per_user_limit,
    allow_new_user: p.allow_new_user,
    invite_code: p.invite_code ?? '',
    enabled: p.enabled,
  })
  dialogOpen.value = true
}

async function save() {
  if (form.grant_quantity < 1) {
    toast.error(t('trial.errGrantQty'))
    return
  }
  if (form.per_user_limit < 1) {
    toast.error(t('trial.errPerUserLimit'))
    return
  }
  saving.value = true
  try {
    if (editingId.value === null) {
      await billingApi.createTrial({
        code: form.code,
        name: form.name,
        grant_subject: form.grant_subject,
        grant_quantity: form.grant_quantity,
        grant_expire_days: form.grant_expire_days,
        per_user_limit: form.per_user_limit,
        allow_new_user: form.allow_new_user,
        invite_code: form.invite_code || undefined,
        enabled: form.enabled,
      })
      toast.success(t('crud.createOk'))
    }
    else {
      await billingApi.updateTrial(editingId.value, {
        name: form.name,
        grant_quantity: form.grant_quantity,
        grant_expire_days: form.grant_expire_days,
        per_user_limit: form.per_user_limit,
        allow_new_user: form.allow_new_user,
        invite_code: form.invite_code || undefined,
        enabled: form.enabled,
      })
      toast.success(t('crud.updateOk'))
    }
    dialogOpen.value = false
    load()
  }
  catch {
    toast.error(editingId.value === null ? t('billing.createFail') : t('billing.updateFail'))
  }
  finally {
    saving.value = false
  }
}

async function remove(p: TrialPolicy) {
  try {
    await billingApi.deleteTrial(p.id)
    toast.success(t('crud.deleteOk'))
    load()
  }
  catch {
    toast.error(t('billing.deleteFail'))
  }
}

// ── 展开行：授予资格 + 发放记录 ──────────────────────────────
async function loadGrants(p: TrialPolicy) {
  grantsLoading[p.id] = true
  try {
    const res = await billingApi.trialGrants(p.id)
    grants[p.id] = res.data
  }
  catch {
    toast.error(t('billing.loadFail'))
  }
  finally {
    grantsLoading[p.id] = false
  }
}

async function doGrantEligibility(p: TrialPolicy) {
  const uid = Number(grantUserIds[p.id] ?? '')
  if (!uid || uid <= 0) {
    toast.error(t('billing.errUserIdRequired'))
    return
  }
  granting[p.id] = true
  try {
    await billingApi.grantEligibility(p.id, uid)
    toast.success(t('trial.grantOk'))
    grantUserIds[p.id] = ''
    loadGrants(p)
  }
  catch {
    toast.error(t('trial.grantFail'))
  }
  finally {
    granting[p.id] = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <Card>
      <CardHeader class="flex-row items-start justify-between gap-3 space-y-0">
        <div class="space-y-1.5">
          <CardTitle>{{ t('trial.title') }}</CardTitle>
          <CardDescription>{{ t('trial.desc') }}</CardDescription>
        </div>
        <Button v-auth="'billing:manage'" size="sm" @click="openCreate">
          <Plus class="size-4" /> {{ t('trial.add') }}
        </Button>
      </CardHeader>
      <CardContent>
        <DataTable
          v-model:search-value="q"
          :columns="columns"
          :data="policies"
          :loading="loading"
          :search-placeholder="t('trial.searchPlaceholder')"
          expandable
          @update:expanded="(row: TrialPolicy) => loadGrants(row)"
        >
          <template #cell-code="{ row }">
            <span class="font-mono text-xs">{{ row.code }}</span>
          </template>
          <template #cell-enabled="{ row }">
            <Badge :variant="row.enabled ? 'default' : 'outline'">
              {{ row.enabled ? t('table.enabled') : t('table.disabled') }}
            </Badge>
          </template>
          <template #cell-grant="{ row }">
            <span class="text-sm">{{ grantDesc(row) }}</span>
          </template>
          <template #cell-allow_new_user="{ row }">
            <Badge :variant="row.allow_new_user ? 'secondary' : 'outline'">
              {{ row.allow_new_user ? t('trial.yes') : t('trial.no') }}
            </Badge>
          </template>
          <template #cell-invite_code="{ row }">
            <span v-if="row.invite_code" class="font-mono text-xs">{{ row.invite_code }}</span>
            <span v-else class="text-muted-foreground">—</span>
          </template>
          <template #cell-actions="{ row }">
            <Button v-auth="'billing:manage'" variant="ghost" size="sm" @click="openEdit(row)">
              <Pencil class="size-4" />
            </Button>
            <Popconfirm :title="t('billing.deleteConfirm', { name: row.name })" @confirm="remove(row)">
              <Button v-auth="'billing:manage'" variant="ghost" size="sm" class="text-destructive hover:text-destructive">
                <Trash2 class="size-4" />
              </Button>
            </Popconfirm>
          </template>

          <!-- 展开行 -->
          <template #expanded="{ row }">
            <div class="space-y-4 p-4">
              <!-- 授予资格 -->
              <div class="space-y-2">
                <p class="text-sm font-medium">{{ t('trial.grantEligibilityTitle') }}</p>
                <div class="flex items-center gap-2">
                  <Input
                    v-model="grantUserIds[row.id]"
                    type="number"
                    class="h-8 w-36"
                    :placeholder="t('billing.inputUserId')"
                    @keyup.enter="doGrantEligibility(row)"
                  />
                  <Button
                    v-auth="'billing:manage'"
                    size="sm"
                    :disabled="granting[row.id]"
                    @click="doGrantEligibility(row)"
                  >
                    {{ granting[row.id] ? t('common.loading') : t('trial.grantBtn') }}
                  </Button>
                </div>
              </div>

              <!-- 发放记录 -->
              <div class="space-y-2">
                <div class="flex items-center gap-2">
                  <p class="text-sm font-medium">{{ t('trial.grantsTitle') }}</p>
                  <Button variant="ghost" size="sm" class="h-7 text-xs" :disabled="grantsLoading[row.id]" @click="loadGrants(row)">
                    {{ t('trial.grantsRefresh') }}
                  </Button>
                </div>
                <template v-if="grantsLoading[row.id]">
                  <p class="text-muted-foreground text-xs">{{ t('common.loading') }}</p>
                </template>
                <template v-else-if="grants[row.id]?.length">
                  <div class="overflow-auto rounded-md border">
                    <table class="w-full text-xs">
                      <thead>
                        <tr class="bg-muted/40 border-b">
                          <th class="px-3 py-2 text-left font-medium">{{ t('trial.colGrantUserId') }}</th>
                          <th class="px-3 py-2 text-left font-medium">{{ t('trial.colGrantSubject') }}</th>
                          <th class="px-3 py-2 text-left font-medium">{{ t('trial.colGrantQty') }}</th>
                          <th class="px-3 py-2 text-left font-medium">{{ t('table.createdAt') }}</th>
                        </tr>
                      </thead>
                      <tbody>
                        <tr v-for="g in grants[row.id]" :key="g.id" class="border-b last:border-0">
                          <td class="px-3 py-1.5 tabular-nums">{{ g.user_id }}</td>
                          <td class="px-3 py-1.5">
                            <Badge variant="outline" class="text-xs">{{ subjectLabel(g.subject) }}</Badge>
                          </td>
                          <td class="px-3 py-1.5 tabular-nums">{{ g.quantity }} {{ subjectUnit(g.subject) }}</td>
                          <td class="text-muted-foreground px-3 py-1.5 tabular-nums">{{ formatDateTime(g.created_at) }}</td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                </template>
                <p v-else class="text-muted-foreground text-xs">
                  {{ t('trial.grantsEmpty') }}
                </p>
              </div>
            </div>
          </template>
        </DataTable>
      </CardContent>
    </Card>

    <!-- 新增 / 编辑 Dialog -->
    <Dialog v-model:open="dialogOpen">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{{ editingId === null ? t('trial.createTitle') : t('trial.editTitle') }}</DialogTitle>
        </DialogHeader>
        <div class="flex flex-col gap-4 py-1">
          <!-- 代码（新建时填写，编辑时只读） -->
          <div class="flex flex-col gap-1.5">
            <Label>{{ t('trial.fCode') }}<span class="text-destructive ml-1">*</span></Label>
            <Input
              v-model="form.code"
              class="font-mono"
              :placeholder="t('trial.fCodePlaceholder')"
              :disabled="editingId !== null"
            />
          </div>
          <!-- 名称 -->
          <div class="flex flex-col gap-1.5">
            <Label>{{ t('trial.fName') }}<span class="text-destructive ml-1">*</span></Label>
            <Input v-model="form.name" :placeholder="t('trial.fNamePlaceholder')" />
          </div>
          <!-- 授予科目 + 授予数量 -->
          <div class="grid grid-cols-2 gap-3">
            <div class="flex flex-col gap-1.5">
              <Label>{{ t('trial.fGrantSubject') }}</Label>
              <Select
                :model-value="form.grant_subject"
                @update:model-value="(v) => { form.grant_subject = String(v) }"
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="s in SUBJECTS" :key="s" :value="s">
                    {{ subjectLabel(s) }}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="flex flex-col gap-1.5">
              <Label>{{ t('trial.fGrantQty') }}</Label>
              <Input v-model.number="form.grant_quantity" type="number" min="1" step="1" />
            </div>
          </div>
          <!-- 有效天数 + 每用户限量 -->
          <div class="grid grid-cols-2 gap-3">
            <div class="flex flex-col gap-1.5">
              <Label>{{ t('trial.fGrantExpireDays') }}</Label>
              <Input v-model.number="form.grant_expire_days" type="number" min="0" step="1" :placeholder="t('trial.fGrantExpireDaysHint')" />
              <p class="text-muted-foreground text-xs">{{ t('trial.fGrantExpireDaysHint') }}</p>
            </div>
            <div class="flex flex-col gap-1.5">
              <Label>{{ t('trial.fPerUserLimit') }}</Label>
              <Input v-model.number="form.per_user_limit" type="number" min="1" step="1" />
            </div>
          </div>
          <!-- 邀请码 -->
          <div class="flex flex-col gap-1.5">
            <Label>{{ t('trial.fInviteCode') }}</Label>
            <Input v-model="form.invite_code" class="font-mono" :placeholder="t('trial.fInviteCodePlaceholder')" />
          </div>
          <!-- 允许新用户 + 启用 -->
          <div class="grid grid-cols-2 gap-3">
            <div class="flex items-center justify-between rounded-md border px-3 py-2">
              <Label class="cursor-pointer">{{ t('trial.fAllowNewUser') }}</Label>
              <Switch v-model="form.allow_new_user" />
            </div>
            <div class="flex items-center justify-between rounded-md border px-3 py-2">
              <Label class="cursor-pointer">{{ t('trial.fEnabled') }}</Label>
              <Switch v-model="form.enabled" />
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="dialogOpen = false">{{ t('crud.cancel') }}</Button>
          <Button :disabled="saving" @click="save">{{ t('crud.confirm') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
