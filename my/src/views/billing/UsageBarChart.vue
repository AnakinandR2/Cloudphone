<script setup lang="ts">
import { StackedBar } from '@unovis/ts'
import { VisAxis, VisStackedBar, VisTooltip, VisXYContainer } from '@unovis/vue'

// 基于 Unovis（shadcn-vue 图表所用引擎）的柱状图。
interface Point { label: string, value: number }
const props = withDefaults(defineProps<{
  data: Point[]
  height?: number
  color?: string
  unit?: string
}>(), { height: 220, color: '#10b981', unit: '' })

const x = (_d: Point, i: number) => i
const y = (d: Point) => d.value
const xTick = (i: number) => props.data[Math.round(i)]?.label ?? ''
const tooltip = (d: Point) => `${d.label}: ${d.value}${props.unit}`
</script>

<template>
  <VisXYContainer :data="data" :height="height" :margin="{ left: 8, right: 12, top: 8, bottom: 4 }">
    <VisStackedBar :x="x" :y="y" :color="color" :rounded-corners="2" :bar-padding="0.2" />
    <VisAxis type="x" :tick-format="xTick" :num-ticks="6" :grid-line="false" :tick-line="false" />
    <VisAxis type="y" :num-ticks="4" :grid-line="true" :tick-line="false" :domain-line="false" />
    <VisTooltip :triggers="{ [StackedBar.selectors.bar]: tooltip }" />
  </VisXYContainer>
</template>
