/** Catalog model as the screens see it (docs/spec.md §2). */

export type Detection = 'titleId' | 'header' | 'structure' | 'extension'

export interface Console {
  id: number
  slug: string
  displayName: string
  igdbPlatformId?: number | null
  releaseYear?: number | null
  logoImageId?: string | null
  extensions: string[]
  sortOrder: number
  gameCount: number
  builtIn: boolean
  detection: Detection
}

export interface ConsoleInput {
  slug: string
  displayName: string
  extensions: string[]
}

export interface ConsoleCreate extends ConsoleInput {
  igdbPlatformId: number
}

export type ItemKind = 'base' | 'update' | 'dlc' | 'disc'
export type ItemShape = 'file' | 'disc' | 'folder'

export interface LibraryItem {
  id: number
  kind: ItemKind
  label?: string | null
  discNumber?: number | null
  shape: ItemShape
  /** Relative to the game folder; the first is the main entry. */
  files: string[]
  size: number
  createdAt: string
  missingSince?: string | null
}

export interface GameSummary {
  id: number
  igdbId: number
  console: string
  title: string
  folder: string
  releaseYear?: number | null
  coverImageId?: string | null
  itemCount: number
  missingCount: number
  size: number
}

export interface GameDetail extends GameSummary {
  path: string
  summary?: string | null
  genres: string[]
  items: LibraryItem[]
}

export type DuplicateAction = 'replace' | 'skip'
export type PlannedAction = 'store' | 'replace' | 'skip' | 'undecided'

export interface RematchRequest {
  igdbGameId: number
  decisions?: { itemId: number; onDuplicate: DuplicateAction }[]
}

export interface RematchPlan {
  console: string
  title: string
  folder: string
  mergeInto?: number | null
  items: {
    item: LibraryItem
    files: string[]
    action: PlannedAction
    duplicate?: LibraryItem | null
  }[]
}

export interface RematchResult {
  gameId: number
  path: string
  merged: boolean
}

export interface TrashEntry {
  id: number
  gameId: number
  console: string
  title: string
  folder: string
  wholeGame: boolean
  reason: 'deleted' | 'replaced'
  trashedAt: string
  expiresAt: string
  size: number
  items: LibraryItem[]
}

export interface RestoreResult {
  gameId: number
  path: string
}

export interface IntegrityReport {
  checkedAt: string
  checked: number
  missing: number
  found: number
  missingTotal: number
}
