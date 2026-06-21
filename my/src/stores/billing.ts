import { defineStore } from 'pinia'
import { ref } from 'vue'
import billingApi from '@/api/modules/billing'

// 计费侧的轻量全局状态：当前「符合资格但未领取」的试用数量，用于菜单「待领取」徽标。
export const useBillingStore = defineStore('billing', () => {
  const claimableTrials = ref(0)

  // 拉取可领取试用数（claimable=资格满足且未超限）。静默失败，不阻塞 UI。
  async function refreshClaimableTrials() {
    try {
      const { data } = await billingApi.trials()
      claimableTrials.value = (data ?? []).filter(it => it.claimable).length
    }
    catch {
      claimableTrials.value = 0
    }
  }

  return { claimableTrials, refreshClaimableTrials }
})
