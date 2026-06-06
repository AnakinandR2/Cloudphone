import type {
  ExampleItem,
  ExampleItemCreate,
  ExampleItemUpdate,
  ExampleListParams,
  ExampleListResult,
} from '@/types/example'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

export default {
  list: (params: ExampleListParams) =>
    api.get<unknown, R<ExampleListResult>>('example/list', { params }),

  detail: (id: number) => api.get<unknown, R<ExampleItem>>(`example/${id}`),

  create: (data: ExampleItemCreate) =>
    api.post<unknown, R<ExampleItem>>('example/create', data),

  update: (id: number, data: ExampleItemUpdate) =>
    api.put<unknown, R<ExampleItem>>(`example/update/${id}`, data),

  delete: (id: number) => api.delete<unknown, R<null>>(`example/delete/${id}`),
}
