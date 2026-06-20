<script setup lang="ts">
// 文章读者反馈（仅客户端）：赞踩 + 文本留言。命中本站 /_content/feedback/*，密钥不进浏览器。
// 登录用户：visitor_id=u:<id>，并把 {user_id,nickname,phone} 经 meta 上送（仅后台明细识别，前台不展示）。
// 联系方式输入框不预填——避免让用户误以为我们擅自获取其手机号。
import type { FeedbackSummary } from '~/types/content'

const props = withDefaults(
  defineProps<{ slug: string; space?: string; vote?: boolean; feedback?: boolean; rating?: boolean }>(),
  { space: 'help', vote: false, feedback: false, rating: false },
)

const { t } = useGp()
const authUser = useAuthUser()
const api = useFeedbackApi(props.space, () => props.slug)

const visitorId = ref('')
const summary = ref<FeedbackSummary | null>(null)
const myVote = computed(() => summary.value?.my_reaction?.vote ?? 0)
const meta = () => buildMeta(authUser.value)

// 赞踩
const voteBusy = ref(false)
const votedThanks = ref(false)

// 留言
const reportOpen = ref(false)
const content = ref('')
const contact = ref('')
const submitting = ref(false)
const feedbackDone = ref(false)
const formError = ref('')

onMounted(async () => {
  visitorId.value = ensureVisitorId(authUser.value)
  try {
    summary.value = await api.fetchSummary(visitorId.value)
  } catch {
    /* 拉取失败：静默降级，仍可提交，只是不显示历史计数 */
  }
})

async function onVote(clicked: 1 | -1) {
  if (voteBusy.value || !visitorId.value) return
  const prev = summary.value
  const newVote = nextVote(myVote.value, clicked)
  voteBusy.value = true
  try {
    summary.value = await api.submitReaction({ vote: newVote, visitorId: visitorId.value, meta: meta() })
    votedThanks.value = newVote !== 0
  } catch {
    summary.value = prev // 回滚乐观态
  } finally {
    voteBusy.value = false
  }
}

