import { ref } from 'vue'

/** 轻量顶部加载进度条状态（模拟载入） */
export const progressValue = ref(0)
export const progressVisible = ref(false)

let timer: ReturnType<typeof setInterval> | null = null

export function startProgress() {
  if (timer) {
    clearInterval(timer)
  }
  progressVisible.value = true
  progressValue.value = 8
  // 缓慢趋近 90%，模拟请求中
  timer = setInterval(() => {
    const remain = 90 - progressValue.value
    if (remain > 0) {
      progressValue.value += Math.max(0.5, remain * 0.12)
    }
  }, 160)
}

export function doneProgress() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
  progressValue.value = 100
  window.setTimeout(() => {
    progressVisible.value = false
    progressValue.value = 0
  }, 240)
}
