<script setup lang="ts">
import { Loader2, Upload, X } from 'lucide-vue-next'
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import partnerApi from '@/api/modules/partner'
import { Button } from '@/components/ui/button'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

const props = defineProps<{ id: number, mode: 'create' | 'edit' }>()
const emit = defineEmits<{ success: [] }>()
const open = defineModel<boolean>({ default: false })
const { t } = useI18n()
const isCreate = computed(() => props.mode === 'create')
const submitting = ref(false)
const nameError = ref('')
const promoError = ref('')

// 两个上传位各自的进行态。
const uploading = reactive({ logo: false, image: false })

const form = reactive({
  name: '',
  promo_url: '',
  intro: '',
  sort: 0,
  enabled: true,
  logo_url: '',
  image_url: '',
})

const dialogTitle = computed(() => (isCreate.value ? t('partner.createTitle') : t('partner.editTitle')))

watch(open, async (v) => {
  if (!v)
    return
  Object.assign(form, { name: '', promo_url: '', intro: '', sort: 0, enabled: true, logo_url: '', image_url: '' })
  nameError.value = ''
  promoError.value = ''
  if (props.id !== 0) {
    const res = await partnerApi.list({ page: 1, size: 999 })
    const found = res.data.list.find(p => p.id === props.id)
    if (found)
      Object.assign(form, found)
  }
})

// 选图即传到 S3，存返回 URL；field 决定写哪个字段。
async function pickImage(e: Event, field: 'logo' | 'image') {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file)
    return
  uploading[field] = true
  try {
    const res = await partnerApi.upload(file)
    if (field === 'logo')
      form.logo_url = res.data.url
    else form.image_url = res.data.url
  }
  finally {
    uploading[field] = false
    input.value = '' // 允许重复选同一文件
  }
}

async function submit() {
  nameError.value = ''
  promoError.value = ''
  if (!form.name.trim()) {
    nameError.value = t('partner.errNameRequired')
    return
  }
  if (!form.promo_url.trim()) {
    promoError.value = t('partner.errPromoRequired')
    return
  }
  submitting.value = true
  try {
    const payload = {
      name: form.name,
      promo_url: form.promo_url,
      intro: form.intro,
      sort: Number(form.sort) || 0,
      enabled: form.enabled,
      logo_url: form.logo_url,
      image_url: form.image_url,
    }
    if (isCreate.value) {
      await partnerApi.create(payload)
      toast.success(t('crud.createOk'))
    }
    else {
      await partnerApi.update(props.id, payload)
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
        <DialogDescription class="sr-only">
          {{ dialogTitle }}
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-2">
        <div class="space-y-2">
          <Label for="p-name">{{ t('partner.fName') }}</Label>
          <Input id="p-name" v-model="form.name" :placeholder="t('partner.fNamePlaceholder')" />
          <p v-if="nameError" class="text-destructive text-xs">
            {{ nameError }}
          </p>
        </div>

        <div class="space-y-2">
          <Label for="p-promo">{{ t('partner.fPromo') }}</Label>
          <Input id="p-promo" v-model="form.promo_url" :placeholder="t('partner.fPromoPlaceholder')" />
          <p v-if="promoError" class="text-destructive text-xs">
            {{ promoError }}
          </p>
        </div>

        <div class="space-y-2">
          <Label for="p-intro">{{ t('partner.fIntro') }}</Label>
          <Textarea id="p-intro" v-model="form.intro" :rows="3" :placeholder="t('partner.fIntroPlaceholder')" />
        </div>

        <!-- 图片上传：logo + 配图 -->
        <div class="grid grid-cols-2 gap-4">
          <div class="space-y-2">
            <Label>{{ t('partner.fLogo') }}</Label>
            <div class="border-input flex h-24 items-center justify-center overflow-hidden rounded-md border bg-muted/30">
              <Loader2 v-if="uploading.logo" class="text-muted-foreground size-5 animate-spin" />
              <div v-else-if="form.logo_url" class="group relative size-full">
                <img :src="form.logo_url" class="size-full object-contain" alt="logo">
                <button type="button" class="bg-background/80 absolute right-1 top-1 rounded p-0.5" @click="form.logo_url = ''">
                  <X class="size-3.5" />
                </button>
              </div>
              <label v-else class="text-muted-foreground flex cursor-pointer flex-col items-center gap-1 text-xs">
                <Upload class="size-4" />
                {{ t('partner.uploadPick') }}
                <input type="file" accept="image/*" class="hidden" @change="(e) => pickImage(e, 'logo')">
              </label>
            </div>
          </div>
          <div class="space-y-2">
            <Label>{{ t('partner.fImage') }}</Label>
            <div class="border-input flex h-24 items-center justify-center overflow-hidden rounded-md border bg-muted/30">
              <Loader2 v-if="uploading.image" class="text-muted-foreground size-5 animate-spin" />
              <div v-else-if="form.image_url" class="group relative size-full">
                <img :src="form.image_url" class="size-full object-cover" alt="image">
                <button type="button" class="bg-background/80 absolute right-1 top-1 rounded p-0.5" @click="form.image_url = ''">
                  <X class="size-3.5" />
                </button>
              </div>
              <label v-else class="text-muted-foreground flex cursor-pointer flex-col items-center gap-1 text-xs">
                <Upload class="size-4" />
                {{ t('partner.uploadPick') }}
                <input type="file" accept="image/*" class="hidden" @change="(e) => pickImage(e, 'image')">
              </label>
            </div>
          </div>
        </div>
        <p class="text-muted-foreground text-xs">
          {{ t('partner.uploadHint') }}
        </p>

        <div class="flex items-end gap-6">
          <div class="space-y-2">
            <Label for="p-sort">{{ t('partner.fSort') }}</Label>
            <Input id="p-sort" v-model="form.sort" type="number" class="w-28" />
            <p class="text-muted-foreground text-xs">
              {{ t('partner.fSortHint') }}
            </p>
          </div>
          <div class="flex items-center gap-2 pb-1">
            <Switch id="p-enabled" v-model="form.enabled" />
            <Label for="p-enabled">{{ t('partner.fEnabled') }}</Label>
          </div>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="open = false">
          {{ t('crud.cancel') }}
        </Button>
        <Button :disabled="submitting || uploading.logo || uploading.image" @click="submit">
          {{ t('crud.confirm') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
