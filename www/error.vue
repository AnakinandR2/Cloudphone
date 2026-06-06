<script setup lang="ts">
import type { NuxtError } from '#app'
import { Button } from '~/components/ui/button'

defineProps<{ error: NuxtError }>()

const localePath = useLocalePath()

function goHome() {
  clearError({ redirect: localePath('/') })
}
</script>

<template>
  <div class="grid min-h-screen place-items-center bg-background px-6">
    <div class="text-center">
      <p class="font-heading text-7xl font-bold text-primary">{{ error?.statusCode || 500 }}</p>
      <h1 class="mt-4 text-2xl font-bold">{{ error?.statusMessage || 'Something went wrong' }}</h1>
      <p class="mx-auto mt-2 max-w-md text-muted-foreground">
        The page you are looking for doesn’t exist or has been moved.
      </p>
      <Button class="mt-8" @click="goHome">Back to home</Button>
    </div>
  </div>
</template>
