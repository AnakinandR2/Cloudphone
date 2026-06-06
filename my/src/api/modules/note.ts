import type {
  Note,
  NoteCreate,
  NoteListParams,
  NoteListResult,
  NoteUpdate,
} from '@/types/note'
import api from '../index'

interface R<T> { code: number, message: string, data: T }

// 笔记：归属当前登录前台用户，需登录，完整增删改查
export default {
  list: (params: NoteListParams) =>
    api.get<unknown, R<NoteListResult>>('note/list', { params }),

  detail: (id: number) => api.get<unknown, R<Note>>(`note/${id}`),

  create: (data: NoteCreate) => api.post<unknown, R<Note>>('note/create', data),

  update: (id: number, data: NoteUpdate) =>
    api.put<unknown, R<Note>>(`note/update/${id}`, data),

  delete: (id: number) => api.delete<unknown, R<null>>(`note/delete/${id}`),
}
