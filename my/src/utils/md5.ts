// 自包含 MD5（RFC 1321）+ 文件/分片哈希工具——中台分片上传要求 contentMd5，
// 而浏览器 SubtleCrypto 不提供 MD5，故内置一份纯实现，无需额外依赖。
// 参考 mcn-oneforall 应用上传弹窗的同款实现。

/** 计算 ArrayBuffer 的十六进制 MD5。 */
export function md5Buffer(buffer: ArrayBuffer): string {
  const T = new Int32Array(64)
  for (let i = 0; i < 64; i++)
    T[i] = (Math.abs(Math.sin(i + 1)) * 0x100000000) | 0
  let a = 0x67452301
  let b = 0xEFCDAB89
  let c = 0x98BADCFE
  let d = 0x10325476
  const bytes = new Uint8Array(buffer)
  const len = bytes.length
  const extra = ((55 - len) % 64 + 64) % 64
  const padded = new Uint8Array(len + extra + 9)
  padded.set(bytes)
  padded[len] = 0x80
  const view = new DataView(padded.buffer)
  view.setUint32(padded.length - 8, (len * 8) & 0xFFFFFFFF, true)
  view.setUint32(padded.length - 4, Math.floor(len / 0x20000000), true)

  const S = [7, 12, 17, 22, 5, 9, 14, 20, 4, 11, 16, 23, 6, 10, 15, 21]
  for (let i = 0; i < padded.length; i += 64) {
    const M = new Int32Array(16)
    for (let j = 0; j < 16; j++)
      M[j] = view.getInt32(i + j * 4, true)
    let [aa, bb, cc, dd] = [a, b, c, d]
    for (let j = 0; j < 64; j++) {
      let f: number
      let g: number
      if (j < 16) {
        f = (b & c) | (~b & d)
        g = j
      }
      else if (j < 32) {
        f = (d & b) | (~d & c)
        g = (5 * j + 1) % 16
      }
      else if (j < 48) {
        f = b ^ c ^ d
        g = (3 * j + 5) % 16
      }
      else {
        f = c ^ (b | ~d)
        g = (7 * j) % 16
      }
      const tmp = d
      d = c
      c = b
      const s = S[(j < 16 ? 0 : j < 32 ? 4 : j < 48 ? 8 : 12) + (j % 4)]
      const x = (aa + f + M[g] + T[j]) | 0
      b = (b + ((x << s) | (x >>> (32 - s)))) | 0
      aa = tmp
    }
    a = (a + aa) | 0
    b = (b + bb) | 0
    c = (c + cc) | 0
    d = (d + dd) | 0
  }
  const out = new DataView(new ArrayBuffer(16))
  ;[a, b, c, d].forEach((v, i) => out.setInt32(i * 4, v, true))
  return Array.from(new Uint8Array(out.buffer)).map(x => x.toString(16).padStart(2, '0')).join('')
}

/** 计算 Blob（分片或整文件）的十六进制 MD5。 */
export function computeBlobMd5(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = e => resolve(md5Buffer(e.target!.result as ArrayBuffer))
    reader.onerror = () => reject(new Error('MD5 读取失败'))
    reader.readAsArrayBuffer(blob)
  })
}

/** 分块读取大文件计算整体 MD5（默认 2MB 一段，避免一次性读入超大文件）。 */
export function computeFileMd5(file: File, chunkSize = 2 * 1024 * 1024): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    const parts: ArrayBuffer[] = []
    let offset = 0

    function readNext() {
      if (offset >= file.size) {
        const total = parts.reduce((s, p) => s + p.byteLength, 0)
        const merged = new Uint8Array(total)
        let pos = 0
        for (const p of parts) {
          merged.set(new Uint8Array(p), pos)
          pos += p.byteLength
        }
        resolve(md5Buffer(merged.buffer))
        return
      }
      reader.readAsArrayBuffer(file.slice(offset, Math.min(offset + chunkSize, file.size)))
    }
    reader.onload = (e) => {
      parts.push(e.target!.result as ArrayBuffer)
      offset += chunkSize
      readNext()
    }
    reader.onerror = () => reject(new Error('MD5 读取失败'))
    readNext()
  })
}
