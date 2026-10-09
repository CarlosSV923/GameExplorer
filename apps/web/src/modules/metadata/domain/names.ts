import type { MetadataGame } from './types'

/** A game name: free text, linked to an IGDB game when picked from the suggestions (RF-11). */
export interface NameValue {
  text: string
  game?: MetadataGame
}

/** The IGDB id to store: only while the text is still the picked game's name. */
export function linkedId(value: NameValue): number | undefined {
  return value.game && value.game.name === value.text ? value.game.id : undefined
}
