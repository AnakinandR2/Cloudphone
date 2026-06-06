<script setup lang="ts">
import type { RoleListItem } from '@/types/role'
import type { StaffCreate, StaffUpdate } from '@/types/staff'
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { toast } from 'vue-sonner'
import roleApi from '@/api/modules/role'
import staffApi from '@/api/modules/staff'
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
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Switch } from '@/components/ui/switch'
import { formatDateTime } from '@/utils/date'

const props = defineProps<{ id: number, mode: 'create' | 'edit' | 'view' }>()
const emit = defineEmits<{ success: [] }>()
const open = defineModel<boolean>({ default: false })
const { t } = useI18n()
const readonly = computed(() => props.mode === 'view')
const isCreate = computed(() => props.mode === 'create')

const loading = ref(false)
const submitting = ref(false)
const roleList = ref<RoleListItem[]>([])

const form = reactive({
  username: '',
  name: '',
  password: '',
  is_active: true,
  is_superuser: false,
  avatar: '',
  role_ids: [] as number[],
  created_at: '',
  updated_at: '',
})

const errors = reactive({ username: '', password: '' })

// RadioGroup 仅接受字符串值，这里用字符串代理布尔 is_active
const activeStr = computed({
  get: () => (form.is_active ? '1' : '0'),
  set: (v: string) => {
    form.is_active = v === '1'
  },
})

const title = computed(() =>
  props.mode === 'view'
    ? t('staff.viewTitle')
    : isCreate.value
      ? t('staff.createTitle')
      : t('staff.editTitle'),
)

function reset() {
  Object.assign(form, {
    username: '',
    name: '',
    password: '',
    is_active: true,
    is_superuser: false,
    avatar: '',
    role_ids: [],
    created_at: '',
    updated_at: '',
  })
  errors.username = ''
  errors.password = ''
}

async function loadRoles() {
  try {
    const res = await roleApi.all()
    roleList.value = res.data.list
  }
  catch {
    /* 无角色权限时忽略 */
  }
}

async function loadDetail() {
  loading.value = true
  try {
    const res = await staffApi.detail(props.id)
    const d = res.data
    Object.assign(form, {
      username: d.username,
      name: d.name,
      password: '',
      is_active: d.is_active,
      is_superuser: d.is_superuser,
      avatar: d.avatar,
      role_ids: d.roles.map(r => r.id),
      created_at: d.created_at,
      updated_at: d.updated_at,
    })
  }
  finally {
    loading.value = false
  }
}

watch(open, async (v) => {
  if (v) {
    reset()
    await loadRoles()
    if (props.id !== 0) await loadDetail()
  }
})

function toggleRole(id: number, checked: boolean) {
  if (checked) {
    if (!form.role_ids.includes(id)) form.role_ids.push(id)
  }
  else {
    form.role_ids = form.role_ids.filter(x => x !== id)
  }
}

function validate() {
  errors.username = ''
  errors.password = ''
  if (!form.username.trim()) errors.username = t('staff.errUsernameRequired')
  else if (form.username.length < 3 || form.username.length > 50)
    errors.username = t('staff.errUsernameLen')
  if (isCreate.value && !form.password) errors.password = t('staff.errPasswordRequired')
  else if (form.password && form.password.length < 6)
    errors.password = t('staff.errPasswordLen')
  return !errors.username && !errors.password
}

