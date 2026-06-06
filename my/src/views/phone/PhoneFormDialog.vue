<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import phoneApi from '@/api/modules/phone'
import proxyApi from '@/api/modules/proxy'
import type { Proxy } from '@/types/proxy'
import { formatDateTime } from '@/utils/date'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

const props = defineProps<{ id: number, mode: 'create' | 'edit' | 'view' }>()
const open = defineModel<boolean>({ default: false })
const emit = defineEmits<{ success: [] }>()

const { t } = useI18n()
const readonly = computed(() => props.mode === 'view')
const isCreate = computed(() => props.mode === 'create')
const submitting = ref(false)
const nameError = ref('')

// 可绑定的代理（不分页，来自后端 /proxy/options）
const proxyOptions = ref<Proxy[]>([])

function emptyForm() {
  return {
    name: '',
    status: 'CREATED',
    // 地区/镜像暂不开放选择，创建时留空透传
    region: '',
    image_id: '',
    proxy_id: 0 as number,
    remark: '',
    created_at: '',
    updated_at: '',
  }
}
const form = reactive(emptyForm())

const dialogTitle = computed(() =>
  props.mode === 'view'
    ? t('phone.viewTitle')
    : isCreate.value
      ? t('phone.createTitle')
      : t('phone.editTitle'),
)

watch(open, async (v) => {
  if (v) {
    Object.assign(form, emptyForm())
    nameError.value = ''
    // 拉取可绑定代理供下拉选择（查看态也加载以便回显名称）
    try {
      const opt = await proxyApi.options()
      proxyOptions.value = opt.data
    }
    catch {
      proxyOptions.value = []
    }
    if (props.id !== 0) {
      const res = await phoneApi.detail(props.id)
      Object.assign(form, res.data)
    }
  }
})

async function submit() {
  nameError.value = ''
  if (!form.name.trim()) {
    nameError.value = t('phone.errNameRequired')
    return
  }
  submitting.value = true
  try {
    const proxyId = Number(form.proxy_id) || 0
    if (isCreate.value) {
      await phoneApi.create({
        name: form.name,
        region: form.region,
        image_id: form.image_id,
        proxy_id: proxyId,
        remark: form.remark,
      })
      toast.success(t('crud.createOk'))
    }
    else {
      await phoneApi.update(props.id, {
        name: form.name,
        region: form.region,
        image_id: form.image_id,
        proxy_id: proxyId,
        remark: form.remark,
      })
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
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ dialogTitle }}</DialogTitle>
        <DialogDescription class="sr-only">{{ dialogTitle }}</DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-2">
        <div class="space-y-2">
          <Label for="ph-name">{{ t('phone.fName') }}</Label>
          <Input id="ph-name" v-model="form.name" :disabled="readonly" :placeholder="t('phone.fNamePlaceholder')" />
          <p v-if="nameError" class="text-destructive text-xs">{{ nameError }}</p>
        </div>
        <div v-if="!isCreate" class="space-y-2">
          <Label>{{ t('phone.fStatus') }}</Label>
          <p class="text-muted-foreground text-sm">{{ t(`phone.status_${form.status}`, form.status) }}</p>
        </div>
        <div class="space-y-2">
          <Label for="ph-proxy">{{ t('phone.fProxyId') }}</Label>
          <NativeSelect id="ph-proxy" v-model.number="form.proxy_id" class="w-full" :disabled="readonly">
            <NativeSelectOption :value="0">
              {{ t('phone.proxyNone', '未绑定') }}
            </NativeSelectOption>
            <NativeSelectOption v-for="p in proxyOptions" :key="p.id" :value="p.id">
              {{ p.name }}（{{ p.protocol }}://{{ p.host }}:{{ p.port }}）
            </NativeSelectOption>
          </NativeSelect>
          <p class="text-muted-foreground text-xs">{{ t('phone.fProxyIdHint') }}</p>
        </div>
        <div class="space-y-2">
          <Label for="ph-remark">{{ t('phone.fRemark') }}</Label>
          <Input id="ph-remark" v-model="form.remark" :disabled="readonly" :placeholder="t('phone.fRemarkPlaceholder')" />
        </div>
        <div v-if="!isCreate" class="text-muted-foreground grid grid-cols-2 gap-2 text-xs">
          <span>{{ t('table.createdAt') }}: {{ formatDateTime(form.created_at) }}</span>
          <span>{{ t('phone.updatedAt') }}: {{ formatDateTime(form.updated_at) }}</span>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="open = false">
          {{ readonly ? t('crud.close') : t('crud.cancel') }}
        </Button>
        <Button v-if="!readonly" :disabled="submitting" @click="submit">
          {{ t('crud.confirm') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
