export interface ExampleItem {
  id: number
  title: string
  content: string
  created_at: string
  updated_at: string
}

export interface ExampleItemCreate {
  title: string
  content: string
}

export interface ExampleItemUpdate {
  title?: string
  content?: string
}

export interface ExampleListParams {
  page: number
  size: number
  title?: string
}

export interface ExampleListResult {
  list: ExampleItem[]
  total: number
}
