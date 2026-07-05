<script setup lang="ts">
import type { DateValue } from '@internationalized/date'
import {
  DateFormatter,

  getLocalTimeZone,
} from '@internationalized/date'
import { toTypedSchema } from '@vee-validate/zod'
import { CalendarIcon, Send } from 'lucide-vue-next'
import { useForm } from 'vee-validate'
import { computed } from 'vue'
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
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
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
import { buildProfileSchema } from './formSchema'

const { t } = useI18n()

// schema 用 computed 包裹，语言切换时校验文案随之更新。
const validationSchema = computed(() => toTypedSchema(buildProfileSchema(t)))

const { handleSubmit, values, resetForm } = useForm({
  validationSchema,
  initialValues: {
    name: '',
    email: '',
    role: '',
    plan: 'pro',
    bio: '',
    notify: true,
    agree: false,
    date: undefined as DateValue | undefined,
  },
})

const df = computed(() => new DateFormatter('zh-CN', { dateStyle: 'long' }))
function formatDate(d?: DateValue) {
  return d ? df.value.format(d.toDate(getLocalTimeZone())) : ''
}

const previewData = computed(() => ({
  ...values,
  date: formatDate(values.date as DateValue | undefined),
}))

const onSubmit = handleSubmit((vals) => {
  toast.success(t('comp.formSubmitted'), {
    description: `${vals.name} · ${vals.email}`,
  })
})

function onReset() {
  resetForm()
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
      <form class="lg:col-span-2" novalidate @submit="onSubmit">
        <Card>
          <CardHeader>
            <CardTitle>{{ t('comp.profileForm') }}</CardTitle>
            <CardDescription>{{ t('comp.profileFormDesc') }}</CardDescription>
          </CardHeader>
          <CardContent class="space-y-6">
            <div class="grid gap-5 sm:grid-cols-2">
              <FormField v-slot="{ componentField }" name="name">
                <FormItem>
                  <FormLabel required>
                    {{ t('table.name') }}
                  </FormLabel>
                  <FormControl>
                    <Input v-bind="componentField" :placeholder="t('comp.namePlaceholder')" />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              </FormField>

              <FormField v-slot="{ componentField }" name="email">
                <FormItem>
                  <FormLabel required>
                    {{ t('table.email') }}
                  </FormLabel>
                  <FormControl>
                    <Input v-bind="componentField" type="email" placeholder="name@example.com" />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              </FormField>

              <!-- 下拉框 -->
              <FormField v-slot="{ componentField }" name="role">
                <FormItem>
                  <FormLabel required>
                    {{ t('table.role') }}
                  </FormLabel>
                  <Select v-bind="componentField">
                    <FormControl>
                      <SelectTrigger class="w-full">
                        <SelectValue :placeholder="t('comp.selectPlaceholder')" />
                      </SelectTrigger>
                    </FormControl>
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
                  <FormMessage />
                </FormItem>
              </FormField>

              <!-- 日期选择（可选） -->
              <FormField v-slot="{ componentField, value }" name="date">
                <FormItem class="flex flex-col">
                  <FormLabel>{{ t('comp.date') }}</FormLabel>
                  <Popover>
                    <PopoverTrigger as-child>
                      <FormControl>
                        <Button
                          variant="outline"
                          :class="cn('w-full justify-start text-left font-normal', !value && 'text-muted-foreground')"
                        >
                          <CalendarIcon />
                          {{ formatDate(value) || t('comp.pickDate') }}
                        </Button>
                      </FormControl>
                    </PopoverTrigger>
                    <PopoverContent class="w-auto p-0">
                      <Calendar v-bind="componentField" initial-focus />
                    </PopoverContent>
                  </Popover>
                </FormItem>
              </FormField>
            </div>

            <Separator />

            <!-- 单选（有默认值） -->
            <FormField v-slot="{ componentField }" name="plan">
              <FormItem class="space-y-3">
                <FormLabel>{{ t('comp.plan') }}</FormLabel>
                <FormControl>
                  <RadioGroup v-bind="componentField" class="grid gap-3 sm:grid-cols-3">
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
                </FormControl>
              </FormItem>
            </FormField>

            <!-- 文本域（可选，限长 200） -->
            <FormField v-slot="{ componentField }" name="bio">
              <FormItem>
                <FormLabel>{{ t('comp.bio') }}</FormLabel>
                <FormControl>
                  <Textarea v-bind="componentField" :placeholder="t('comp.bioPlaceholder')" rows="3" />
                </FormControl>
                <FormMessage />
              </FormItem>
            </FormField>

            <Separator />

            <!-- 开关 -->
            <FormField v-slot="{ value, handleChange }" name="notify">
              <FormItem class="flex flex-row items-center justify-between rounded-lg border p-4">
                <div class="space-y-0.5">
                  <FormLabel>{{ t('comp.notify') }}</FormLabel>
                  <p class="text-muted-foreground text-sm">
                    {{ t('comp.notifyDesc') }}
                  </p>
                </div>
                <FormControl>
                  <Switch :model-value="value" @update:model-value="handleChange" />
                </FormControl>
              </FormItem>
            </FormField>

            <!-- 复选（必勾） -->
            <FormField v-slot="{ value, handleChange }" name="agree">
              <FormItem>
                <div class="flex items-start gap-3">
                  <FormControl>
                    <Checkbox
                      id="f-agree"
                      :model-value="value"
                      @update:model-value="handleChange"
                    />
                  </FormControl>
                  <FormLabel for="f-agree" class="text-muted-foreground text-sm font-normal leading-relaxed" required>
                    {{ t('comp.agree') }}
                  </FormLabel>
                </div>
                <FormMessage />
              </FormItem>
            </FormField>
          </CardContent>
          <CardFooter class="gap-2">
            <Button type="submit">
              <Send />{{ t('comp.submit') }}
            </Button>
            <Button type="button" variant="outline" @click="onReset">
              {{ t('common.reset') }}
            </Button>
          </CardFooter>
        </Card>
      </form>

      <!-- 实时预览 -->
      <Card class="lg:sticky lg:top-4">
        <CardHeader>
          <CardTitle>{{ t('comp.preview') }}</CardTitle>
          <CardDescription>{{ t('comp.previewDesc') }}</CardDescription>
        </CardHeader>
        <CardContent>
          <pre class="bg-muted/50 overflow-x-auto rounded-lg p-3 text-xs leading-relaxed">{{ JSON.stringify(previewData, null, 2) }}</pre>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
