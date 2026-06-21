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

// ── 营销展示单选（后端清其它，全局至多一条）───────────────────
const featuring = reactive<Record<number, boolean>>({})
async function toggleFeature(p: TrialPolicy, val: boolean) {
  featuring[p.id] = true
  try {
    await billingApi.featureTrial(p.id, val)
    toast.success(t('trial.featureOk'))
    await load() // 重拉：单选互斥后其它行同步关闭
  }
  catch {
    toast.error(t('trial.featureFail'))
  }
  finally {
    featuring[p.id] = false
  }
}

// 三类资源科目（表单固定三行；数量 0 = 不发该项）。
// 统一顺序：实例席位 → 包月开机数 → 临时开机时长。
const ITEM_SUBJECTS = ['seat', 'boot_slot', 'runtime_minute'] as const

function subjectLabel(s: string): string {
  return t(`billing.subject_${s}`)
}

function subjectUnit(s: string): string {
  if (s === 'runtime_minute') return t('billing.unitMinute')
  if (s === 'boot_slot') return t('billing.unitSlot')
  return t('billing.unitSeat')
}

// 按统一顺序（实例席位→包月开机数→临时开机时长）排序发放项。
function orderedItems<T extends { subject: string }>(items: T[]): T[] {
  const rank = (s: string) => {
    const i = (ITEM_SUBJECTS as readonly string[]).indexOf(s)
    return i < 0 ? 99 : i
  }
  return [...items].sort((a, b) => rank(a.subject) - rank(b.subject))
}

function grantDesc(p: TrialPolicy): string {
  if (!p.items?.length) {
    return '—'
  }
  return orderedItems(p.items).map((it) => {
    const base = `${subjectLabel(it.subject)} × ${it.quantity} ${subjectUnit(it.subject)}`
    const exp = it.expire_days > 0 ? t('trial.nDaysValid', { n: it.expire_days }) : t('trial.permanent')
    return `${base}（${exp}）`
  }).join('　')
}

