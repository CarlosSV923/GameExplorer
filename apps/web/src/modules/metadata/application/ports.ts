import { createPortContext } from '@/shared/kernel/ports'

import type { ImageSize, MetadataGame, MetadataStatus } from '../domain/types'

export interface MetadataPorts {
  /** Rejects with `igdbUnconfigured` or `igdbUnavailable` when IGDB cannot answer. */
  searchGames(query: string, platformId?: number, limit?: number): Promise<MetadataGame[]>
  status(): Promise<MetadataStatus>
  /** Where to load an IGDB image from (the API's local cache, or the demo). */
  imageUrl(size: ImageSize, imageId: string): string
}

export const [MetadataContext, useMetadataPorts] = createPortContext<MetadataPorts>('Metadata')
