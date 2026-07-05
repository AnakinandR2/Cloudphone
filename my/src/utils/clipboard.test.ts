import { afterEach, describe, expect, it, vi } from 'vitest'
import { copyText } from './clipboard'

// jsdom 不实现 execCommand，测试里按需注入 mock。
function stubExecCommand(result: boolean) {
  const fn = vi.fn().mockReturnValue(result)
  document.execCommand = fn as unknown as typeof document.execCommand
  return fn
}

// CP-0046 / #41：复制须兼容非 HTTPS 环境（内网 http 下 navigator.clipboard 不可用）。
describe('copyText', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
    // @ts-expect-error 清理注入的 execCommand
    delete document.execCommand
  })

  it('安全上下文且文档聚焦 → 走 Clipboard API', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { clipboard: { writeText } })
    Object.defineProperty(window, 'isSecureContext', { value: true, configurable: true })
    vi.spyOn(document, 'hasFocus').mockReturnValue(true)

    expect(await copyText('secret-key')).toBe(true)
    expect(writeText).toHaveBeenCalledWith('secret-key')
  })

  it('非安全上下文（http）→ 回退 execCommand，仍能复制', async () => {
    Object.defineProperty(window, 'isSecureContext', { value: false, configurable: true })
    const writeText = vi.fn()
    vi.stubGlobal('navigator', { clipboard: { writeText } })
    const exec = stubExecCommand(true)

    expect(await copyText('secret-key')).toBe(true)
    expect(writeText).not.toHaveBeenCalled() // 未走 Clipboard API
    expect(exec).toHaveBeenCalledWith('copy') // 走了回退
  })

  it('clipboard 不可用 → 回退 execCommand', async () => {
    Object.defineProperty(window, 'isSecureContext', { value: true, configurable: true })
    vi.stubGlobal('navigator', {}) // 无 clipboard
    const exec = stubExecCommand(true)

    expect(await copyText('x')).toBe(true)
    expect(exec).toHaveBeenCalledWith('copy')
  })

  it('回退 execCommand 返回 false → copyText 返回 false', async () => {
    Object.defineProperty(window, 'isSecureContext', { value: false, configurable: true })
    vi.stubGlobal('navigator', {})
    stubExecCommand(false)

    expect(await copyText('x')).toBe(false)
  })
})
