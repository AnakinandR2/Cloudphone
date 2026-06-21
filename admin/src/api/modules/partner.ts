import type {
  Partner,
  PartnerClickListResult,
  PartnerCreate,
  PartnerListParams,
  PartnerListResult,
  PartnerUpdate,
} from '@/types/partner'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

// 代理IP合作商管理（运营侧，需 staff 权限 partner:view / partner:manage）。
export default {
  list: (params: PartnerListParams) =>
    api.get<unknown, R<PartnerListResult>>('admin/partners', { params }),

  create: (data: PartnerCreate) =>
    api.post<unknown, R<Partner>>('admin/partners', data),

  update: (id: number, data: PartnerUpdate) =>
    api.put<unknown, R<Partner>>(`admin/partners/${id}`, data),

  delete: (id: number) => api.delete<unknown, R<null>>(`admin/partners/${id}`),

  // 上传图片（logo/配图）到 S3，返回 { url }。multipart，不手动设 Content-Type。
  upload: (file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    return api.post<unknown, R<{ url: string }>>('admin/partners/upload', fd, { timeout: 120000 })
  },

  // 点击明细分页（趋势/明细查看）。
  clicks: (id: number, params: { page: number, size: number }) =>
    api.get<unknown, R<PartnerClickListResult>>(`admin/partners/${id}/clicks`, { params }),
}
