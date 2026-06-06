<script setup lang="ts">
defineProps<{ type: string }>()
</script>

<template>
  <span class="partner-mark" aria-hidden="true">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <template v-if="type === 'commerce'"><path d="M7 8h10l-1 9H8L7 8z"/><path d="M9 8a3 3 0 0 1 6 0"/><path d="M10 12h4"/></template>
      <template v-else-if="type === 'globe'"><circle cx="12" cy="12" r="7"/><path d="M5 12h14M12 5a10 10 0 0 1 0 14M12 5a10 10 0 0 0 0 14"/></template>
      <template v-else-if="type === 'anchor'"><path d="M12 5v12"/><circle cx="12" cy="5" r="2"/><path d="M7 11H5v2a7 7 0 0 0 14 0v-2h-2"/><path d="M9 17l-2 2M15 17l2 2"/></template>
      <template v-else-if="type === 'drift'"><path d="M5 15c4-7 8-7 14-5"/><path d="M5 18c5-3 9-2 14 0"/><path d="M6 11c3-2 6-3 10-2"/></template>
      <template v-else-if="type === 'river'"><path d="M6 7c4-3 8 3 12 0"/><path d="M6 12c4-3 8 3 12 0"/><path d="M6 17c4-3 8 3 12 0"/></template>
      <template v-else-if="type === 'hex'"><path d="M12 4l7 4v8l-7 4-7-4V8l7-4z"/><path d="M9 10h6M9 14h6"/></template>
      <template v-else-if="type === 'compass'"><path d="M12 4l5 16-5-3-5 3 5-16z"/><path d="M12 9v8"/></template>
      <template v-else><path d="M8 17h8a4 4 0 0 0 .7-7.9A6 6 0 0 0 5.3 10.4 3.5 3.5 0 0 0 8 17z"/><path d="M10 13h4"/></template>
    </svg>
  </span>
</template>
