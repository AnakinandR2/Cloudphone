// copyText 复制文本到剪贴板，兼容非 HTTPS 环境（CP-0046 / #41）。
// - 安全上下文（HTTPS / localhost）且文档已聚焦：用 Clipboard API（未聚焦会抛 "Document is not focused"）。
// - 否则（内网 http、navigator.clipboard 不可用、未聚焦）：回退临时 textarea + execCommand，保证仍可复制。
// 返回是否成功，供调用方决定 toast 成功/失败。
export async function copyText(text: string): Promise<boolean> {
  // 1) Clipboard API：仅在安全上下文且文档已聚焦时用。
  try {
    if (navigator.clipboard?.writeText && window.isSecureContext && document.hasFocus()) {
      await navigator.clipboard.writeText(text)
      return true
    }
  }
  catch { /* 落到回退 */ }
  // 2) 回退：临时 textarea 自己抢焦点 + execCommand，兼容非 HTTPS / 文档未聚焦。
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.setAttribute('readonly', '')
    ta.style.position = 'fixed'
    ta.style.top = '0'
    ta.style.left = '0'
    ta.style.width = '1px'
    ta.style.height = '1px'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.focus()
    ta.select()
    ta.setSelectionRange(0, text.length)
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    return ok
  }
  catch {
    return false
  }
}
