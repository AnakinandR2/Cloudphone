<script setup lang="ts">
import type { PartnerCard } from '@/types/partner'
import { ExternalLink } from 'lucide-vue-next'
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import partnerApi from '@/api/modules/partner'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { getAnonymousId } from '@/utils/visitor'

const { t } = useI18n()
const items = ref<PartnerCard[]>([])
const loading = ref(true)

async function load() {
  loading.value = true
  try {
    const res = await partnerApi.list()
    items.value = res.data
  }
  finally {
    loading.value = false
  }
}

// 点击 CTA：先 best-effort 上报点击（带匿名标识 + channel=my），再打开推广链接。
// 上报失败也不阻断跳转。
async function visit(p: PartnerCard) {
  try {
    await partnerApi.click(p.id, { anonymous_id: getAnonymousId(), channel: 'my' })
  }
  catch {
    // 忽略：点击上报是 best-effort
  }
  finally {
    window.open(p.promo_url, '_blank', 'noopener,noreferrer')
  }
}

onMounted(load)
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ t('partner.recommendTitle') }}</CardTitle>
      <CardDescription>{{ t('partner.recommendDesc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <!-- 加载骨架 -->
      <div v-if="loading" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="i in 3" :key="i" class="space-y-3 rounded-lg border p-4">
          <Skeleton class="h-32 w-full rounded-md" />
          <Skeleton class="h-5 w-1/2" />
          <Skeleton class="h-4 w-full" />
          <Skeleton class="h-9 w-full" />
        </div>
      </div>

      <div v-else-if="items.length === 0" class="text-muted-foreground py-16 text-center text-sm">
        {{ t('partner.empty') }}
      </div>

      <div v-else class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <div
          v-for="p in items"
          :key="p.id"
          class="group hover:border-primary/50 flex flex-col overflow-hidden rounded-lg border transition-colors"
        >
          <div class="bg-muted aspect-[2/1] w-full overflow-hidden">
            <img
              v-if="p.image_url"
              :src="p.image_url"
              :alt="p.name"
              class="size-full object-cover transition-transform group-hover:scale-105"
            >
          </div>
          <div class="flex flex-1 flex-col gap-2 p-4">
            <div class="flex items-center gap-2">
              <img v-if="p.logo_url" :src="p.logo_url" :alt="p.name" class="size-8 rounded object-contain">
              <span class="font-medium">{{ p.name }}</span>
            </div>
            <p class="text-muted-foreground line-clamp-3 flex-1 text-sm">
              {{ p.intro }}
            </p>
            <Button class="mt-1 w-full" size="sm" @click="visit(p)">
              <ExternalLink class="size-4" /> {{ t('partner.visit') }}
            </Button>
          </div>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
