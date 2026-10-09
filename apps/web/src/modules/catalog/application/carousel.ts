import { unassignedSlug } from '../domain/items'
import type { Console, UnassignedEntry } from '../domain/types'
import { useConsoles, useUnassigned } from './queries'

/** One stop of the carousel: a console with games, or the unassigned section. */
export interface CarouselEntry {
  slug: string
  name: string
  releaseYear?: number | null
  logoImageId?: string | null
  /** Games of the console, or files of the unassigned section. */
  count: number
  unassigned: boolean
}

/**
 * The carousel (RF-20): consoles with games in the Settings order, then the
 * unassigned section when it has files.
 */
export function carouselEntries(
  consoles: readonly Console[],
  unassigned: readonly UnassignedEntry[],
): CarouselEntry[] {
  const entries = consoles
    .filter((c) => c.gameCount > 0)
    .map<CarouselEntry>((c) => ({
      slug: c.slug,
      name: c.displayName,
      releaseYear: c.releaseYear ?? null,
      logoImageId: c.logoImageId ?? null,
      count: c.gameCount,
      unassigned: false,
    }))
  if (unassigned.length > 0) {
    const files = unassigned.reduce((n, e) => n + e.files.length, 0)
    entries.push({ slug: unassignedSlug, name: '', count: files, unassigned: true })
  }
  return entries
}

export function useCarousel() {
  const consoles = useConsoles()
  const unassigned = useUnassigned()
  return {
    entries: carouselEntries(consoles.data ?? [], unassigned.data ?? []),
    /** Every console, for the empty library ("Sube un juego de…"). */
    consoleNames: (consoles.data ?? []).map((c) => c.displayName),
    isPending: consoles.isPending || unassigned.isPending,
    isError: consoles.isError || unassigned.isError,
    refetch: () => Promise.all([consoles.refetch(), unassigned.refetch()]),
  }
}
