<script setup lang="ts">
import { StreamLanguage } from '@codemirror/language'
import { lua } from '@codemirror/legacy-modes/mode/lua'
import { EditorState } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { basicSetup } from 'codemirror'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = defineProps<{ placeholder?: string }>()
const model = defineModel<string>({ default: '' })

const host = ref<HTMLDivElement | null>(null)
let view: EditorView | null = null
let syncing = false

onMounted(() => {
  if (!host.value)
    return
  const updateListener = EditorView.updateListener.of((u) => {
    if (u.docChanged) {
      syncing = true
      model.value = u.state.doc.toString()
      syncing = false
    }
  })
  view = new EditorView({
    parent: host.value,
    state: EditorState.create({
      doc: model.value,
      extensions: [
        basicSetup,
        StreamLanguage.define(lua),
        updateListener,
        EditorView.theme({
          '&': { fontSize: '13px', backgroundColor: 'transparent' },
          '.cm-content': { fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' },
          '&.cm-focused': { outline: 'none' },
        }),
        EditorView.contentAttributes.of({ 'aria-label': props.placeholder ?? 'Lua' }),
      ],
    }),
  })
})

// 外部值变化（如上传文件灌入）时同步到编辑器。
watch(model, (v) => {
  if (syncing || !view)
    return
  if (v !== view.state.doc.toString()) {
    view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: v ?? '' } })
  }
})

onBeforeUnmount(() => {
  view?.destroy()
  view = null
})
</script>

<template>
  <div ref="host" class="overflow-hidden rounded-md border bg-background [&_.cm-editor]:max-h-80 [&_.cm-editor]:min-h-40 [&_.cm-scroller]:overflow-auto" />
</template>
