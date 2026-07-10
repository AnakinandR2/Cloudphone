<script setup lang="ts">
import { ArrowRight, Bell, Loader2, Mail, Plus, Search, Sparkles } from 'lucide-vue-next'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import FilterBar from '@/components/FilterBar.vue'
import CountTabs from '@/components/CountTabs.vue'
import FilterDatePicker from '@/components/FilterDatePicker.vue'
import FilterDateRangePicker from '@/components/FilterDateRangePicker.vue'
import FilterField from '@/components/FilterField.vue'
import FilterSearchInput from '@/components/FilterSearchInput.vue'
import FilterSelect from '@/components/FilterSelect.vue'
import TableComponentDemo from '@/components/demos/TableComponentDemo.vue'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Separator } from '@/components/ui/separator'

const { t } = useI18n()
const loading = ref(false)
const filterSearch = ref('')
const filterStatus = ref('all')
const filterDate = ref('')
const filterDateRange = ref({ from: '', to: '' })
const countTab = ref('pending')

const filterStatusOptions = computed(() => [
  { value: 'all', label: t('comp.filterSelectAll') },
  { value: 'active', label: t('comp.filterSelectActive') },
  { value: 'paused', label: t('comp.filterSelectPaused') },
])

const countTabItems = computed(() => [
  { value: 'pending', label: t('comp.countTabsPending'), count: 128 },
  { value: 'claimed', label: t('comp.countTabsClaimed'), count: 12 },
  { value: 'unavailable', label: t('comp.countTabsUnavailable'), count: 3 },
])

function fakeLoad() {
  loading.value = true
  setTimeout(() => (loading.value = false), 1600)
}
</script>

