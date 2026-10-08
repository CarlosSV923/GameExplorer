import { createPortContext } from '@/shared/kernel/ports'

import type {
  Console,
  ConsoleCreate,
  ConsoleInput,
  GameDetail,
  GameSummary,
  IntegrityReport,
  RematchPlan,
  RematchRequest,
  RematchResult,
  RestoreResult,
  TrashEntry,
} from '../domain/types'

/** Consoles: the carousel and Settings (RF-40, RF-41). */
export interface ConsolePorts {
  list(): Promise<Console[]>
  create(input: ConsoleCreate): Promise<Console>
  update(id: number, input: ConsoleInput): Promise<Console>
  remove(id: number): Promise<void>
  /** Every console id, first to last. */
  reorder(ids: number[]): Promise<Console[]>
}

/** Browsing, downloads, re-match and integrity (RF-20 to RF-26). */
export interface LibraryPorts {
  consoleGames(slug: string): Promise<GameSummary[]>
  search(query: string): Promise<GameSummary[]>
  game(id: number): Promise<GameDetail>
  trashGame(id: number): Promise<void>
  trashItem(id: number): Promise<void>
  forgetItem(id: number): Promise<void>
  planRematch(gameId: number, request: RematchRequest): Promise<RematchPlan>
  rematch(gameId: number, request: RematchRequest): Promise<RematchResult>
  lastCheck(): Promise<IntegrityReport | null>
  check(): Promise<IntegrityReport>
  /** Where a download link points (a real file, or the demo's note). */
  gameDownloadUrl(gameId: number): string
  itemDownloadUrl(itemId: number): string
}

/** The trash (RF-30). */
export interface TrashPorts {
  list(): Promise<TrashEntry[]>
  /** Rejects with `conflict` when the item's place is taken and replace is false. */
  restore(id: number, replace: boolean): Promise<RestoreResult>
  remove(id: number): Promise<void>
  empty(): Promise<void>
}

export interface CatalogPorts {
  consoles: ConsolePorts
  library: LibraryPorts
  trash: TrashPorts
}

export const [CatalogContext, useCatalogPorts] = createPortContext<CatalogPorts>('Catalog')
