import type { SystemPorts } from '../../application/ports'

/** Nothing to diagnose without a server. */
export function createSystemDemo(): SystemPorts {
  return { health: () => Promise.resolve({ status: 'ok', checks: [] }) }
}
