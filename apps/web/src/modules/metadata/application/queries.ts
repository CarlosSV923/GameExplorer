import { keepPreviousData, useQuery } from '@tanstack/react-query'

import { useDebounced } from '@/shared/kernel/hooks'

import { useMetadataPorts } from './ports'

/** IGDB needs at least two characters (api/openapi.yaml). */
export const minQuery = 2

/** Suggestions shown under a name field (docs/design-handoff.md §9). */
export const maxSuggestions = 5

/** Games on IGDB as the user types (debounced), limited to one platform. */
export function useGameSearch(query: string, platformId: number | undefined, enabled = true) {
  const ports = useMetadataPorts()
  const q = useDebounced(query.trim(), 300)
  return useQuery({
    queryKey: ['metadata', 'games', q, platformId ?? null],
    queryFn: () => ports.searchGames(q, platformId, maxSuggestions),
    enabled: enabled && q.length >= minQuery,
    placeholderData: keepPreviousData,
    staleTime: 5 * 60_000,
  })
}

/** Whether IGDB is configured; it does not change while the app runs. */
export function useMetadataStatus() {
  const ports = useMetadataPorts()
  return useQuery({
    queryKey: ['metadata', 'status'],
    queryFn: () => ports.status(),
    staleTime: Infinity,
  })
}
