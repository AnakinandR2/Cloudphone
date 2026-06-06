export interface Note {
  id: number
  user_id: number
  title: string
  content: string
  created_at: string
  updated_at: string
}

export interface NoteCreate {
  title: string
  content: string
}

export interface NoteUpdate {
  title?: string
  content?: string
}

export interface NoteListParams {
  page: number
  size: number
  title?: string
}

export interface NoteListResult {
  list: Note[]
  total: number
}
