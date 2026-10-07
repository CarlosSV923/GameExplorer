export type DataSource = 'api' | 'demo'

/**
 * Resolves which adapter set the composition root wires in.
 * Anything other than an explicit "demo" falls back to the real API.
 */
export function resolveDataSource(raw: string | undefined): DataSource {
  return raw === 'demo' ? 'demo' : 'api'
}

export const dataSource: DataSource = resolveDataSource(import.meta.env.VITE_DATA_SOURCE)
