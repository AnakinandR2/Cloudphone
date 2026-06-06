/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL: string
  readonly VITE_API_TARGET: string
  readonly VITE_APP_TITLE: string
  readonly VITE_APP_MOCK: string
  readonly VITE_APP_STORAGE_PREFIX: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
