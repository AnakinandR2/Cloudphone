<script setup lang="ts">
import type { ProbeOutcome } from '@/types/proxy'
import { Activity, Loader2 } from 'lucide-vue-next'
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import proxyApi from '@/api/modules/proxy'
import { formatDateTime } from '@/utils/date'
import { Badge } from '@/components/ui/badge'
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

const props = defineProps<{ id: number, mode: 'create' | 'edit' | 'view' }>()
const open = defineModel<boolean>({ default: false })
const emit = defineEmits<{ success: [] }>()

const { t } = useI18n()
const readonly = computed(() => props.mode === 'view')
const isCreate = computed(() => props.mode === 'create')
const submitting = ref(false)
const nameError = ref('')
const hostError = ref('')
const portError = ref('')

// 即时测试（不落库）：测试中状态 + 结果
const probing = ref(false)
const probeResult = ref<ProbeOutcome | null>(null)

function emptyForm() {
  return {
    name: '',
    protocol: 'socks5',
    host: '',
    port: '' as number | string,
    username: '',
    password: '',
    region: '',
    remark: '',
    created_at: '',
    updated_at: '',
  }
}
const form = reactive(emptyForm())

const dialogTitle = computed(() =>
  props.mode === 'view'
    ? t('proxy.viewTitle')
    : isCreate.value
      ? t('proxy.createTitle')
      : t('proxy.editTitle'),
)

watch(open, async (v) => {
  if (v) {
    Object.assign(form, emptyForm())
    nameError.value = ''
    hostError.value = ''
    portError.value = ''
    probeResult.value = null
    if (props.id !== 0) {
      const res = await proxyApi.detail(props.id)
      Object.assign(form, res.data, { password: '' })
    }
  }
})

// 表单内即时测试：按当前填写的 host/port/用户名/密码探测，展示结果但不保存
async function doProbe() {
  hostError.value = ''
  portError.value = ''
  if (!String(form.host).trim()) {
    hostError.value = t('proxy.errHostRequired')
    return
  }
  if (!String(form.port).toString().trim() || Number(form.port) <= 0) {
    portError.value = t('proxy.errPortRequired')
    return
  }
  probing.value = true
  probeResult.value = null
  try {
    const res = await proxyApi.probe({
      host: String(form.host),
      port: Number(form.port),
      username: form.username,
      password: form.password,
    })
    probeResult.value = res.data
    toast[res.data.status === 'ok' ? 'success' : 'error'](
      res.data.status === 'ok' ? t('proxy.testOk') : t('proxy.testFail'),
    )
  }
  finally {
    probing.value = false
  }
}

function validate() {
  nameError.value = ''
  hostError.value = ''
  portError.value = ''
  let ok = true
  if (!form.name.trim()) {
    nameError.value = t('proxy.errNameRequired')
    ok = false
  }
  if (!String(form.host).trim()) {
    hostError.value = t('proxy.errHostRequired')
    ok = false
  }
  if (!String(form.port).toString().trim() || Number(form.port) <= 0) {
    portError.value = t('proxy.errPortRequired')
    ok = false
  }
  return ok
}

