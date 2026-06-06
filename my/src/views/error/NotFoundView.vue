<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { Button } from '@/components/ui/button'

const { t } = useI18n()
</script>

<template>
  <div class="flex min-h-screen flex-col items-center justify-center gap-4">
    <h1 class="text-6xl font-bold">
      404
    </h1>
    <p class="text-muted-foreground">
      {{ t('notFound.desc') }}
    </p>
    <Button as-child>
      <RouterLink to="/">
        {{ t('notFound.back') }}
      </RouterLink>
    </Button>
  </div>
</template>
