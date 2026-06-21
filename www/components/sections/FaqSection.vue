<script setup lang="ts">
// FAQ 板块：从内容中台 faq 空间取问答。
//  - 默认（首页）：取前 6 条。
//  - 传 keywords（如价格页）：仅展示分类或问题文本命中关键词的问答（计费相关）。
const props = defineProps<{ keywords?: string[] }>()
const { t } = useGp()
const { groups, flat } = useFaq()

const items = computed(() => {
  const kws = (props.keywords ?? []).map(k => k.toLowerCase()).filter(Boolean)
  if (!kws.length) return flat.value.slice(0, 6)
  return groups.value.flatMap((g) => {
    const catHit = kws.some(k => g.category.toLowerCase().includes(k))
    return g.items.filter(it => catHit || kws.some(k => it.question.toLowerCase().includes(k)))
  }).slice(0, 8)
})
</script>

<template>
  <section id="help" class="section">
    <div class="container">
      <header style="text-align: center">
        <span class="eyebrow">{{ t.faq.eyebrow }}</span>
        <h2 class="section-title">{{ t.faq.title }}</h2>
      </header>
      <FaqAccordion v-if="items.length" :items="items" />
    </div>
  </section>
</template>