async function submit() {
  if (!validate())
    return
  submitting.value = true
  try {
    const payload = {
      name: form.name,
      protocol: form.protocol || 'socks5',
      host: String(form.host),
      port: Number(form.port),
      username: form.username,
      password: form.password,
      region: form.region,
      remark: form.remark,
    }
    if (isCreate.value) {
      await proxyApi.create(payload)
      toast.success(t('crud.createOk'))
    }
    else {
      await proxyApi.update(props.id, payload)
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
          <Label for="p-name">{{ t('proxy.fName') }}</Label>
          <Input id="p-name" v-model="form.name" :disabled="readonly" :placeholder="t('proxy.fNamePlaceholder')" />
          <p v-if="nameError" class="text-destructive text-xs">{{ nameError }}</p>
        </div>
        <div class="grid grid-cols-3 gap-3">
          <div class="space-y-2">
            <Label for="p-protocol">{{ t('proxy.fProtocol') }}</Label>
            <!-- 协议固定 socks5，不可编辑/选择 -->
            <Input id="p-protocol" :model-value="form.protocol || 'socks5'" disabled />
          </div>
          <div class="space-y-2 col-span-2">
            <Label for="p-host">{{ t('proxy.fHost') }}</Label>
            <Input id="p-host" v-model="form.host" :disabled="readonly" :placeholder="t('proxy.fHostPlaceholder')" />
            <p v-if="hostError" class="text-destructive text-xs">{{ hostError }}</p>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div class="space-y-2">
            <Label for="p-port">{{ t('proxy.fPort') }}</Label>
            <Input id="p-port" v-model="form.port" type="number" :disabled="readonly" :placeholder="t('proxy.fPortPlaceholder')" />
            <p v-if="portError" class="text-destructive text-xs">{{ portError }}</p>
          </div>
          <div class="space-y-2">
            <Label for="p-region">{{ t('proxy.fRegion') }}</Label>
            <Input id="p-region" v-model="form.region" :disabled="readonly" :placeholder="t('proxy.fRegionPlaceholder')" />
          </div>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div class="space-y-2">
            <Label for="p-username">{{ t('proxy.fUsername') }}</Label>
            <Input id="p-username" v-model="form.username" :disabled="readonly" :placeholder="t('proxy.fUsernamePlaceholder')" />
          </div>
          <div v-if="!readonly" class="space-y-2">
            <Label for="p-password">{{ t('proxy.fPassword') }}</Label>
            <Input id="p-password" v-model="form.password" type="password" :placeholder="isCreate ? t('proxy.fPasswordPlaceholder') : t('proxy.fPasswordEdit')" />
          </div>
        </div>
        <!-- 即时测试：按当前填写探测连通性并展示出口信息（不保存） -->
        <div v-if="!readonly" class="space-y-2 rounded-md border p-3">
          <div class="flex items-center justify-between">
            <Label class="m-0">{{ t('proxy.test') }}</Label>
            <Button variant="outline" size="sm" :disabled="probing" @click="doProbe">
              <Loader2 v-if="probing" class="size-4 animate-spin" />
              <Activity v-else class="size-4" />
              {{ t('proxy.test') }}
            </Button>
          </div>
          <div v-if="probeResult" class="text-xs">
            <div class="mb-2 flex items-center gap-2">
              <Badge :variant="probeResult.status === 'ok' ? 'default' : 'destructive'">
                {{ probeResult.status === 'ok' ? t('proxy.status_ok') : t('proxy.status_fail') }}
              </Badge>
              <span v-if="probeResult.status === 'ok'" class="text-muted-foreground tabular-nums">{{ probeResult.latency }} ms</span>
              <span v-else class="text-destructive">{{ probeResult.message }}</span>
            </div>
            <div v-if="probeResult.status === 'ok'" class="text-muted-foreground grid grid-cols-2 gap-x-6 gap-y-1">
              <div>
                <span class="text-foreground/70">{{ t('proxy.colEgressIp') }}：</span>
                <span class="font-mono">{{ probeResult.egress_ip || '-' }}</span>
                <Badge v-if="probeResult.country" variant="secondary" class="ml-1.5">{{ probeResult.country }}</Badge>
              </div>
              <div><span class="text-foreground/70">{{ t('proxy.colRegion') }}：</span>{{ [probeResult.country, probeResult.city].filter(Boolean).join(' ') || '-' }}</div>
              <div><span class="text-foreground/70">{{ t('proxy.colAsn') }}：</span>{{ probeResult.asn ? `${probeResult.asn}${probeResult.asn_name ? ` ${probeResult.asn_name}` : ''}` : '-' }}</div>
              <div><span class="text-foreground/70">{{ t('proxy.colCompany') }}：</span>{{ probeResult.company || '-' }}</div>
            </div>
          </div>
        </div>

        <div class="space-y-2">
          <Label for="p-remark">{{ t('proxy.fRemark') }}</Label>
          <Input id="p-remark" v-model="form.remark" :disabled="readonly" :placeholder="t('proxy.fRemarkPlaceholder')" />
        </div>
        <div v-if="!isCreate" class="text-muted-foreground grid grid-cols-2 gap-2 text-xs">
          <span>{{ t('table.createdAt') }}: {{ formatDateTime(form.created_at) }}</span>
          <span>{{ t('proxy.updatedAt') }}: {{ formatDateTime(form.updated_at) }}</span>
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
