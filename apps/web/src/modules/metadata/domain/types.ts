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

/** An IGDB platform, to add a console (RF-41). */
export interface MetadataPlatform {
  id: number
  name: string
  abbreviation?: string | null
  logoImageId?: string | null
  releaseYear?: number | null
}

export type ImageSize = 'cover_small' | 'cover_big' | 'logo_med' | 'screenshot_med'
