import type { ApiClient } from '@/shared/api/client'
import { unwrap } from '@/shared/api/result'

import type { SystemPorts } from '../../application/ports'

export function createSystemHttp(client: ApiClient): SystemPorts {
  return {
    health: async () => {
      const h = await unwrap(client.GET('/health'))
      return {
        status: h.status,
        checks: h.checks.map((c) => ({
          name: c.name,
          ok: c.ok,
          ...(c.message ? { message: c.message } : {}),
        })),
      }
    },
  }
}
