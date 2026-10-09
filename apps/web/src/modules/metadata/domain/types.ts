/** A game on IGDB (read through the metadata anti-corruption layer). */
export interface MetadataGame {
  id: number
  name: string
  releaseYear?: number | null
  coverImageId?: string | null
  summary?: string | null
  genres: string[]
  platformIds: number[]
}

/** Whether IGDB credentials are set (RF-54): without them nothing is suggested. */
export interface MetadataStatus {
  configured: boolean
}

export type ImageSize = 'cover_small' | 'cover_big' | 'logo_med' | 'screenshot_med'
