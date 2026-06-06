<script setup lang="ts">
import { Activity, CreditCard, TrendingUp, Users } from 'lucide-vue-next'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

const { t } = useI18n()

const stats = computed(() => [
  { label: t('dashboard.totalUsers'), value: '2,350', delta: '+12.5%', icon: Users },
  { label: t('dashboard.activeSessions'), value: '573', delta: '+4.1%', icon: Activity },
  { label: t('dashboard.revenue'), value: '¥45,231', delta: '+20.3%', icon: CreditCard },
  { label: t('dashboard.conversion'), value: '3.24%', delta: '+0.8%', icon: TrendingUp },
])
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-semibold tracking-tight">
        {{ t('dashboard.title') }}
      </h1>
      <p class="text-muted-foreground text-sm">
        {{ t('dashboard.welcome') }}
      </p>
    </div>

    <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <Card
        v-for="s in stats"
        :key="s.label"
        class="transition-all duration-200 hover:-translate-y-0.5 hover:shadow-md"
      >
        <CardHeader>
          <CardDescription class="flex items-center justify-between">
            {{ s.label }}
            <component :is="s.icon" class="text-muted-foreground size-4" />
          </CardDescription>
          <CardTitle class="text-2xl">
            {{ s.value }}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <span class="text-xs text-emerald-600">{{ s.delta }} {{ t('dashboard.vsLast') }}</span>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
