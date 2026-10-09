import type { IdentityPorts } from '../../application/ports'

/** The demo has no login (RF-65): there is always a session. */
export function createIdentityDemo(): IdentityPorts {
  const session = { expiresAt: new Date(Date.now() + 365 * 24 * 3600 * 1000).toISOString() }
  return {
    session: () => Promise.resolve(session),
    login: () => Promise.resolve(),
    logout: () => Promise.resolve(),
  }
}
