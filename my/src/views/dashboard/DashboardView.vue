<script setup lang="ts">
import { VisDonut, VisSingleContainer } from '@unovis/vue'
import {
  AppWindow,
  CheckCircle2,
  ChevronRight,
  Clock,
  Loader2,
  Network,
  Plus,
  ScrollText,
  Smartphone,
  Upload,
  Wallet,
  XCircle,
} from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import UsageLineChart from '@/views/billing/UsageLineChart.vue'

const { t } = useI18n()
const router = useRouter()

// 全部为原型静态数据，待接入真实接口。
const kpis = [
  { key: 'phones', icon: Smartphone, value: '42', sub: 'kpiPhonesSub', subN: 31, delta: '+3', tone: 'text-emerald-600' },
  { key: 'proxies', icon: Network, value: '18', sub: 'kpiProxiesSub', subN: 15, delta: '+1', tone: 'text-emerald-600' },
  { key: 'apps', icon: AppWindow, value: '27', sub: 'kpiAppsSub', subN: 27, delta: '+5', tone: 'text-emerald-600' },
  { key: 'balance', icon: Wallet, value: '¥ 1,280', sub: 'kpiBalanceSub', subN: 26, delta: '', tone: 'text-muted-foreground' },
]

// 云手机状态分布（运行/停止/创建/异常）。
const dist = [
  { key: 'running', n: 31, color: '#10b981' },
  { key: 'stopped', n: 7, color: '#cbd5e1' },
  { key: 'creating', n: 3, color: '#f59e0b' },
  { key: 'error', n: 1, color: '#ef4444' },
]
const distTotal = dist.reduce((s, d) => s + d.n, 0)
const distValues = dist.map(d => d.n)
function donutValue(d: number) {
  return d
}
function donutColor(_v: number, i: number) {
  return dist[i].color
}

// 近 7 天活跃云手机数。
const trend = [22, 28, 25, 31, 27, 34, 31]
const trendData = trend.map((v, i) => ({ label: `D${i + 1}`, value: v }))

// 最近任务执行。
const recentTasks = [
  { id: 1, name: '每日签到 - 早 8 点', time: '08:00', status: 'success' },
  { id: 2, name: '养号 - 循环', time: '16:00', status: 'running' },
  { id: 3, name: '批量点赞 - 手动', time: '昨天 21:14', status: 'failed' },
  { id: 4, name: '关键词采集 - 工作日', time: '昨天 09:30', status: 'success' },
]
const taskStatusMeta: Record<string, { icon: typeof CheckCircle2, cls: string }> = {
  success: { icon: CheckCircle2, cls: 'text-emerald-600' },
  failed: { icon: XCircle, cls: 'text-destructive' },
  running: { icon: Loader2, cls: 'text-muted-foreground animate-spin' },
}