async function onSubmit() {
  formError.value = ''
  const c = content.value.trim()
  if (!c) {
    formError.value = t.value.feedback.emptyError
    return
  }
  submitting.value = true
  try {
    summary.value = await api.submitFeedback({
      content: c,
      contact: contact.value.trim() || undefined,
      visitorId: visitorId.value,
      meta: meta(),
    })
    feedbackDone.value = true
  } catch {
    formError.value = t.value.feedback.error
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <section class="af">
    <!-- 赞踩 -->
    <div v-if="vote" class="af-vote">
      <span class="af-vote__q">{{ t.feedback.title }}</span>
      <div class="af-vote__btns">
        <button type="button" class="af-btn" :class="{ active: myVote === 1 }" :disabled="voteBusy" @click="onVote(1)">
          <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path fill="currentColor" d="M2 21h2a1 1 0 0 0 1-1v-9a1 1 0 0 0-1-1H2zm5-1a2 2 0 0 0 2 2h8.5a2 2 0 0 0 1.96-1.6l1.4-7A2 2 0 0 0 18.9 11H14V5.5A2.5 2.5 0 0 0 11.5 3l-.5 1.2L7 10.6z" /></svg>
          {{ t.feedback.helpful }}<em v-if="summary">{{ summary.up_count }}</em>
        </button>
        <button type="button" class="af-btn" :class="{ active: myVote === -1 }" :disabled="voteBusy" @click="onVote(-1)">
          <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true"><path fill="currentColor" d="M22 3h-2a1 1 0 0 0-1 1v9a1 1 0 0 0 1 1h2zm-5 1a2 2 0 0 0-2-2H6.5a2 2 0 0 0-1.96 1.6l-1.4 7A2 2 0 0 0 5.1 13H10v5.5A2.5 2.5 0 0 0 12.5 21l.5-1.2L17 13.4z" /></svg>
          {{ t.feedback.notHelpful }}<em v-if="summary">{{ summary.down_count }}</em>
        </button>
        <span v-if="votedThanks" class="af-thanks">{{ t.feedback.thanksVote }}</span>
      </div>
    </div>

    <!-- 留言 -->
    <div v-if="feedback" class="af-report">
      <button v-if="!reportOpen && !feedbackDone" type="button" class="af-link" @click="reportOpen = true">
        {{ t.feedback.reportToggle }}
      </button>
      <form v-else-if="!feedbackDone" class="af-form" @submit.prevent="onSubmit">
        <label class="af-form__label">{{ t.feedback.reportTitle }}</label>
        <textarea v-model="content" class="af-input" rows="4" :placeholder="t.feedback.placeholder" />
        <input v-model="contact" class="af-input" :placeholder="t.feedback.contact" />
        <div class="af-form__row">
          <button type="submit" class="af-submit" :disabled="submitting">
            {{ submitting ? t.feedback.submitting : t.feedback.submit }}
          </button>
          <span v-if="formError" class="af-error">{{ formError }}</span>
        </div>
      </form>
      <p v-else class="af-thanks af-thanks--block">{{ t.feedback.thanksFeedback }}</p>
    </div>
  </section>
</template>

<style scoped>
.af {
  margin-top: 48px;
  padding-top: 28px;
  border-top: 1px solid rgb(var(--border));
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.af-vote { display: flex; align-items: center; flex-wrap: wrap; gap: 14px; }
.af-vote__q { font-weight: 600; color: rgb(var(--fg)); }
.af-vote__btns { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.af-btn {
  display: inline-flex; align-items: center; gap: 7px;
  padding: 7px 14px; border-radius: 999px;
  border: 1px solid rgb(var(--border)); background: rgb(var(--bg-sunken));
  color: rgb(var(--fg-muted)); font-size: 13.5px; font-weight: 600;
  cursor: pointer; transition: all 0.15s ease;
}
.af-btn:hover:not(:disabled) { color: rgb(var(--fg)); border-color: rgb(var(--border-strong)); }
.af-btn:disabled { opacity: 0.6; cursor: default; }
.af-btn.active { background: rgb(var(--accent) / 0.12); border-color: var(--accent-color); color: var(--accent-color); }
.af-btn em { font-style: normal; font-variant-numeric: tabular-nums; opacity: 0.8; }
.af-thanks { color: var(--accent-color); font-size: 13.5px; font-weight: 600; }
.af-thanks--block { margin: 0; }

.af-link {
  background: none; border: none; padding: 0; cursor: pointer;
  color: rgb(var(--fg-muted)); font-size: 13px; text-decoration: underline; text-underline-offset: 3px;
}
.af-link:hover { color: var(--accent-color); }

.af-form { display: flex; flex-direction: column; gap: 10px; max-width: 520px; }
.af-form__label { font-weight: 600; color: rgb(var(--fg)); font-size: 14px; }
.af-input {
  width: 100%; padding: 10px 12px; border-radius: 10px;
  border: 1px solid rgb(var(--border)); background: rgb(var(--bg));
  color: rgb(var(--fg)); font: inherit; font-size: 14px; resize: vertical;
}
.af-input:focus { outline: none; border-color: var(--accent-color); }
.af-form__row { display: flex; align-items: center; gap: 12px; }
.af-submit {
  padding: 8px 18px; border-radius: 10px; border: none;
  background: var(--accent-color); color: rgb(var(--accent-fg));
  font-weight: 600; font-size: 13.5px; cursor: pointer; transition: background 0.15s ease;
}
.af-submit:hover:not(:disabled) { background: var(--accent-strong-color); }
.af-submit:disabled { opacity: 0.6; cursor: default; }
.af-error { color: rgb(var(--danger, 220 38 38)); font-size: 13px; }
</style>
