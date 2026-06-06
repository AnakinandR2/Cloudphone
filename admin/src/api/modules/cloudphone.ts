import type {
  AppListParams,
  AppListResult,
  AvailZone,
  BootPlan,
  ImageInfo,
  PhoneSpec,
  PlanImage,
  SpecKind,
  VirtualMachine,
  VirtualSpec,
  VMListParams,
  VMStatus,
} from '@/types/cloudphone'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

export default {
  /** 可用区列表 */
  listZones: () => api.get<unknown, R<AvailZone[]>>('cloudphone/zones'),

  /**
   * 规格列表（kind=phone 默认 PhoneSpec[]；kind=vm VirtualSpec[]）。
   *  不传 zoneId = 聚合全部可用区，每条带 zoneId/zoneName 列。
   */
  listSpecs: (kind: SpecKind = 'phone', zoneId?: number) =>
    api.get<unknown, R<(PhoneSpec | VirtualSpec)[]>>('cloudphone/specs', {
      params: { kind, ...(zoneId ? { zoneId } : {}) },
    }),

  /** 云主机列表（过滤可选） */
  listVMs: (params: VMListParams = {}) =>
    api.get<unknown, R<VirtualMachine[]>>('cloudphone/vms', { params }),

  /** 云主机状态枚举 */
  listVMStatuses: () =>
    api.get<unknown, R<VMStatus[]>>('cloudphone/vm-statuses'),

  /** 镜像列表 */
  listImages: () => api.get<unknown, R<ImageInfo[]>>('cloudphone/images'),

  /** 按云主机规格查可用云手机套餐（规格）。供云主机行操作用。 */
  listBootPlans: (specId: number) =>
    api.get<unknown, R<BootPlan[]>>('cloudphone/boot-plans', { params: { specId } }),

  /** 按云主机规格查可用镜像（套餐镜像并集）。供云主机行操作用。 */
  listSpecImages: (specId: number) =>
    api.get<unknown, R<PlanImage[]>>('cloudphone/spec-images', { params: { specId } }),

  /** 应用市场（分页） */
  listApps: (params: AppListParams = {}) =>
    api.get<unknown, R<AppListResult>>('cloudphone/apps', { params }),
}
