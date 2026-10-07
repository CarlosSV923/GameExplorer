/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** "api" talks to the Go backend; "demo" runs fully in the browser (Vercel). */
  readonly VITE_DATA_SOURCE?: 'api' | 'demo'
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
