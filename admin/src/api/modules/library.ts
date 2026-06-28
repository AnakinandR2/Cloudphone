import type {
  AdminLibraryGrantRequest,
  AdminLibraryUserView,
  LibraryPricingConfig,
} from '@/types/library'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

export default {
  // 素材库定价配置（GET/PUT /admin/library/pricing）
  getPricing: () => api.get<unknown, R<LibraryPricingConfig>>('admin/library/pricing'),
  savePricing: (d: LibraryPricingConfig) => api.put<unknown, R<LibraryPricingConfig>>('admin/library/pricing', d),

  // 单用户用量 + 订阅（gap-7，可选）
  getUser: (userId: number) => api.get<unknown, R<AdminLibraryUserView>>(`admin/library/users/${userId}`),
  grant: (userId: number, body: AdminLibraryGrantRequest) =>
    api.post<unknown, R<AdminLibraryUserView>>(`admin/library/users/${userId}/grant`, body),
}
