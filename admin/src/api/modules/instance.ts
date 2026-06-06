import type {
  CloudPhone,
  CloudPhoneListParams,
  CloudPhoneListResult,
  Tag,
} from '@/types/instance'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

export default {
  adminList: (params: CloudPhoneListParams) =>
    api.get<unknown, R<CloudPhoneListResult>>('admin/phones/list', { params }),

  tags: () => api.get<unknown, R<Tag[]>>('admin/phones/tags'),

  adminDetail: (id: number) =>
    api.get<unknown, R<CloudPhone>>(`admin/phones/${id}`),

  adminDelete: (id: number) =>
    api.delete<unknown, R<null>>(`admin/phones/delete/${id}`),
}
