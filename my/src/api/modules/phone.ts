import type { AppRef } from '@/types/app'
import type {
  AdbInfo,
  CloudPhone,
  CloudPhoneCreate,
  CloudPhoneListParams,
  CloudPhoneListResult,
  CloudPhoneUpdate,
  InstalledApp,
  PhoneFile,
  PushResult,
  RunLog,
  RuntimeInfo,
  ScriptRunResult,
  ScriptTaskDetail,
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

  // 从素材库选定文件推送到一台或多台云手机（后端用 library 门面直读私有 S3 下发，
  // 字节不经浏览器；允许部分成功，逐台返回 {phone_id, ok, error?}）。
  pushFromLibrary: (phoneIds: number[], fileIds: number[]) =>
    api.post<unknown, R<{ results: PushResult[] }>>('phone/files/push-from-library', {
      phone_ids: phoneIds,
      file_ids: fileIds,
    }),

  // ---- 应用管理 ----
  apps: (id: number) => api.get<unknown, R<InstalledApp[]>>(`phone/${id}/apps`),

  // 按 URL 安装：给中台一个下载 URL（user=素材库私有桶 presigned / market=公有桶）。
  // 异步下发，返回 task_info_list（taskId/instanceId）；前端提示「已下发」。
  installByUrl: (phoneIds: number[], refs: AppRef[]) =>
    api.post<unknown, R<{ task_info_list: { task_id: string, instance_id: string }[] }>>(
      'phone/apps/install-by-url',
      { phone_ids: phoneIds, apps: refs },
    ),

  // 应用管理目前不做启停（start/stop/kill-all），只安装/卸载。
  uninstallApp: (id: number, payload: { appIds?: number[], packageNames?: string[] }) =>
    api.post<unknown, R<null>>(`phone/${id}/apps/uninstall`, payload),

  // ---- 运行日志（中台 §2.9，服务端分页）----
  runLogs: (id: number, params: { page: number, size: number }) =>
    api.get<unknown, R<{ list: RunLog[], total: number }>>(`phone/${id}/run-logs`, { params }),
  // 远控真实开机时长（按中台运行日志，服务端算秒数）
  runtime: (id: number) => api.get<unknown, R<RuntimeInfo>>(`phone/${id}/runtime`),

  // ---- ADB Token 接管（中台 v3.25.9 §3.5 token/enable·disable / §2.6 连接信息）。不管理白名单 ----
  adbInfo: (id: number) => api.get<unknown, R<AdbInfo>>(`phone/${id}/adb`),

  // 开启即续期：再次调用签发全新 token（旧 token 失效）。有效期由中台固定，无需入参。
  adbEnable: (id: number) => api.post<unknown, R<AdbInfo>>(`phone/${id}/adb/enable`),

  adbDisable: (id: number) => api.post<unknown, R<null>>(`phone/${id}/adb/disable`),

  // ---- Root 开关（中台 §3.4.1 update-root，同步生效，需已开机）----
  root: (id: number, enable: boolean) => api.post<unknown, R<null>>(`phone/${id}/root`, { enable }),

  // ---- 自动化脚本最小闭环（中台 §7）----
  // 下发一个 hello-world 示例脚本任务（需已开机），返回任务主键供轮询。
  runHelloScript: (id: number) => api.post<unknown, R<ScriptRunResult>>(`phone/${id}/script/hello`),
  // 查脚本任务状态/报告（轮询用）。
  scriptTask: (id: number, taskId: number | string) =>
    api.get<unknown, R<ScriptTaskDetail>>(`phone/${id}/script/task/${taskId}`),
}
