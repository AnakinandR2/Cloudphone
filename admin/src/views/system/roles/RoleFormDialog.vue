<script setup lang="ts">
import type { PermissionGroup } from '@/types/role'
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import roleApi from '@/api/modules/role'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { formatDateTime } from '@/utils/date'

const props = defineProps<{ id: number, mode: 'create' | 'edit' | 'view' }>()
const emit = defineEmits<{ success: [] }>()
const open = defineModel<boolean>({ default: false })
const { t } = useI18n()
const readonly = computed(() => props.mode === 'view')
const isCreate = computed(() => props.mode === 'create')

const loading = ref(false)
const submitting = ref(false)
const groups = ref<PermissionGroup[]>([])
const selected = ref<Set<string>>(new Set())
const form = reactive({ name: '', description: '', created_at: '', updated_at: '' })
const nameError = ref('')

const allLeafKeys = computed(() => groups.value.flatMap(g => g.permissions.map(p => p.key)))
const allChecked = computed(
  () => allLeafKeys.value.length > 0 && allLeafKeys.value.every(k => selected.value.has(k)),
)

/** 全局全选框的三态：true 全选 / 'indeterminate' 部分 / false 未选 */
const allModel = computed<boolean | 'indeterminate'>(() => {
  const total = allLeafKeys.value.length
  const count = allLeafKeys.value.filter(k => selected.value.has(k)).length
  if (count === 0) return false
  return count === total ? true : 'indeterminate'
})

// —— 分组（模块）级联：父级勾选控制本组全部子权限，支持半选态 ——
function groupKeys(g: PermissionGroup) {
  return g.permissions.map(p => p.key)
}
function groupSelectedCount(g: PermissionGroup) {
  return groupKeys(g).filter(k => selected.value.has(k)).length
}
function groupModel(g: PermissionGroup): boolean | 'indeterminate' {
  const count = groupSelectedCount(g)
  if (count === 0) return false
  return count === g.permissions.length ? true : 'indeterminate'
}
function toggleGroup(g: PermissionGroup) {
  const keys = groupKeys(g)
  const next = new Set(selected.value)
  if (keys.every(k => next.has(k))) {
    keys.forEach(k => next.delete(k))
  }
  else {
    keys.forEach(k => next.add(k))
  }
  selected.value = next
}

const title = computed(() =>
  props.mode === 'view'
    ? t('roles.viewTitle')
    : isCreate.value
      ? t('roles.createTitle')
      : t('roles.editTitle'),
)

async function loadPermissions() {
  const res = await roleApi.permissions()
  groups.value = res.data
}

async function loadDetail() {
  loading.value = true
  try {
    const res = await roleApi.detail(props.id)
    const d = res.data
    form.name = d.name
    form.description = d.description
    form.created_at = d.created_at
    form.updated_at = d.updated_at
    selected.value = d.permissions.includes('*')
      ? new Set(allLeafKeys.value)
      : new Set(d.permissions)
  }
  finally {
    loading.value = false
  }
}

watch(open, async (v) => {
  if (v) {
    form.name = ''
    form.description = ''
    form.created_at = ''
    form.updated_at = ''
    selected.value = new Set()
    nameError.value = ''
    await loadPermissions()
    if (props.id !== 0) await loadDetail()
  }
})

function toggle(key: string, checked: boolean) {
  const next = new Set(selected.value)
  if (checked) next.add(key)
  else next.delete(key)
  selected.value = next
}

function toggleAll() {
  selected.value = allChecked.value ? new Set() : new Set(allLeafKeys.value)
}

async function submit() {
  nameError.value = ''
  if (!form.name.trim()) {
    nameError.value = t('roles.errNameRequired')
    return
  }
  if (form.name.length < 2 || form.name.length > 50) {
    nameError.value = t('roles.errNameLen')
    return
  }
  submitting.value = true
  try {
    const payload = {
      name: form.name,
      description: form.description,
      permissions: [...selected.value],
    }
    if (isCreate.value) {
      await roleApi.create(payload)
      toast.success(t('crud.createOk'))
    }
    else {
      await roleApi.update(props.id, payload)
      toast.success(t('crud.updateOk'))
    }
    open.value = false
    emit('success')
  }
  finally {
    submitting.value = false
  }
}
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="max-h-[85vh] overflow-y-auto sm:max-w-xl">
      <DialogHeader>
        <DialogTitle>{{ title }}</DialogTitle>
        <DialogDescription class="sr-only">
          {{ title }}
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-2">
        <div class="space-y-2">
          <Label for="r-name">{{ t('roles.fName') }}</Label>
          <Input id="r-name" v-model="form.name" :disabled="readonly" :placeholder="t('roles.fNamePlaceholder')" />
          <p v-if="nameError" class="text-destructive text-xs">
            {{ nameError }}
          </p>
        </div>

        <div class="space-y-2">
          <Label for="r-desc">{{ t('roles.fDesc') }}</Label>
          <Textarea id="r-desc" v-model="form.description" :disabled="readonly" :rows="2" :placeholder="t('roles.fDescPlaceholder')" />
        </div>

        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <Label>{{ t('roles.fPermissions') }}</Label>
            <label v-if="!readonly" class="flex cursor-pointer items-center gap-2 select-none">
              <Checkbox :model-value="allModel" @update:model-value="toggleAll" />
              <span class="text-muted-foreground text-sm">{{ t('roles.checkAll') }}</span>
            </label>
          </div>
          <div class="max-h-72 space-y-3 overflow-y-auto rounded-md border p-3">
            <div v-for="g in groups" :key="g.module_key" class="space-y-1.5">
              <!-- 模块父级：级联控制本组全部权限，支持半选 -->
              <label
                class="flex items-center gap-2 select-none"
                :class="readonly ? '' : 'cursor-pointer'"
              >
                <Checkbox
                  :model-value="groupModel(g)"
                  :disabled="readonly"
                  @update:model-value="() => toggleGroup(g)"
                />
                <span class="text-sm font-medium">{{ g.module }}</span>
                <span class="text-muted-foreground text-xs tabular-nums">
                  {{ groupSelectedCount(g) }}/{{ g.permissions.length }}
                </span>
              </label>
              <!-- 子权限 -->
              <div class="grid grid-cols-2 gap-2 pl-6 sm:grid-cols-3">
                <label
                  v-for="p in g.permissions"
                  :key="p.key"
                  class="flex items-center gap-2 select-none"
                  :class="readonly ? '' : 'cursor-pointer'"
                  :title="p.key"
                >
                  <Checkbox
                    :model-value="selected.has(p.key)"
                    :disabled="readonly"
                    @update:model-value="(v) => toggle(p.key, v === true)"
                  />
                  <span class="text-muted-foreground text-sm font-normal">{{ p.label }}</span>
                </label>
              </div>
            </div>
          </div>
        </div>

        <div v-if="!isCreate" class="text-muted-foreground grid grid-cols-2 gap-2 text-xs">
          <span>{{ t('table.createdAt') }}: {{ formatDateTime(form.created_at) }}</span>
          <span>{{ t('users.colUpdatedAt') }}: {{ formatDateTime(form.updated_at) }}</span>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="open = false">
          {{ readonly ? t('crud.close') : t('crud.cancel') }}
        </Button>
        <Button v-if="!readonly" :disabled="submitting || loading" @click="submit">
          {{ t('crud.confirm') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
