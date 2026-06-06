<script setup lang="ts">
import type { BootPlan, PlanImage, VirtualMachine } from '@/types/cloudphone'
import { Loader2 } from 'lucide-vue-next'
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import cloudphoneApi from '@/api/modules/cloudphone'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'

const props = defineProps<{ host: VirtualMachine | null, kind: 'specs' | 'images' }>()
const open = defineModel<boolean>({ default: false })

const { t } = useI18n()

const loading = ref(false)
const plans = ref<BootPlan[]>([])
const images = ref<PlanImage[]>([])

async function load() {
  if (!props.host)
    return
  loading.value = true
  plans.value = []
  images.value = []
  try {
    if (props.kind === 'specs') {
      const res = await cloudphoneApi.listBootPlans(props.host.specificationId)
      plans.value = res.data ?? []
    }
    else {
      const res = await cloudphoneApi.listSpecImages(props.host.specificationId)
      images.value = res.data ?? []
    }
  }
  catch {
    toast.error(t('hostRes.loadFail'))
  }
  finally {
    loading.value = false
  }
}

watch(open, (v) => {
  if (v)
    load()
})
</script>

<template>
  <Dialog v-model:open="open">
    <DialogContent class="sm:max-w-2xl">
      <DialogHeader>
        <DialogTitle>
          {{ kind === 'specs' ? t('hostRes.specsTitle') : t('hostRes.imagesTitle') }}
          <span v-if="host" class="text-muted-foreground font-normal"> — {{ host.vmUid }}（{{ host.specificationName }}）</span>
        </DialogTitle>
        <DialogDescription>
          {{ kind === 'specs' ? t('hostRes.specsDesc') : t('hostRes.imagesDesc') }}
        </DialogDescription>
      </DialogHeader>

      <div class="max-h-[60vh] overflow-y-auto">
        <div v-if="loading" class="text-muted-foreground flex items-center justify-center gap-2 py-10 text-sm">
          <Loader2 class="size-4 animate-spin" /> {{ t('common.loading') }}
        </div>

        <!-- 可用云手机规格（套餐） -->
        <table v-else-if="kind === 'specs'" class="w-full text-sm">
          <thead class="text-muted-foreground border-b text-left text-xs">
            <tr>
              <th class="py-2 pr-3 font-medium">
                {{ t('hostRes.planName') }}
              </th>
              <th class="py-2 pr-3 font-medium">
                {{ t('hostRes.cpu') }}
              </th>
              <th class="py-2 pr-3 font-medium">
                {{ t('hostRes.memory') }}
              </th>
              <th class="py-2 pr-3 font-medium">
                {{ t('hostRes.storage') }}
              </th>
              <th class="py-2 pr-3 font-medium">
                {{ t('hostRes.resolution') }}
              </th>
              <th class="py-2 pr-3 font-medium">
                {{ t('hostRes.mode') }}
              </th>
              <th class="py-2 pr-3 text-right font-medium">
                {{ t('hostRes.raw') }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr v-for="p in plans" :key="p.id">
              <td class="py-2 pr-3">
                {{ p.planName || p.specName || `#${p.id}` }}
                <span v-if="p.scenarioName" class="text-muted-foreground text-xs">（{{ p.scenarioName }}）</span>
              </td>
              <td class="py-2 pr-3 tabular-nums">
                {{ p.minCore || p.core }}<span v-if="p.maxCore && p.maxCore !== p.minCore">~{{ p.maxCore }}</span> 核
              </td>
              <td class="py-2 pr-3 tabular-nums">
                <template v-if="p.minMemory || p.maxMemory">
                  {{ p.minMemory || p.maxMemory }}<span v-if="p.maxMemory && p.maxMemory !== p.minMemory">~{{ p.maxMemory }}</span>G
                </template>
                <template v-else>
                  {{ p.memory || '-' }}{{ p.memory ? 'G' : '' }}
                </template>
              </td>
              <td class="py-2 pr-3 tabular-nums">
                {{ p.storage ? `${p.storage}G` : '-' }}
              </td>
              <td class="py-2 pr-3 tabular-nums">
                {{ p.width }}×{{ p.height }} @{{ p.fps }}fps
              </td>
              <td class="py-2 pr-3">
                <Badge variant="outline">
                  {{ p.resourceUtilization || '-' }}
                </Badge>
              </td>
              <td class="py-2 pr-3 text-right">
                <Popover>
                  <PopoverTrigger as-child>
                    <Button variant="ghost" size="sm" class="h-7 px-2 text-xs">
                      {{ t('hostRes.viewRaw') }}
                    </Button>
                  </PopoverTrigger>
                  <PopoverContent align="end" class="max-h-[50vh] w-96 overflow-auto p-0">
                    <pre class="whitespace-pre-wrap break-all p-3 font-mono text-xs">{{ JSON.stringify(p, null, 2) }}</pre>
                  </PopoverContent>
                </Popover>
              </td>
            </tr>
            <tr v-if="!plans.length">
              <td colspan="7" class="text-muted-foreground py-10 text-center">
                {{ t('common.empty') }}
              </td>
            </tr>
          </tbody>
        </table>

        <!-- 可用镜像 -->
        <table v-else class="w-full text-sm">
          <thead class="text-muted-foreground border-b text-left text-xs">
            <tr>
              <th class="py-2 pr-3 font-medium">
                {{ t('hostRes.imageName') }}
              </th>
              <th class="py-2 pr-3 font-medium">
                {{ t('hostRes.androidVersion') }}
              </th>
              <th class="py-2 pr-3 font-medium">
                {{ t('hostRes.imageId') }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y">
            <tr v-for="im in images" :key="im.imageId">
              <td class="py-2 pr-3">
                {{ im.imageName || '-' }}
              </td>
              <td class="py-2 pr-3">
                <Badge v-if="im.androidVersion" variant="secondary">
                  Android {{ im.androidVersion }}
                </Badge>
                <span v-else>-</span>
              </td>
              <td class="text-muted-foreground py-2 pr-3 font-mono text-xs">
                {{ im.imageId }}
              </td>
            </tr>
            <tr v-if="!images.length">
              <td colspan="3" class="text-muted-foreground py-10 text-center">
                {{ t('common.empty') }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </DialogContent>
  </Dialog>
</template>
