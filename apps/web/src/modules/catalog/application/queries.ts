import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from '@tanstack/react-query'

import type { ConsoleCreate, ConsoleInput, DuplicateAction } from '../domain/types'
import { useCatalogPorts } from './ports'

export const catalogKeys = {
  consoles: ['consoles'] as const,
  games: ['games'] as const,
  consoleGames: (slug: string) => ['games', 'console', slug] as const,
  search: (q: string) => ['games', 'search', q] as const,
  game: (id: number) => ['games', 'detail', id] as const,
  trash: ['trash'] as const,
  integrity: ['integrity'] as const,
}

/** Any library change: counts, lists, details and the trash may move. */
export function invalidateLibrary(client: QueryClient) {
  return Promise.all([
    client.invalidateQueries({ queryKey: catalogKeys.consoles }),
    client.invalidateQueries({ queryKey: catalogKeys.games }),
    client.invalidateQueries({ queryKey: catalogKeys.trash }),
  ])
}

// ---------- consoles ----------

export function useConsoles() {
  const { consoles } = useCatalogPorts()
  return useQuery({ queryKey: catalogKeys.consoles, queryFn: () => consoles.list() })
}

export function useCreateConsole() {
  const { consoles } = useCatalogPorts()
  const client = useQueryClient()
  return useMutation({
    mutationFn: (input: ConsoleCreate) => consoles.create(input),
    onSuccess: () => client.invalidateQueries({ queryKey: catalogKeys.consoles }),
  })
}

export function useUpdateConsole() {
  const { consoles } = useCatalogPorts()
  const client = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: number; input: ConsoleInput }) => consoles.update(id, input),
    onSuccess: () => invalidateLibrary(client),
  })
}

export function useDeleteConsole() {
  const { consoles } = useCatalogPorts()
  const client = useQueryClient()
  return useMutation({
    mutationFn: (id: number) => consoles.remove(id),
    onSuccess: () => client.invalidateQueries({ queryKey: catalogKeys.consoles }),
  })
}

export function useReorderConsoles() {
  const { consoles } = useCatalogPorts()
  const client = useQueryClient()
  return useMutation({
    mutationFn: (ids: number[]) => consoles.reorder(ids),
    onSuccess: (list) => {
      client.setQueryData(catalogKeys.consoles, list)
    },
  })
}

// ---------- library ----------

export function useConsoleGames(slug: string | undefined) {
  const { library } = useCatalogPorts()
  return useQuery({
    queryKey: catalogKeys.consoleGames(slug ?? ''),
    queryFn: () => library.consoleGames(slug ?? ''),
    enabled: Boolean(slug),
  })
}

export function useLibrarySearch(query: string) {
  const { library } = useCatalogPorts()
  const q = query.trim()
  return useQuery({
    queryKey: catalogKeys.search(q),
    queryFn: () => library.search(q),
    enabled: q.length > 0,
    placeholderData: keepPreviousData,
  })
}

export function useGame(id: number | undefined) {
  const { library } = useCatalogPorts()
  return useQuery({
    queryKey: catalogKeys.game(id ?? 0),
    queryFn: () => library.game(id ?? 0),
    enabled: id !== undefined,
    retry: false,
  })
}

function useLibraryMutation<T, R>(run: (input: T) => Promise<R>) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: run,
    onSuccess: () => invalidateLibrary(client),
  })
}

export function useTrashGame() {
  const { library } = useCatalogPorts()
  return useLibraryMutation((id: number) => library.trashGame(id))
}

export function useTrashItem() {
  const { library } = useCatalogPorts()
  return useLibraryMutation((id: number) => library.trashItem(id))
}

export function useForgetItem() {
  const { library } = useCatalogPorts()
  return useLibraryMutation((id: number) => library.forgetItem(id))
}

export function useRematchPlan(
  gameId: number,
  igdbGameId: number | undefined,
  decisions: Readonly<Record<number, DuplicateAction>>,
) {
  const { library } = useCatalogPorts()
  const list = Object.entries(decisions).map(([itemId, onDuplicate]) => ({
    itemId: Number(itemId),
    onDuplicate,
  }))
  return useQuery({
    queryKey: ['rematch', gameId, igdbGameId, list],
    queryFn: () => library.planRematch(gameId, { igdbGameId: igdbGameId ?? 0, decisions: list }),
    enabled: igdbGameId !== undefined,
    placeholderData: keepPreviousData,
    retry: false,
  })
}

export function useRematch(gameId: number) {
  const { library } = useCatalogPorts()
  return useLibraryMutation(
    (input: { igdbGameId: number; decisions: Readonly<Record<number, DuplicateAction>> }) =>
      library.rematch(gameId, {
        igdbGameId: input.igdbGameId,
        decisions: Object.entries(input.decisions).map(([itemId, onDuplicate]) => ({
          itemId: Number(itemId),
          onDuplicate,
        })),
      }),
  )
}

export function useLastCheck() {
  const { library } = useCatalogPorts()
  return useQuery({ queryKey: catalogKeys.integrity, queryFn: () => library.lastCheck() })
}

export function useCheckLibrary() {
  const { library } = useCatalogPorts()
  const client = useQueryClient()
  return useMutation({
    mutationFn: () => library.check(),
    onSuccess: async (report) => {
      client.setQueryData(catalogKeys.integrity, report)
      await client.invalidateQueries({ queryKey: catalogKeys.games })
    },
  })
}

// ---------- trash ----------

export function useTrash() {
  const { trash } = useCatalogPorts()
  return useQuery({ queryKey: catalogKeys.trash, queryFn: () => trash.list() })
}

export function useRestore() {
  const { trash } = useCatalogPorts()
  return useLibraryMutation(({ id, replace }: { id: number; replace: boolean }) =>
    trash.restore(id, replace),
  )
}

export function useDeleteTrashEntry() {
  const { trash } = useCatalogPorts()
  return useLibraryMutation((id: number) => trash.remove(id))
}

export function useEmptyTrash() {
  const { trash } = useCatalogPorts()
  return useLibraryMutation(() => trash.empty())
}
