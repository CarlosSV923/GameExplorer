/** Catalog model as the screens see it (docs/spec.md §2). */

/**
 * What a file is within its game: base, update and dlc on consoles with
 * add-ons (Switch); game on consoles with one file per game (Wii, PSP).
 */
export type ItemKind = 'base' | 'update' | 'dlc' | 'game'

export interface CustomExtension {
  extension: string
  /** Files in the library with it; it can be removed only at 0. */
  fileCount: number
}

/** A console defined in code, with what the user can change (RF-40 to RF-42). */
export interface Console {
  /** Folder in the library; fixed. */
  slug: string
  displayName: string
  defaultName: string
  igdbPlatformId: number
  releaseYear?: number | null
  logoImageId?: string | null
  /** From the environment variable (or the code default): fixed here. */
  extensions: string[]
  customExtensions: CustomExtension[]
  kinds: ItemKind[]
  /** Several game files per upload (Switch) or exactly one (Wii, PSP). */
  multipleFiles: boolean
  sortOrder: number
  gameCount: number
}

export interface LibraryItem {
  id: number
  kind: ItemKind
  /** Update version (without the v) or DLC name. */
  label?: string | null
  /** Name inside the game folder. */
  file: string
  size: number
  createdAt: string
}

export interface GameSummary {
  id: number
  /** Unset for a name of the user's own. */
  igdbId?: number | null
  console: string
  title: string
  folder: string
  releaseYear?: number | null
  coverImageId?: string | null
  itemCount: number
  size: number
}

export interface GameDetail extends GameSummary {
  /** Library-relative folder ("switch/Limbo"). */
  path: string
  summary?: string | null
  genres: string[]
  items: LibraryItem[]
}

export type DuplicateAction = 'replace' | 'skip'
export type PlannedAction = 'store' | 'replace' | 'skip' | 'undecided'

/** Rename a game or move it to another console (RF-24). */
export interface GameEdit {
  console: string
  title: string
  igdbId?: number | null
  decisions?: { itemId: number; onDuplicate: DuplicateAction }[]
}

export interface GameEditPlan {
  console: string
  title: string
  folder: string
  /** A game of the target console already has the name: they merge. */
  mergeInto?: number | null
  items: {
    item: LibraryItem
    file: string
    action: PlannedAction
    duplicate?: LibraryItem | null
  }[]
}

export interface GameEditResult {
  gameId: number
  path: string
  merged: boolean
}

/** Change a file's kind, version or DLC name (RF-24). */
export interface ItemEdit {
  kind: ItemKind
  label?: string
  onDuplicate?: DuplicateAction
}

export type UnassignedReason = 'samba' | 'upload' | 'manual'

/** A file in `_unassigned/` (RF-27). */
export interface UnassignedFile {
  id: number
  /** Relative to the unassigned folder. */
  path: string
  name: string
  /** A library path, or the upload's file name. */
  origin: string
  reason: UnassignedReason
  size: number
  arrivedAt: string
  /** Consoles whose extensions accept it. */
  consoles: string[]
  /** A zip, 7z or rar: validated once extracted. */
  archive: boolean
  /** Still being copied over SMB (id is 0): no action yet (RF-26a). */
  copying?: boolean
  /** The console it came from, if known. */
  console?: string
  /** The IGDB game picked when it was uploaded without console. */
  igdbId?: number | null
}

/** An entry of the unassigned section: a first-level folder, or a loose file (RF-27). */
export interface UnassignedEntry {
  /** The lowest id of its files; 0 while every file is still copying. */
  id: number
  name: string
  folder: boolean
  size: number
  /** Prefill Asignar. */
  console?: string
  igdbId?: number | null
  arrivedAt: string
  /** A file is still being copied: no action. */
  copying: boolean
  /** An assignment in progress uses it: no action. */
  busy: boolean
  files: UnassignedFile[]
}

export interface TrashEntry {
  id: number
  kind: 'game' | 'unassigned'
  gameId?: number | null
  console?: string | null
  /** Empty for unassigned files. */
  title: string
  folder?: string | null
  wholeGame: boolean
  reason: 'deleted' | 'replaced'
  trashedAt: string
  expiresAt: string
  size: number
  items: LibraryItem[]
  files: UnassignedFile[]
}

export interface RestoreResult {
  gameId?: number | null
  path: string
}

/** What a library scan changed (RF-26). */
export interface ScanReport {
  scannedAt: string
  /** Unknown files moved to (or found in) the unassigned section. */
  unassigned: number
  /** Files deleted over SMB that left the library. */
  removed: number
  /** Unknown files still changing; the next scan moves them. */
  pending: number
}