async function submit() {
  if (!validate()) return
  submitting.value = true
  try {
    if (isCreate.value) {
      const payload: StaffCreate = {
        username: form.username,
        name: form.name.trim(),
        password: form.password,
        is_active: form.is_active,
        is_superuser: form.is_superuser,
        avatar: form.avatar,
        role_ids: form.role_ids,
      }
      await staffApi.create(payload)
      toast.success(t('crud.createOk'))
    }
    else {
      const payload: StaffUpdate = {
        username: form.username,
        name: form.name.trim(),
        is_active: form.is_active,
        is_superuser: form.is_superuser,
        avatar: form.avatar,
        role_ids: form.role_ids,
      }
      if (form.password) payload.password = form.password
      await staffApi.update(props.id, payload)
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
    <DialogContent class="max-h-[85vh] overflow-y-auto sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ title }}</DialogTitle>
        <DialogDescription class="sr-only">
          {{ title }}
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-2">
        <div class="space-y-2">
          <Label for="u-username">{{ t('table.username') }}</Label>
          <Input id="u-username" v-model="form.username" :disabled="readonly" :placeholder="t('table.username')" />
          <p v-if="errors.username" class="text-destructive text-xs">
            {{ errors.username }}
          </p>
        </div>

        <div class="space-y-2">
          <Label for="u-name">{{ t('staff.fName') }}</Label>
          <Input id="u-name" v-model="form.name" :disabled="readonly" :placeholder="t('staff.fNamePlaceholder')" />
        </div>

        <div v-if="!readonly" class="space-y-2">
          <Label for="u-password">{{ t('staff.fPassword') }}</Label>
          <Input
            id="u-password"
            v-model="form.password"
            type="password"
            :placeholder="isCreate ? t('staff.fPasswordCreate') : t('staff.fPasswordEdit')"
          />
          <p v-if="errors.password" class="text-destructive text-xs">
            {{ errors.password }}
          </p>
        </div>

        <div class="space-y-2">
          <Label>{{ t('table.status') }}</Label>
          <RadioGroup v-model="activeStr" :disabled="readonly" class="flex gap-6">
            <div class="flex items-center gap-2">
              <RadioGroupItem id="u-active-1" value="1" />
              <Label for="u-active-1" class="font-normal">{{ t('table.enabled') }}</Label>
            </div>
            <div class="flex items-center gap-2">
              <RadioGroupItem id="u-active-0" value="0" />
              <Label for="u-active-0" class="font-normal">{{ t('table.disabled') }}</Label>
            </div>
          </RadioGroup>
        </div>

        <div class="space-y-1.5">
          <div class="flex items-center gap-3">
            <Switch v-model="form.is_superuser" :disabled="readonly" />
            <Label class="font-normal">{{ t('staff.fSuperuser') }}</Label>
          </div>
          <p class="text-muted-foreground text-xs">
            {{ t('staff.fSuperuserHint') }}
          </p>
        </div>

        <div v-if="!form.is_superuser" class="space-y-2">
          <Label>{{ t('staff.fRoles') }}</Label>
          <div class="grid grid-cols-2 gap-2 rounded-md border p-3">
            <div v-for="role in roleList" :key="role.id" class="flex items-center gap-2">
              <Checkbox
                :id="`u-role-${role.id}`"
                :model-value="form.role_ids.includes(role.id)"
                :disabled="readonly"
                @update:model-value="(v) => toggleRole(role.id, v === true)"
              />
              <Label :for="`u-role-${role.id}`" class="font-normal">{{ role.name }}</Label>
            </div>
            <p v-if="!roleList.length" class="text-muted-foreground col-span-2 text-xs">
              {{ t('staff.noRoles') }}
            </p>
          </div>
          <p class="text-muted-foreground text-xs">
            {{ t('staff.fRolesHint') }}
          </p>
        </div>

        <div class="space-y-2">
          <Label for="u-avatar">{{ t('staff.fAvatar') }}</Label>
          <Input id="u-avatar" v-model="form.avatar" :disabled="readonly" :placeholder="t('staff.fAvatarPlaceholder')" />
        </div>

        <div v-if="!isCreate" class="text-muted-foreground grid grid-cols-2 gap-2 text-xs">
          <span>{{ t('table.createdAt') }}: {{ formatDateTime(form.created_at) }}</span>
          <span>{{ t('staff.colUpdatedAt') }}: {{ formatDateTime(form.updated_at) }}</span>
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
