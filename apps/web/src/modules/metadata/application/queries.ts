import { keepPreviousData, useQuery } from '@tanstack/react-query'

import { useDebounced } from '@/shared/kernel/hooks'

import { useMetadataPorts } from './ports'

/** IGDB needs at least two characters (api/openapi.yaml). */
export const minQuery = 2

/** Games on IGDB as the user types (debounced), limited to one platform. */
export function useGameSearch(query: string, platformId: number | undefined) {
  const ports = useMetadataPorts()
  const q = useDebounced(query.trim(), 300)
  return useQuery({
    queryKey: ['metadata', 'games', q, platformId ?? null],
    queryFn: () => ports.searchGames(q, platformId),
    enabled: q.length >= minQuery,
    placeholderData: keepPreviousData,
    staleTime: 5 * 60_000,
  })
}

export function usePlatformSearch(query: string) {
  const ports = useMetadataPorts()
  const q = useDebounced(query.trim(), 300)
  return useQuery({
    queryKey: ['metadata', 'platforms', q],
    queryFn: () => ports.searchPlatforms(q),
    enabled: q.length >= minQuery,
    placeholderData: keepPreviousData,
    staleTime: 5 * 60_000,
  })
}
