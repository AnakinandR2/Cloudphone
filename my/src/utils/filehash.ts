// 上传去重/秒传用的文件哈希工具。
// 全量 md5 + slice_md5（前 256KB 的 md5）+ size。
// 大文件流式分块读（FileReader 按 chunk），不一次性载入内存。
import SparkMD5 from 'spark-md5'

const CHUNK_SIZE = 2 * 1024 * 1024 // 2MB 分块读
const SLICE_SIZE = 256 * 1024 // 秒传持有证明：前 256KB

export interface FileHash {
  md5: string
  sliceMd5: string
  size: number
}

function readChunk(blob: Blob): Promise<ArrayBuffer> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as ArrayBuffer)
    reader.onerror = () => reject(reader.error ?? new Error('read error'))
    reader.readAsArrayBuffer(blob)
  })
}

// 流式计算全量 md5 与前 256KB 的 slice_md5。
export async function hashFile(file: File): Promise<FileHash> {
  const size = file.size

  // slice_md5：仅取前 256KB（或更小文件的全部）。
  const sliceSpark = new SparkMD5.ArrayBuffer()
  sliceSpark.append(await readChunk(file.slice(0, Math.min(SLICE_SIZE, size))))
  const sliceMd5 = sliceSpark.end()

  // 全量 md5：分块增量。
  const fullSpark = new SparkMD5.ArrayBuffer()
  for (let offset = 0; offset < size; offset += CHUNK_SIZE) {
    const chunk = file.slice(offset, Math.min(offset + CHUNK_SIZE, size))
    fullSpark.append(await readChunk(chunk))
  }
  const md5 = fullSpark.end()

  return { md5, sliceMd5, size }
}
