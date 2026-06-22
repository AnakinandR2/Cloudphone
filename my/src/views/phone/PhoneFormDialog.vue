<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { toast } from 'vue-sonner'

import phoneApi from '@/api/modules/phone'
import proxyApi from '@/api/modules/proxy'
import type { CloudPhone } from '@/types/phone'
import type { Proxy } from '@/types/proxy'
import { formatDateTime } from '@/utils/date'
import { countBoundProxies, formatProxyAddress, formatProxyIp } from './proxyBind'
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
import { Switch } from '@/components/ui/switch'

const props = withDefaults(
  defineProps<{ id: number, mode: 'create' | 'edit' | 'view', phones?: CloudPhone[] }>(),
  { phones: () => [] },
)
const open = defineModel<boolean>({ default: false })
const emit = defineEmits<{ success: [] }>()

const { t } = useI18n()
const readonly = computed(() => props.mode === 'view')
const isCreate = computed(() => props.mode === 'create')
const submitting = ref(false)
const nameError = ref('')

// 可选代理（不分页，含未测试/失败态，便于在表格里完整展示与选择）。
const proxies = ref<Proxy[]>([])
// 绑定代理开关：打开才显示代理表格，关闭则提示「必须绑定代理才能开机」（proxy_id=0）。
const bindProxy = ref(false)
// 每个代理「已绑定数量」：从本人云手机列表聚合（父组件传入），无需额外请求。
const boundCounts = computed(() => countBoundProxies(props.phones))

function emptyForm() {
  return {
    name: '',
    status: 'CREATED',
    // 镜像暂不开放选择，创建时留空透传
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

// 选中代理（行点击）。view 态只读，不可改。
function pickProxy(id: number) {
  if (readonly.value)
    return
  form.proxy_id = id
}

// 切换「绑定代理」开关：关闭即清空绑定；打开且未选时默认选中第一个可选代理。
function onToggleBind(on: boolean) {
  if (readonly.value)
    return
  bindProxy.value = on
  if (!on) {
    form.proxy_id = 0
  }
  else if (form.proxy_id === 0 && proxies.value.length) {
    form.proxy_id = proxies.value[0].id
  }
}

watch(open, async (v) => {
  if (v) {
    Object.assign(form, emptyForm())
    nameError.value = ''
    // 拉取全部代理供表格选择（查看态也加载以便回显）。
    try {
      const res = await proxyApi.list({ page: 1, size: 999 })
      proxies.value = res.data.list
    }
    catch {
      proxies.value = []
    }
    if (props.id !== 0) {
      const res = await phoneApi.detail(props.id)
      Object.assign(form, res.data)
    }
    // 已绑代理则默认打开开关并回显选中；未绑则关闭并提示。
    bindProxy.value = form.proxy_id > 0
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
    // 开关关闭 = 不绑定（proxy_id=0）；打开则以表格选中为准。
    const proxyId = bindProxy.value ? (Number(form.proxy_id) || 0) : 0
    if (isCreate.value) {
      await phoneApi.create({
        name: form.name,
        image_id: form.image_id,
        proxy_id: proxyId,
        remark: form.remark,
      })
      toast.success(t('crud.createOk'))
    }
    else {
      await phoneApi.update(props.id, {
        name: form.name,
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
    <DialogContent class="sm:max-w-xl">
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

        <!-- 绑定代理：开关打开才显示代理表格；关闭则提示「必须绑定代理才能开机」 -->
        <div class="space-y-2">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2">
              <Switch
                id="ph-bind"
                :model-value="bindProxy"
                :disabled="readonly"
                @update:model-value="(v) => onToggleBind(v === true)"
              />
              <Label for="ph-bind">{{ t('phone.fProxy') }}</Label>
            </div>
            <RouterLink v-if="bindProxy" to="/proxy" class="text-primary text-xs hover:underline" @click="open = false">
              {{ t('phone.proxyGotoManage') }} →
            </RouterLink>
          </div>

          <!-- 开：代理表格（代理 ID / 地址 / IP 信息 / 已绑定数量） -->
          <template v-if="bindProxy">
            <div class="max-h-56 overflow-y-auto rounded-md border">
              <table class="w-full text-xs">
                <thead class="bg-muted/50 text-muted-foreground sticky top-0">
                  <tr>
                    <th class="w-8 p-2" />
                    <th class="p-2 text-left font-medium">{{ t('phone.proxyColId') }}</th>
                    <th class="p-2 text-left font-medium">{{ t('phone.proxyColAddress') }}</th>
                    <th class="p-2 text-left font-medium">{{ t('phone.proxyColIp') }}</th>
                    <th class="p-2 text-right font-medium">{{ t('phone.proxyColBound') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="p in proxies" :key="p.id" class="hover:bg-muted/40 cursor-pointer border-t" @click="pickProxy(p.id)">
                    <td class="p-2 text-center">
                      <input type="radio" name="ph-proxy" class="accent-primary" :checked="form.proxy_id === p.id" :disabled="readonly">
                    </td>
                    <td class="p-2 tabular-nums">#{{ p.id }}</td>
                    <td class="p-2 font-mono">{{ formatProxyAddress(p) }}</td>
                    <td class="p-2">{{ formatProxyIp(p) || t('phone.proxyNotTested') }}</td>
                    <td class="p-2 text-right tabular-nums">{{ boundCounts.get(p.id) ?? 0 }}</td>
                  </tr>
                </tbody>
              </table>
              <div v-if="!proxies.length" class="text-muted-foreground p-4 text-center text-xs">
                {{ t('phone.proxyEmpty') }} ·
                <RouterLink to="/proxy" class="text-primary hover:underline" @click="open = false">
                  {{ t('phone.proxyGotoManage') }}
                </RouterLink>
              </div>
            </div>
            <p class="text-muted-foreground text-xs">{{ t('phone.proxyManageHint') }}</p>
          </template>

          <!-- 关：未绑代理提示 -->
          <p v-else class="text-xs text-amber-600 dark:text-amber-400">
            {{ isCreate ? t('phone.proxyCreateReminder') : t('phone.proxyEditReminder') }}
          </p>
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
