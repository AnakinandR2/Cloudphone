<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

const route = useRoute()
const { t } = useI18n()
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle>{{ route.meta.title ? t(route.meta.title) : t('blank.title') }}</CardTitle>
      <CardDescription>{{ t('blank.desc') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <p class="text-muted-foreground text-sm">
        {{ t('blank.current', { path: route.path }) }}
      </p>
    </CardContent>
  </Card>
</template>
