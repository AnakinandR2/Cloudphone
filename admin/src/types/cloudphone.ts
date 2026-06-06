/** 云手机资源（只读）类型定义。字段名与后端 JSON 完全一致。 */

/** 可用区 */
export interface AvailZone {
  id: number
  name: string
  regionName: string
  supplier: string
  zoneCode: string
  createTime: string
}

/** 云手机规格（kind=phone） */
export interface PhoneSpec {
  id: number
  name: string
  supplier: string
  core: number
  memory: number
  storage: number
  type: string
  feature: string
  createTime: string
  zoneId: number
  zoneName: string
}

/** 云主机规格（kind=vm） */
export interface VirtualSpec {
  id: number
  name: string
  supplier: string
  core: number
  memory: number
  storage: number
  card: string
  maxPhone: number
  type: string
  feature: string
  hasVM: boolean
  createTime: string
  zoneId: number
  zoneName: string
}

/** 云主机（云虚机） */
/** 云主机 / 云虚机（对应中台 §5.4 /server/page，租户已分配的服务器） */
export interface VirtualMachine {
  id: number
  /** 虚机唯一标识，人类可读编号如 VM00001043（展示用） */
  vmUid: string
  /** 虚机编号（创建云手机时用的 vmId） */
  vmId: string
  vmIp: string
  vmStatus: string
  isMaintain: boolean
  specificationId: number
  specificationName: string
  core: number
  memory: number
  storage: number
  bandwidth: number
  maxStartCount: number
  maxPhone: number
  createPhoneNumber: number
  createTime: string
  expireTime: string
  expired: boolean
}

/** 镜像 */
export interface ImageInfo {
  id: number
  imageId: string
  imageName: string
  imageVersion: string
  /** 0 启用 / 1 禁用 */
  isForbid: number
  downloadUrl: string
  createTime: string
  updateTime: string
}

/** 云手机套餐（按云主机规格可用的「云手机规格」） */
export interface BootPlan {
  id: number
  specId: number
  planName: string
  scenarioName: string
  specName: string
  supplier: string
  resourceUtilization: string
  width: number
  height: number
  fps: number
  // 单台云手机资源（core/memory 常为 null，优先用 min/max）
  core: number
  minCore: number
  maxCore: number
  memory: number
  minMemory: number
  maxMemory: number
  storage: number
  bandwidth: number
  // 宿主云主机总量（不是单台手机的）
  specCore: number
  specMemory: number
  specStorage: number
  specBandwidth: number
  maxPhone: number
  phoneCount: number
}

/** 套餐可用镜像（中台 /img/query/by-plan） */
export interface PlanImage {
  imageId: string
  imageName: string
  androidVersion: string
}

/** 应用市场应用（对应中台 §1.1 /app/page records[]） */
export interface AppInfo {
  /** 字符串化的 Long */
  id: string
  /** 应用 code */
  appCode: string
  appName: string
  packageName: string
  version: string
  iconUrl: string
  /** 已格式化字符串，如 "256MB"（非字节数） */
  fileSize: string
  /** 创建人 */
  createUser: string
  createTime: string
}

/** 规格列表类型 */
export type SpecKind = 'phone' | 'vm'

/** 应用市场分页参数 */
export interface AppListParams {
  appName?: string
  page?: number
  size?: number
}

/** 应用市场分页结果 */
export interface AppListResult {
  list: AppInfo[]
  total: number
}

/** 云主机列表过滤参数 */
export interface VMListParams {
  zoneId?: number
  status?: string
  vmSource?: string
}

/** 云主机状态枚举项：每个对象一个键值对 { "<code>": "<desc>" } */
export type VMStatus = Record<string, string>
