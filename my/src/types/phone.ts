export interface Tag {
  name: string
  /** 预设色 key（slate/red/orange/amber/green/teal/sky/violet/pink） */
  color: string
}

export interface CloudPhone {
  id: number
  user_id: number
  cp_id: string
  name: string
  status: string
  region: string
  vm_id: string
  image_id: string
  proxy_id: number
  remark: string
  tags?: Tag[]
  adb_enabled?: boolean
  rooted?: boolean
  created_at: string
  updated_at: string
}

export interface CloudPhoneCreate {
  name: string
  region: string
  image_id: string
  proxy_id: number
  remark: string
}

export interface CloudPhoneUpdate {
  name?: string
  status?: string
  region?: string
  image_id?: string
  proxy_id?: number
  remark?: string
}

export interface CloudPhoneListParams {
  page: number
  size: number
  kw?: string
  status?: string
  tag?: string
}

export interface CloudPhoneListResult {
  list: CloudPhone[]
  total: number
}

/** 远程控制（WebRTC）凭证 */
export interface WebRTCAuth {
  cpId: string
  vmId: string
  zoneId: string
  pushStreamUrl: string
  signalUrl: string
  authToken: string
}

/** 云手机内已安装的应用 */
export interface InstalledApp {
  id: number
  packageName: string
  version: string
  md5: string
  appName: string
  iconPath: string
  /** 已格式化字符串，如 "256MB"（中台 §2.13 返回格式化值，非字节数） */
  fileSize: string
}

/** 云手机文件系统中的一个条目（文件 / 目录） */
export interface PhoneFile {
  name: string
  path: string
  /** file / directory / symlink / block / char / fifo / socket */
  type: string
  permission?: string
  owner?: string
  group?: string
  size: number
  accessed?: number
  modified?: number
  changed?: number
  created?: number
}

/** 运行会话日志一条（中台 §2.9，展示值由中台格式化好） */
export interface RunLog {
  rowNo: number
  logNo: string
  cpId: string
  vmUid?: string
  powerOnTime: string
  powerOffTime: string // 运行中返回「运行中」
  duration: string // 如 25m / 3h12m
  powerOffReason: string // 正常关机 / 容器异常 / …
  powerOffReasonCode?: string
  sessionStatus: string // 运行中 / 已关机
  tenantName?: string
  updateTime?: string
}

/** ADB 连接信息（后端从中台 §2.6 详情抽出，enabled 由是否有 token 派生） */
export interface AdbInfo {
  enabled: boolean
  adbAddress: string
  adbToken: string
  adbTokenExpiredAt: string
  status: string
}