const quickActions = [
  { key: 'quickPhone', icon: Plus, to: '/phone' },
  { key: 'quickApp', icon: Upload, to: '/phone/apps' },
  { key: 'quickTask', icon: ScrollText, to: '/automation/schedules' },
  { key: 'quickBuy', icon: Wallet, to: '/billing' },
]
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center gap-2">
      <div>
        <h1 class="flex items-center gap-2 text-2xl font-semibold tracking-tight">
          {{ t('dashboard.title') }}
          <Badge variant="outline" class="border-amber-500 font-normal text-amber-600 dark:text-amber-400">
            {{ t('dashboard.prototype') }}
          </Badge>
        </h1>
        <p class="text-sm text-muted-foreground">
          {{ t('dashboard.welcome') }}
        </p>
      </div>
    </div>

    <!-- KPI -->
    <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <Card v-for="k in kpis" :key="k.key" interactive>
        <CardHeader class="pb-2">
          <CardDescription class="flex items-center justify-between">
            {{ t(`dashboard.${k.key === 'phones' ? 'kpiPhones' : k.key === 'proxies' ? 'kpiProxies' : k.key === 'apps' ? 'kpiApps' : 'kpiBalance'}`) }}
            <component :is="k.icon" class="size-4 text-muted-foreground" />
          </CardDescription>
          <CardTitle class="text-2xl">
            {{ k.value }}
          </CardTitle>
        </CardHeader>
        <CardContent class="flex items-center justify-between">
          <span class="text-xs text-muted-foreground">{{ t(`dashboard.${k.sub}`, { n: k.subN }) }}</span>
          <span v-if="k.delta" class="text-xs" :class="k.tone">{{ k.delta }} {{ t('dashboard.vsLast') }}</span>
        </CardContent>
      </Card>
    </div>

    <!-- 状态分布 + 趋势 -->
    <div class="grid gap-4 lg:grid-cols-3">
      <Card>
        <CardHeader>
          <CardTitle class="text-base">
            {{ t('dashboard.statusTitle') }}
          </CardTitle>
          <CardDescription>{{ t('dashboard.statusDesc') }}</CardDescription>
        </CardHeader>
        <CardContent class="flex items-center gap-5">
          <div class="relative size-28 shrink-0">
            <VisSingleContainer :data="distValues" :height="112" :width="112">
              <VisDonut :value="donutValue" :color="donutColor" :arc-width="14" :pad-angle="0.015" :corner-radius="2" />
            </VisSingleContainer>
            <div class="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
              <span class="text-xl font-semibold tabular-nums">{{ distTotal }}</span>
              <span class="text-[10px] text-muted-foreground">{{ t('dashboard.statusTotal') }}</span>
            </div>
          </div>
          <div class="flex-1 space-y-1.5">
            <div v-for="d in dist" :key="d.key" class="flex items-center gap-2 text-sm">
              <span class="size-2.5 shrink-0 rounded-full" :style="{ background: d.color }" />
              <span class="text-muted-foreground">{{ t(`dashboard.status_${d.key}`) }}</span>
              <span class="ml-auto tabular-nums font-medium">{{ d.n }}</span>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card class="lg:col-span-2">
        <CardHeader>
          <CardTitle class="text-base">
            {{ t('dashboard.trendTitle') }}
          </CardTitle>
          <CardDescription>{{ t('dashboard.trendDesc') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <UsageLineChart :data="trendData" :height="180" color="#14b8a6" />
        </CardContent>
      </Card>
    </div>

    <!-- 最近任务 + 快捷操作 -->
    <div class="grid gap-4 lg:grid-cols-3">
      <Card class="lg:col-span-2">
        <CardHeader class="flex-row items-center justify-between space-y-0">
          <CardTitle class="text-base">
            {{ t('dashboard.tasksTitle') }}
          </CardTitle>
          <Button variant="ghost" size="sm" class="text-muted-foreground" @click="router.push('/automation/task-logs')">
            {{ t('dashboard.viewAll') }} <ChevronRight class="size-3.5" />
          </Button>
        </CardHeader>
        <CardContent class="divide-y">
          <div v-for="r in recentTasks" :key="r.id" class="flex items-center gap-3 py-2.5 first:pt-0 last:pb-0">
            <component :is="taskStatusMeta[r.status].icon" class="size-4 shrink-0" :class="taskStatusMeta[r.status].cls" />
            <span class="truncate text-sm font-medium">{{ r.name }}</span>
            <Badge variant="secondary" class="ml-auto shrink-0 font-normal">
              {{ t(`dashboard.task_${r.status}`) }}
            </Badge>
            <span class="flex shrink-0 items-center gap-1 text-xs tabular-nums text-muted-foreground"><Clock class="size-3" />{{ r.time }}</span>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle class="text-base">
            {{ t('dashboard.quickTitle') }}
          </CardTitle>
        </CardHeader>
        <CardContent class="grid grid-cols-2 gap-2">
          <Button
            v-for="q in quickActions" :key="q.key"
            variant="outline" class="h-auto flex-col gap-1.5 py-3"
            @click="router.push(q.to)"
          >
            <component :is="q.icon" class="size-5" />
            <span class="text-xs">{{ t(`dashboard.${q.key}`) }}</span>
          </Button>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
