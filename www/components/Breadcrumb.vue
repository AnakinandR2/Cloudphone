<script setup lang="ts">
interface Crumb {
  label: string
  /** Resolved (localePath) path. Omit for the current page. */
  to?: string
}

const props = defineProps<{ items: Crumb[] }>()

// Absolute URLs for structured data (works in SSR via the request URL).
const reqUrl = useRequestURL()
const origin = computed(() => reqUrl.origin)

const jsonLd = computed(() => ({
  '@context': 'https://schema.org',
  '@type': 'BreadcrumbList',
  itemListElement: props.items.map((c, i) => ({
    '@type': 'ListItem',
    position: i + 1,
    name: c.label,
    ...(c.to ? { item: origin.value + c.to } : {}),
  })),
}))

useHead(() => ({
  script: [
    {
      type: 'application/ld+json',
      innerHTML: JSON.stringify(jsonLd.value),
    },
  ],
}))
</script>

<template>
  <nav class="breadcrumb" aria-label="Breadcrumb">
    <ol>
      <li v-for="(c, i) in items" :key="i">
        <NuxtLink v-if="c.to && i < items.length - 1" :to="c.to" class="breadcrumb__link">{{ c.label }}</NuxtLink>
        <span v-else class="breadcrumb__current" aria-current="page">{{ c.label }}</span>
        <svg
          v-if="i < items.length - 1"
          class="breadcrumb__sep"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        ><polyline points="9 6 15 12 9 18" /></svg>
      </li>
    </ol>
  </nav>
</template>
