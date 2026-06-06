<script setup lang="ts">
import type { AccessLog } from '@/types/access_log'
import { onMounted, ref } from 'vue'

import { useI18n } from 'vue-i18n'
import accessLogApi from '@/api/modules/access_log'
import { Skeleton } from '@/components/ui/skeleton'

const props = defineProps<{ id: number }>()
const { t } = useI18n()

const loading = ref(true)
const detail = ref<AccessLog | null>(null)

function formatJSON(raw: string) {
  if (!raw) return '—'
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  }
  catch {
    return raw
  }
}

// 展开时调用详情接口获取请求/响应头与体（参考原查看功能）
onMounted(async () => {
  try {
    const res = await accessLogApi.detail(props.id)
    detail.value = res.data
  }
  finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="p-4">
    <!-- 加载骨架屏（与列表风格统一） -->
    <div v-if="loading" class="space-y-4">
      <div class="grid grid-cols-1 gap-x-6 gap-y-3 md:grid-cols-4">
        <div v-for="i in 2" :key="i" class="space-y-1.5">
          <Skeleton class="h-3 w-16" />
          <Skeleton class="h-4 w-24" />
        </div>
      </div>
      <div class="grid gap-4 md:grid-cols-2">
        <div v-for="i in 2" :key="i" class="space-y-3">
          <Skeleton class="h-4 w-20" />
          <Skeleton class="h-20 w-full" />
          <Skeleton class="h-4 w-20" />
          <Skeleton class="h-20 w-full" />
        </div>
      </div>
    </div>

    <div v-else-if="detail" class="space-y-4">
      <!-- 仅展示列表未呈现的字段（其余字段列上已有，不再重复） -->
      <dl class="grid grid-cols-1 gap-x-6 gap-y-3 text-sm md:grid-cols-4">
        <div>
          <dt class="text-muted-foreground text-xs">
            {{ t('accessLog.colUser') }} ID
          </dt>
          <dd class="tabular-nums">
            <template v-if="detail.user_id">
              {{ detail.user_id }}
            </template>
            <span v-else class="text-muted-foreground">{{ t('accessLog.unauth') }}</span>
          </dd>
        </div>
        <div class="md:col-span-3">
          <dt class="text-muted-foreground text-xs">
            {{ t('accessLog.ua') }}
          </dt>
          <dd class="text-muted-foreground break-all text-xs">
            {{ detail.user_agent || '—' }}
          </dd>
        </div>
      </dl>

      <!-- 左请求 / 右响应 -->
      <div class="grid gap-4 md:grid-cols-2">
        <div class="space-y-3">
          <div class="space-y-1.5">
            <div class="text-sm font-medium">
              {{ t('accessLog.reqHeaders') }}
            </div>
            <pre class="bg-background max-h-48 overflow-auto rounded-md border p-3 font-mono text-xs whitespace-pre-wrap">{{ formatJSON(detail.request_headers) }}</pre>
          </div>
          <div class="space-y-1.5">
            <div class="text-sm font-medium">
              {{ t('accessLog.reqBody') }}
            </div>
            <pre class="bg-background max-h-48 overflow-auto rounded-md border p-3 font-mono text-xs whitespace-pre-wrap">{{ formatJSON(detail.request_body) }}</pre>
          </div>
        </div>
        <div class="space-y-3">
          <div class="space-y-1.5">
            <div class="text-sm font-medium">
              {{ t('accessLog.resHeaders') }}
            </div>
            <pre class="bg-background max-h-48 overflow-auto rounded-md border p-3 font-mono text-xs whitespace-pre-wrap">{{ formatJSON(detail.response_headers) }}</pre>
          </div>
          <div class="space-y-1.5">
            <div class="text-sm font-medium">
              {{ t('accessLog.resBody') }}
            </div>
            <pre class="bg-background max-h-48 overflow-auto rounded-md border p-3 font-mono text-xs whitespace-pre-wrap">{{ formatJSON(detail.response_body) }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
