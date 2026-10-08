import type { ApiClient } from '@/shared/api/client'
import { unwrap } from '@/shared/api/result'

import type { MetadataPorts } from '../../application/ports'

export function createMetadataHttp(client: ApiClient, baseUrl = '/api'): MetadataPorts {
  return {
    searchGames: (q, platformId) =>
      unwrap(
        client.GET('/metadata/games', {
          params: { query: { q, limit: 10, ...(platformId === undefined ? {} : { platformId }) } },
        }),
      ),
    searchPlatforms: (q) => unwrap(client.GET('/metadata/platforms', { params: { query: { q } } })),
    imageUrl: (size, imageId) => `${baseUrl}/images/${size}/${encodeURIComponent(imageId)}`,
  }
}
