import { createPortContext } from '@/shared/kernel/ports'

export interface Session {
  expiresAt: string
}

/** Single shared password and a remembered session (RF-50). */
export interface IdentityPorts {
  /** The current session; rejects with `unauthorized` without one. */
  session(): Promise<Session>
  /** Rejects with `unauthorized` (wrong password) or `rateLimited`. */
  login(password: string): Promise<void>
  logout(): Promise<void>
}

export const [IdentityContext, useIdentityPorts] = createPortContext<IdentityPorts>('Identity')