<template>
  <div class="space-y-6 pb-2">
    <!-- 页头 -->
    <div class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight">
          {{ t('comp.basicTitle') }}
        </h1>
        <p class="text-muted-foreground text-sm">
          {{ t('comp.basicDesc') }}
        </p>
      </div>
      <Badge variant="secondary" class="gap-1">
        <Sparkles class="size-3" />shadcn-vue
      </Badge>
    </div>

    <div class="grid gap-6 xl:grid-cols-2">
      <!-- 按钮 -->
      <Card>
        <CardHeader>
          <CardTitle>{{ t('comp.buttons') }}</CardTitle>
          <CardDescription>{{ t('comp.buttonsDesc') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-5">
          <div class="flex flex-wrap gap-2">
            <Button>{{ t('comp.variantDefault') }}</Button>
            <Button variant="secondary">
              {{ t('comp.variantSecondary') }}
            </Button>
            <Button variant="outline">
              {{ t('comp.variantOutline') }}
            </Button>
            <Button variant="ghost">
              {{ t('comp.variantGhost') }}
            </Button>
            <Button variant="link">
              {{ t('comp.variantLink') }}
            </Button>
            <Button variant="destructive">
              {{ t('comp.variantDestructive') }}
            </Button>
            <Button variant="warning">
              {{ t('comp.variantWarning') }}
            </Button>
          </div>

          <Separator />

          <div class="flex flex-wrap items-center gap-2">
            <Button size="sm">
              {{ t('comp.sizeSm') }}
            </Button>
            <Button>{{ t('comp.sizeMd') }}</Button>
            <Button size="lg">
              {{ t('comp.sizeLg') }}
            </Button>
            <Button size="icon" variant="outline">
              <Plus />
            </Button>
            <Button size="icon-sm" variant="ghost">
              <Bell />
            </Button>
          </div>

          <Separator />

          <div class="flex flex-wrap items-center gap-2">
            <Button><Plus />{{ t('comp.withIcon') }}</Button>
            <Button variant="outline">
              {{ t('comp.next') }}<ArrowRight />
            </Button>
            <Button :disabled="loading" @click="fakeLoad">
              <Loader2 v-if="loading" class="animate-spin" />
              {{ loading ? t('comp.loading') : t('comp.clickLoad') }}
            </Button>
            <Button disabled>
              {{ t('comp.disabled') }}
            </Button>
          </div>
        </CardContent>
      </Card>

      <!-- 徽章 -->
      <Card>
        <CardHeader>
          <CardTitle>{{ t('comp.badges') }}</CardTitle>
          <CardDescription>{{ t('comp.badgesDesc') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-5">
          <div class="flex flex-wrap gap-2">
            <Badge>{{ t('comp.variantDefault') }}</Badge>
            <Badge variant="secondary">
              {{ t('comp.variantSecondary') }}
            </Badge>
            <Badge variant="outline">
              {{ t('comp.variantOutline') }}
            </Badge>
            <Badge variant="destructive">
              {{ t('comp.variantDestructive') }}
            </Badge>
          </div>
          <Separator />
          <div class="flex flex-wrap gap-2">
            <Badge class="gap-1 border-transparent bg-emerald-500/15 text-emerald-600 dark:text-emerald-400">
              <span class="size-1.5 rounded-full bg-emerald-500" />{{ t('comp.statusActive') }}
            </Badge>
            <Badge class="gap-1 border-transparent bg-amber-500/15 text-amber-600 dark:text-amber-400">
              <span class="size-1.5 rounded-full bg-amber-500" />{{ t('comp.statusPending') }}
            </Badge>
            <Badge class="gap-1 border-transparent bg-slate-500/15 text-slate-600 dark:text-slate-400">
              <span class="size-1.5 rounded-full bg-slate-400" />{{ t('comp.statusOffline') }}
            </Badge>
            <Badge variant="outline" class="gap-1">
              <Bell class="size-3" />8
            </Badge>
          </div>
        </CardContent>
      </Card>

      <!-- 输入框 -->
      <Card>
        <CardHeader>
          <CardTitle>{{ t('comp.inputs') }}</CardTitle>
          <CardDescription>{{ t('comp.inputsDesc') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-4">
          <Input :placeholder="t('comp.inputPlaceholder')" />
          <div class="relative">
            <Search class="text-muted-foreground pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2" />
            <Input class="pl-9" :placeholder="t('common.search')" />
          </div>
          <div class="relative">
            <Mail class="text-muted-foreground pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2" />
            <Input type="email" class="pl-9" placeholder="name@example.com" />
          </div>
          <div class="flex gap-2">
            <Input :placeholder="t('comp.inputWithButton')" />
            <Button><Search />{{ t('common.search') }}</Button>
          </div>
          <Input disabled :placeholder="t('comp.disabled')" />
        </CardContent>
      </Card>

      <!-- 头像 -->
      <Card>
        <CardHeader>
          <CardTitle>{{ t('comp.avatars') }}</CardTitle>
          <CardDescription>{{ t('comp.avatarsDesc') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-5">
          <div class="flex items-center gap-3">
            <Avatar class="size-8">
              <AvatarImage src="https://i.pravatar.cc/120?img=12" alt="user" />
              <AvatarFallback>AB</AvatarFallback>
            </Avatar>
            <Avatar>
              <AvatarImage src="https://i.pravatar.cc/120?img=32" alt="user" />
              <AvatarFallback>CD</AvatarFallback>
            </Avatar>
            <Avatar class="size-12">
              <AvatarImage src="https://i.pravatar.cc/120?img=5" alt="user" />
              <AvatarFallback>EF</AvatarFallback>
            </Avatar>
            <Avatar class="size-12">
              <AvatarFallback class="bg-primary/10 text-primary font-medium">
                JK
              </AvatarFallback>
            </Avatar>
          </div>
          <Separator />
          <div class="flex items-center">
            <Avatar
              v-for="(n, i) in [11, 22, 33, 44]"
              :key="n"
              class="ring-background size-9 ring-2"
              :class="i > 0 ? '-ml-3' : ''"
            >
              <AvatarImage :src="`https://i.pravatar.cc/120?img=${n}`" alt="user" />
              <AvatarFallback>{{ n }}</AvatarFallback>
            </Avatar>
            <span class="bg-muted text-muted-foreground ring-background -ml-3 flex size-9 items-center justify-center rounded-full text-xs font-medium ring-2">
              +9
            </span>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- 计数 Tab -->
    <Card>
      <CardHeader>
        <CardTitle>{{ t('comp.countTabs') }}</CardTitle>
        <CardDescription>{{ t('comp.countTabsDesc') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <CountTabs v-model="countTab" :items="countTabItems" />
      </CardContent>
    </Card>

    <!-- 表格筛选 -->
    <Card>
      <CardHeader>
        <CardTitle>{{ t('comp.tableFilters') }}</CardTitle>
        <CardDescription>{{ t('comp.tableFiltersDesc') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <FilterBar>
          <FilterField :label="t('comp.filterSearchLabel')">
            <FilterSearchInput v-model="filterSearch" :placeholder="t('comp.filterSearchPlaceholder')" />
          </FilterField>
          <FilterField :label="t('comp.filterSelectLabel')">
            <FilterSelect
              v-model="filterStatus"
              :options="filterStatusOptions"
              :placeholder="t('comp.filterSelectAll')"
            />
          </FilterField>
          <FilterField :label="t('comp.filterDateLabel')">
            <FilterDatePicker v-model="filterDate" :placeholder="t('comp.filterDateEmpty')" />
          </FilterField>
          <FilterField :label="t('comp.filterDateRangeLabel')">
            <FilterDateRangePicker v-model="filterDateRange" :placeholder="t('comp.filterDateRangeEmpty')" />
          </FilterField>
        </FilterBar>
      </CardContent>
    </Card>

    <!-- 表格组件 -->
    <Card>
      <CardHeader>
        <CardTitle>{{ t('comp.tableComponent') }}</CardTitle>
        <CardDescription>{{ t('comp.tableComponentDesc') }}</CardDescription>
      </CardHeader>
      <CardContent class="min-w-0 overflow-visible">
        <TableComponentDemo />
      </CardContent>
    </Card>

    <!-- 卡片范例 -->
    <Card>
      <CardHeader>
        <CardTitle>{{ t('comp.cards') }}</CardTitle>
        <CardDescription>{{ t('comp.cardsDesc') }}</CardDescription>
      </CardHeader>
      <CardContent class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Card
          v-for="plan in [
            { name: t('comp.planStarter'), price: '¥0', accent: false },
            { name: t('comp.planPro'), price: '¥99', accent: true },
            { name: t('comp.planTeam'), price: '¥299', accent: false },
          ]"
          :key="plan.name"
          interactive
          :class="plan.accent ? 'border-primary ring-primary/20 ring-1' : ''"
        >
          <CardHeader>
            <CardDescription class="flex items-center justify-between">
              {{ plan.name }}
              <Badge v-if="plan.accent">
                {{ t('comp.popular') }}
              </Badge>
            </CardDescription>
            <CardTitle class="text-3xl">
              {{ plan.price }}<span class="text-muted-foreground text-sm font-normal">/mo</span>
            </CardTitle>
          </CardHeader>
          <CardFooter>
            <Button class="w-full" :variant="plan.accent ? 'default' : 'outline'">
              {{ t('comp.choose') }}
            </Button>
          </CardFooter>
        </Card>
      </CardContent>
    </Card>
  </div>
</template>
