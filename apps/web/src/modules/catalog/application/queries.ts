import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from '@tanstack/react-query'

import { useDebounced } from '@/shared/kernel/hooks'

import type { Console, GameEdit, ItemEdit } from '../domain/types'
import { useCatalogPorts } from './ports'

export const catalogKeys = {
  consoles: ['consoles'] as const,
  games: ['games'] as const,
  consoleGames: (slug: string) => ['games', 'console', slug] as const,
  search: (q: string) => ['games', 'search', q] as const,
  game: (id: number) => ['games', 'detail', id] as const,
  unassigned: ['unassigned'] as const,
  trash: ['trash'] as const,
  scan: ['scan'] as const,
}

/** Any library change: counts, lists, details, the unassigned section and the trash may move. */
export function invalidateLibrary(client: QueryClient) {
  return Promise.all([
    client.invalidateQueries({ queryKey: catalogKeys.consoles }),
    client.invalidateQueries({ queryKey: catalogKeys.games }),
    client.invalidateQueries({ queryKey: catalogKeys.unassigned }),
    client.invalidateQueries({ queryKey: catalogKeys.trash }),
  ])
}

// ---------- consoles ----------

export function useConsoles() {
  const { consoles } = useCatalogPorts()
  return useQuery({ queryKey: catalogKeys.consoles, queryFn: () => consoles.list() })
}

function useConsoleMutation<T>(run: (input: T) => Promise<Console>) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: run,
    onSuccess: (updated) => {
      client.setQueryData<Console[]>(catalogKeys.consoles, (list) =>
        list?.map((c) => (c.slug === updated.slug ? updated : c)),
      )
    },
  })
}

export function useRenameConsole() {
  const { consoles } = useCatalogPorts()
  return useConsoleMutation(({ slug, name }: { slug: string; name: string }) =>
    consoles.rename(slug, name),
  )
}

export function useAddExtension() {
  const { consoles } = useCatalogPorts()
  return useConsoleMutation(({ slug, extension }: { slug: string; extension: string }) =>
    consoles.addExtension(slug, extension),
  )
}

export function useRemoveExtension() {
  const { consoles } = useCatalogPorts()
  return useConsoleMutation(({ slug, extension }: { slug: string; extension: string }) =>
    consoles.removeExtension(slug, extension),
  )
}

export function useReorderConsoles() {
  const { consoles } = useCatalogPorts()
  const client = useQueryClient()
  return useMutation({
    mutationFn: (slugs: string[]) => consoles.reorder(slugs),
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

export function useUnassignGame() {
  const { library } = useCatalogPorts()
  return useLibraryMutation((id: number) => library.unassignGame(id))
}

export function useUnassignItem() {
  const { library } = useCatalogPorts()
  return useLibraryMutation((id: number) => library.unassignItem(id))
}

/** The preview of a rename or move (RF-24), as the dialog changes. */
export function useEditPlan(gameId: number, edit: GameEdit | undefined) {
  const { library } = useCatalogPorts()
  const debounced = useDebounced(edit, 300)
  return useQuery({
    queryKey: ['games', 'edit', gameId, debounced],
    queryFn: () => library.planEdit(gameId, debounced as GameEdit),
    enabled: debounced !== undefined,
    placeholderData: keepPreviousData,
    retry: false,
  })
}

export function useEditGame(gameId: number) {
  const { library } = useCatalogPorts()
  return useLibraryMutation((edit: GameEdit) => library.edit(gameId, edit))
}

export function useEditItem() {
  const { library } = useCatalogPorts()
  return useLibraryMutation(({ id, edit }: { id: number; edit: ItemEdit }) =>
    library.editItem(id, edit),
  )
}

export function useLastScan() {
  const { library } = useCatalogPorts()
  return useQuery({ queryKey: catalogKeys.scan, queryFn: () => library.lastScan() })
}

export function useScanLibrary() {
  const { library } = useCatalogPorts()
  const client = useQueryClient()
  return useMutation({
    mutationFn: () => library.scan(),
    onSuccess: async (report) => {
      client.setQueryData(catalogKeys.scan, report)
      await invalidateLibrary(client)
    },
  })
}

// ---------- unassigned ----------

export function useUnassigned() {
  const { unassigned } = useCatalogPorts()
  return useQuery({ queryKey: catalogKeys.unassigned, queryFn: () => unassigned.list() })
}

export function useTrashUnassigned() {
  const { unassigned } = useCatalogPorts()
  return useLibraryMutation((id: number) => unassigned.trash(id))
}

export function useDeleteUnassigned() {
  const { unassigned } = useCatalogPorts()
  return useLibraryMutation((id: number) => unassigned.remove(id))
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
