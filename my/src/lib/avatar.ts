/** 与基础组件页头像占位样式一致 */
export const avatarFallbackClass = 'bg-primary/10 text-primary font-medium'

/** 取用户 ID 的首字作为头像文字 */
export function getUserIdInitial(id: number): string {
  if (!id)
    return 'U'
  return String(id)[0]
}