const columns = computed<ColumnDef<TrialPolicy>[]>(() => [
  { id: 'expander', header: '', enableHiding: false, meta: { label: '' } },
  { accessorKey: 'code', id: 'code', header: t('trial.fCode'), meta: { label: 'trial.fCode' } },
  { accessorKey: 'name', id: 'name', header: t('trial.fName'), meta: { label: 'trial.fName' } },
  { accessorKey: 'enabled', id: 'enabled', header: t('trial.fEnabled'), meta: { label: 'trial.fEnabled' } },
  { accessorKey: 'marketing_featured', id: 'marketing_featured', header: t('trial.fMarketing'), meta: { label: 'trial.fMarketing' } },
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

interface ItemRow { quantity: number, expire_days: number }
interface FormState {
  code: string
  name: string
  items: Record<string, ItemRow>
  per_user_limit: number
  allow_new_user: boolean
  invite_code: string
  enabled: boolean
}

function emptyItems(): Record<string, ItemRow> {
  return {
    seat: { quantity: 0, expire_days: 0 },
    boot_slot: { quantity: 0, expire_days: 0 },
    runtime_minute: { quantity: 0, expire_days: 0 },
  }
}

const form = reactive<FormState>({
  code: '',
  name: '',
  items: emptyItems(),
  per_user_limit: 1,
  allow_new_user: false,
  invite_code: '',
  enabled: true,
})

function resetMeta() {
  form.per_user_limit = 1
  form.allow_new_user = false
  form.invite_code = ''
  form.enabled = true
}

function openCreate() {
  editingId.value = null
  form.code = ''
  form.name = ''
  form.items = emptyItems()
  resetMeta()
  dialogOpen.value = true
}

function openEdit(p: TrialPolicy) {
  editingId.value = p.id
  form.code = p.code
  form.name = p.name
  form.items = emptyItems()
  for (const it of p.items ?? []) {
    if (form.items[it.subject]) {
      form.items[it.subject] = { quantity: it.quantity, expire_days: it.expire_days }
    }
  }
  form.per_user_limit = p.per_user_limit
  form.allow_new_user = p.allow_new_user
  form.invite_code = p.invite_code ?? ''
  form.enabled = p.enabled
  dialogOpen.value = true
}

// 收集数量>0 的发放项。数量/有效天数留空（''）一律按 0 处理（有效天数 0 = 永久）。
function buildItems() {
  return ITEM_SUBJECTS
    .map(s => ({
      subject: s,
      quantity: Number(form.items[s].quantity) || 0,
      expire_days: Number(form.items[s].expire_days) || 0,
    }))
    .filter(it => it.quantity > 0)
}

async function save() {
  const items = buildItems()
  if (!items.length) {
    toast.error(t('trial.errNoItem'))
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
        items,
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
        items,
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
          <template #cell-marketing_featured="{ row }">
            <Switch
              :model-value="row.marketing_featured"
              :disabled="featuring[row.id]"
              :title="t('trial.fMarketingHint')"
              @update:model-value="(v) => toggleFeature(row, v === true)"
            />
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
                <p class="text-sm font-medium">
                  {{ t('trial.grantEligibilityTitle') }}
                </p>
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
                  <p class="text-sm font-medium">
                    {{ t('trial.grantsTitle') }}
                  </p>
                  <Button variant="ghost" size="sm" class="h-7 text-xs" :disabled="grantsLoading[row.id]" @click="loadGrants(row)">
                    {{ t('trial.grantsRefresh') }}
                  </Button>
                </div>
                <template v-if="grantsLoading[row.id]">
                  <p class="text-muted-foreground text-xs">
                    {{ t('common.loading') }}
                  </p>
                </template>
                <template v-else-if="grants[row.id]?.length">
                  <div class="overflow-auto rounded-md border">
                    <table class="w-full text-xs">
                      <thead>
                        <tr class="bg-muted/40 border-b">
                          <th class="px-3 py-2 text-left font-medium">
                            {{ t('trial.colGrantUserId') }}
                          </th>
                          <th class="px-3 py-2 text-left font-medium">
                            {{ t('trial.colGrantSubject') }}
                          </th>
                          <th class="px-3 py-2 text-left font-medium">
                            {{ t('trial.colGrantQty') }}
                          </th>
                          <th class="px-3 py-2 text-left font-medium">
                            {{ t('table.createdAt') }}
                          </th>
                        </tr>
                      </thead>
                      <tbody>
                        <tr v-for="g in grants[row.id]" :key="g.id" class="border-b last:border-0">
                          <td class="px-3 py-1.5 tabular-nums">
                            {{ g.user_id }}
                          </td>
                          <td class="px-3 py-1.5">
                            <Badge variant="outline" class="text-xs">
                              {{ subjectLabel(g.subject) }}
                            </Badge>
                          </td>
                          <td class="px-3 py-1.5 tabular-nums">
                            {{ g.quantity }} {{ subjectUnit(g.subject) }}
                          </td>
                          <td class="text-muted-foreground px-3 py-1.5 tabular-nums">
                            {{ formatDateTime(g.created_at) }}
                          </td>
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
          <!-- 发放项（三类资源，数量 0 = 不发该项；有效天数 0 = 永久） -->
          <div class="flex flex-col gap-1.5">
            <Label>{{ t('trial.fGrantItems') }}</Label>
            <div class="divide-y rounded-md border">
              <div v-for="s in ITEM_SUBJECTS" :key="s" class="grid grid-cols-[1fr_auto_auto] items-center gap-3 px-3 py-2">
                <span class="text-sm">{{ subjectLabel(s) }}</span>
                <div class="flex items-center gap-1.5">
                  <span class="text-muted-foreground text-xs">{{ t('trial.fGrantQty') }}</span>
                  <Input v-model.number="form.items[s].quantity" type="number" min="0" step="1" class="h-8 w-20" />
                  <span class="text-muted-foreground text-xs">{{ subjectUnit(s) }}</span>
                </div>
                <div class="flex items-center gap-1.5">
                  <span class="text-muted-foreground text-xs">{{ t('trial.fGrantExpireDays') }}</span>
                  <Input v-model.number="form.items[s].expire_days" type="number" min="0" step="1" class="h-8 w-20" />
                </div>
              </div>
            </div>
            <p class="text-muted-foreground text-xs">
              {{ t('trial.fGrantItemsHint') }}
            </p>
          </div>
          <!-- 每用户限量 -->
          <div class="flex flex-col gap-1.5">
            <Label>{{ t('trial.fPerUserLimit') }}</Label>
            <Input v-model.number="form.per_user_limit" type="number" min="1" step="1" />
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
          <Button variant="outline" @click="dialogOpen = false">
            {{ t('crud.cancel') }}
          </Button>
          <Button :disabled="saving" @click="save">
            {{ t('crud.confirm') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
