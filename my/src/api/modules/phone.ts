import type {
  AdbInfo,
  AdbWhitelistEntry,
  CloudPhone,
  CloudPhoneCreate,
  CloudPhoneListParams,
  CloudPhoneListResult,
  CloudPhoneUpdate,
  InstalledApp,
  PhoneFile,
  Tag,
  WebRTCAuth,
} from '@/types/phone'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

// 我的云手机：归属当前登录前台用户，需登录，完整增删改查 + 实例操作
export default {
  list: (params: CloudPhoneListParams) =>
    api.get<unknown, R<CloudPhoneListResult>>('phone/list', { params }),

  detail: (id: number) => api.get<unknown, R<CloudPhone>>(`phone/${id}`),

  // ---- 标签 ----
  listTags: () => api.get<unknown, R<Tag[]>>('phone/tags'),
  setTags: (ids: number[], tags: Tag[]) => api.post<unknown, R<null>>('phone/tags', { ids, tags }),

  create: (data: CloudPhoneCreate) => api.post<unknown, R<CloudPhone>>('phone/create', data),

  update: (id: number, data: CloudPhoneUpdate) =>
    api.put<unknown, R<CloudPhone>>(`phone/update/${id}`, data),

  delete: (id: number) => api.delete<unknown, R<null>>(`phone/delete/${id}`),

  // ---- 实例操作（中台）----
  power: (id: number, operation: '开机' | '关机') =>
    api.post<unknown, R<null>>(`phone/${id}/power`, { operation }),

  restart: (id: number) => api.post<unknown, R<null>>(`phone/${id}/restart`),

  reset: (id: number, imageId?: string) =>
    api.post<unknown, R<null>>(`phone/${id}/reset`, imageId ? { imageId } : {}),

  // 一键新机（一键刷新）：擦数据 + 重装系统（保留代理）
  newDevice: (id: number) => api.post<unknown, R<null>>(`phone/${id}/new-device`),

  destroy: (id: number) => api.post<unknown, R<null>>(`phone/${id}/destroy`),

  // ---- 远程控制（WebRTC）----
  webrtcAuth: (id: number) => api.post<unknown, R<WebRTCAuth>>(`phone/${id}/webrtc-auth`),

  webrtcState: (id: number) =>
    api.get<unknown, R<{ in_webrtc: boolean }>>(`phone/${id}/webrtc-state`),

  screenshot: (id: number, format: 'png' | 'jpeg' = 'png') =>
    api.post<unknown, R<unknown>>(`phone/${id}/screenshot`, { format }),

  volume: (id: number, volume: number) =>
    api.post<unknown, R<null>>(`phone/${id}/volume`, { volume }),

  rotate: (id: number, orientation: 'landscape' | 'portrait') =>
    api.post<unknown, R<null>>(`phone/${id}/rotate`, { orientation }),

  shake: (id: number) => api.post<unknown, R<null>>(`phone/${id}/shake`),

  // ---- 文件管理 ----
  // 列目录（path 缺省时后端默认 /sdcard）
  fileList: (id: number, path?: string) =>
    api.post<unknown, R<PhoneFile[]>>(`phone/${id}/files/list`, { path }),

  // 下载单个文件，返回二进制 Blob（responseType=blob 绕过 {code,message,data} 解包）
  fileDownload: (id: number, path: string) =>
    api.request<unknown, Blob>({
      url: `phone/${id}/files/download`,
      method: 'post',
      data: { path },
      responseType: 'blob',
    }),

  // 删除一个或多个文件（透传中台 file-delete）
  fileDelete: (id: number, paths: string[]) =>
    api.post<unknown, R<null>>(`phone/${id}/files/delete`, { paths }),

  // 上传一个或多个本地文件到 folderPath（multipart；不手动设 Content-Type，由 axios 自动补 boundary）
  fileUpload: (id: number, folderPath: string, files: File[], onProgress?: (percent: number) => void) => {
    const fd = new FormData()
    if (folderPath)
      fd.append('folderPath', folderPath)
    files.forEach(f => fd.append('files', f))
    return api.post<unknown, R<null>>(`phone/${id}/files/upload`, fd, {
      timeout: 300000,
      onUploadProgress: (e) => {
        if (onProgress && e.total)
          onProgress(Math.round((e.loaded / e.total) * 100))
      },
    })
  },

  // ---- 应用管理 ----
  apps: (id: number) => api.get<unknown, R<InstalledApp[]>>(`phone/${id}/apps`),

  installApp: (id: number, appIds: number[]) =>
    api.post<unknown, R<null>>(`phone/${id}/apps/install`, { appIds }),

  // 应用管理目前不做启停（start/stop/kill-all），只安装/卸载。
  uninstallApp: (id: number, payload: { appIds?: number[], packageNames?: string[] }) =>
    api.post<unknown, R<null>>(`phone/${id}/apps/uninstall`, payload),

  // ---- ADB（中台 §3.1 operate / §3.2 whitelist / §2.16 page 连接信息）----
  adbInfo: (id: number) => api.get<unknown, R<AdbInfo>>(`phone/${id}/adb`),

  adbEnable: (id: number, payload: { whiteIp?: string[], ttl?: number }) =>
    api.post<unknown, R<AdbInfo>>(`phone/${id}/adb/enable`, payload),

  adbDisable: (id: number) => api.post<unknown, R<null>>(`phone/${id}/adb/disable`),

  adbWhitelist: (id: number) =>
    api.get<unknown, R<AdbWhitelistEntry[]>>(`phone/${id}/adb/whitelist`),

  adbUpdateWhitelist: (id: number, whiteIp: string[]) =>
    api.post<unknown, R<null>>(`phone/${id}/adb/whitelist`, { whiteIp }),
}
