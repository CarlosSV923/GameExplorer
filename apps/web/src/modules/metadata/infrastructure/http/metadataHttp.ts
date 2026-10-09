import type { ApiClient } from '@/shared/api/client'
import { unwrap } from '@/shared/api/result'

import type { MetadataPorts } from '../../application/ports'

export function createMetadataHttp(client: ApiClient, baseUrl = '/api'): MetadataPorts {
  return {
    searchGames: (q, platformId, limit = 10) =>
      unwrap(
        client.GET('/metadata/games', {
          params: { query: { q, limit, ...(platformId === undefined ? {} : { platformId }) } },
        }),
      ),
    status: () => unwrap(client.GET('/metadata/status')),
    imageUrl: (size, imageId) => `${baseUrl}/images/${size}/${encodeURIComponent(imageId)}`,
  }
}
