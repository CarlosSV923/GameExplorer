import { createPortContext } from '@/shared/kernel/ports'

import type {
  Console,
  GameDetail,
  GameEdit,
  GameEditPlan,
  GameEditResult,
  GameSummary,
  ItemEdit,
  RestoreResult,
  ScanReport,
  TrashEntry,
  UnassignedEntry,
} from '../domain/types'

/** Consoles: the carousel and Settings (RF-40 to RF-42). */
export interface ConsolePorts {
  list(): Promise<Console[]>
  rename(slug: string, displayName: string): Promise<Console>
  /** Every console slug, first to last. */
  reorder(slugs: string[]): Promise<Console[]>
  /** Rejects with `conflict` when another console already has it. */
  addExtension(slug: string, extension: string): Promise<Console>
  /** Rejects with `conflict` while a file uses it. */
  removeExtension(slug: string, extension: string): Promise<Console>
}

/** Browsing, downloads, editing and the scan (RF-20 to RF-26). */
export interface LibraryPorts {
  consoleGames(slug: string): Promise<GameSummary[]>
  search(query: string): Promise<GameSummary[]>
  game(id: number): Promise<GameDetail>
  trashGame(id: number): Promise<void>
  trashItem(id: number): Promise<void>
  unassignGame(id: number): Promise<void>
  unassignItem(id: number): Promise<void>
  planEdit(gameId: number, edit: GameEdit): Promise<GameEditPlan>
  edit(gameId: number, edit: GameEdit): Promise<GameEditResult>
  /** Rejects with `conflict` when the new name is taken and onDuplicate is unset. */
  editItem(itemId: number, edit: ItemEdit): Promise<GameDetail>
  lastScan(): Promise<ScanReport | null>
  scan(): Promise<ScanReport>
  /** Where a download link points (a real file, or the demo's note). */
  gameDownloadUrl(gameId: number): string
  itemDownloadUrl(itemId: number): string
}

/**
 * The unassigned section by entry (RF-27); assigning an entry starts an
 * upload job (ingestion). Files and entries are acted on by id (an entry's
 * id is one of its files').
 */
export interface UnassignedPorts {
  list(): Promise<UnassignedEntry[]>
  trash(fileId: number): Promise<void>
  remove(fileId: number): Promise<void>
  downloadUrl(fileId: number): string
  trashEntry(entryId: number): Promise<void>
  removeEntry(entryId: number): Promise<void>
  entryDownloadUrl(entryId: number): string
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
  unassigned: UnassignedPorts
  trash: TrashPorts
}

export const [CatalogContext, useCatalogPorts] = createPortContext<CatalogPorts>('Catalog')
