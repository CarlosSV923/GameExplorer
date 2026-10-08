import { useQuery } from '@tanstack/react-query'

import { createPortContext } from '@/shared/kernel/ports'

export interface HealthCheck {
  name: string
  ok: boolean
  message?: string
}

export interface Health {
  status: 'ok' | 'degraded'
  checks: HealthCheck[]
}

/** Startup diagnostics (RNF-03): e.g. the library is not writable. */
export interface SystemPorts {
  health(): Promise<Health>
}

export const [SystemContext, useSystemPorts] = createPortContext<SystemPorts>('System')

export function useHealth() {
  const ports = useSystemPorts()
  return useQuery({
    queryKey: ['health'],
    queryFn: () => ports.health(),
    refetchInterval: 60_000,
  })
}
