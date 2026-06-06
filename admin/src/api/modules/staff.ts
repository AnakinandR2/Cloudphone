import type {
  Staff,
  StaffCreate,
  StaffListParams,
  StaffListResult,
  StaffUpdate,
} from '@/types/staff'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

export default {
  list: (params: StaffListParams) =>
    api.get<unknown, R<StaffListResult>>('staff/list', { params }),

  detail: (id: number) => api.get<unknown, R<Staff>>(`staff/${id}`),

  create: (data: StaffCreate) => api.post<unknown, R<Staff>>('staff/create', data),

  update: (id: number, data: StaffUpdate) =>
    api.put<unknown, R<Staff>>(`staff/update/${id}`, data),

  delete: (id: number) => api.delete<unknown, R<null>>(`staff/delete/${id}`),
}
