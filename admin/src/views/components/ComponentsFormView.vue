<script setup lang="ts">
import type { DateValue } from '@internationalized/date'
import {
  DateFormatter,

  getLocalTimeZone,
} from '@internationalized/date'
import { CalendarIcon, Send } from 'lucide-vue-next'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'

import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

const { t } = useI18n()

const form = ref({
  name: '',
  email: '',
  role: '',
  plan: 'pro',
  bio: '',
  notify: true,
  agree: false,
})
const date = ref<DateValue>()

const df = computed(() => new DateFormatter('zh-CN', { dateStyle: 'long' }))
const dateLabel = computed(() =>
  date.value ? df.value.format(date.value.toDate(getLocalTimeZone())) : '',
)

function submit() {
  if (!form.value.agree) {
    toast.warning(t('comp.formAgreeWarn'))
    return
  }
  toast.success(t('comp.formSubmitted'), {
    description: `${form.value.name || '—'} · ${form.value.email || '—'}`,
  })
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-semibold tracking-tight">
        {{ t('comp.formTitle') }}
      </h1>
      <p class="text-muted-foreground text-sm">
        {{ t('comp.formDesc') }}
      </p>
    </div>

    <div class="grid items-start gap-6 lg:grid-cols-3">
      <!-- 表单主体 -->
      <Card class="lg:col-span-2">
        <CardHeader>
          <CardTitle>{{ t('comp.profileForm') }}</CardTitle>
          <CardDescription>{{ t('comp.profileFormDesc') }}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-6">
          <div class="grid gap-5 sm:grid-cols-2">
            <div class="space-y-2">
              <Label for="f-name">{{ t('table.name') }}</Label>
              <Input id="f-name" v-model="form.name" :placeholder="t('comp.namePlaceholder')" />
            </div>
            <div class="space-y-2">
              <Label for="f-email">{{ t('table.email') }}</Label>
              <Input id="f-email" v-model="form.email" type="email" placeholder="name@example.com" />
            </div>

            <!-- 下拉框 -->
            <div class="space-y-2">
              <Label>{{ t('table.role') }}</Label>
              <Select v-model="form.role">
                <SelectTrigger class="w-full">
                  <SelectValue :placeholder="t('comp.selectPlaceholder')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectLabel>{{ t('table.role') }}</SelectLabel>
                    <SelectItem value="admin">
                      {{ t('comp.roleAdmin') }}
                    </SelectItem>
                    <SelectItem value="editor">
                      {{ t('comp.roleEditor') }}
                    </SelectItem>
                    <SelectItem value="viewer">
                      {{ t('comp.roleViewer') }}
                    </SelectItem>
                    <SelectItem value="guest" disabled>
                      {{ t('comp.roleGuest') }}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>

            <!-- 日期选择 -->
            <div class="space-y-2">
              <Label>{{ t('comp.date') }}</Label>
              <Popover>
                <PopoverTrigger as-child>
                  <Button
                    variant="outline"
                    :class="cn('w-full justify-start text-left font-normal', !date && 'text-muted-foreground')"
                  >
                    <CalendarIcon />
                    {{ dateLabel || t('comp.pickDate') }}
                  </Button>
                </PopoverTrigger>
                <PopoverContent class="w-auto p-0">
                  <Calendar v-model="date" initial-focus />
                </PopoverContent>
              </Popover>
            </div>
          </div>

          <Separator />

          <!-- 单选 -->
          <div class="space-y-3">
            <Label>{{ t('comp.plan') }}</Label>
            <RadioGroup v-model="form.plan" class="grid gap-3 sm:grid-cols-3">
              <Label
                v-for="p in [
                  { v: 'starter', label: t('comp.planStarter') },
                  { v: 'pro', label: t('comp.planPro') },
                  { v: 'team', label: t('comp.planTeam') },
                ]"
                :key="p.v"
                :for="`plan-${p.v}`"
                class="hover:bg-accent has-[:checked]:border-primary has-[:checked]:bg-primary/5 flex cursor-pointer items-center gap-3 rounded-lg border p-3 transition-colors"
              >
                <RadioGroupItem :id="`plan-${p.v}`" :value="p.v" />
                <span class="text-sm font-medium">{{ p.label }}</span>
              </Label>
            </RadioGroup>
          </div>

          <!-- 文本域 -->
          <div class="space-y-2">
            <Label for="f-bio">{{ t('comp.bio') }}</Label>
            <Textarea id="f-bio" v-model="form.bio" :placeholder="t('comp.bioPlaceholder')" rows="3" />
          </div>

          <Separator />

          <!-- 开关 + 复选 -->
          <div class="flex items-center justify-between rounded-lg border p-4">
            <div class="space-y-0.5">
              <Label>{{ t('comp.notify') }}</Label>
              <p class="text-muted-foreground text-sm">
                {{ t('comp.notifyDesc') }}
              </p>
            </div>
            <Switch v-model="form.notify" />
          </div>
          <div class="flex items-start gap-3">
            <Checkbox id="f-agree" v-model="form.agree" />
            <Label for="f-agree" class="text-muted-foreground text-sm font-normal leading-relaxed">
              {{ t('comp.agree') }}
            </Label>
          </div>
        </CardContent>
        <CardFooter class="gap-2">
          <Button @click="submit">
            <Send />{{ t('comp.submit') }}
          </Button>
          <Button variant="outline" @click="date = undefined">
            {{ t('common.reset') }}
          </Button>
        </CardFooter>
      </Card>

      <!-- 实时预览 -->
      <Card class="lg:sticky lg:top-4">
        <CardHeader>
          <CardTitle>{{ t('comp.preview') }}</CardTitle>
          <CardDescription>{{ t('comp.previewDesc') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <pre class="bg-muted/50 overflow-x-auto rounded-lg p-3 text-xs leading-relaxed">{{ JSON.stringify({ ...form, date: dateLabel }, null, 2) }}</pre>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
