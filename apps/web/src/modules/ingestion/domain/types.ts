import type {
  DuplicateAction,
  ItemKind,
  LibraryItem,
  PlannedAction,
} from '@/modules/catalog/domain/types'

/**
 * uploading → uploaded → (extracting ⇄ needs_password) → review →
 * committing → done; failed, cancelled and merged are terminal. Parts of a
 * multi-volume archive wait in waiting_parts until the set is complete.
 */
export type JobStatus =
  | 'uploading'
  | 'uploaded'
  | 'waiting_parts'
  | 'merged'
  | 'extracting'
  | 'needs_password'
  | 'review'
  | 'committing'
  | 'done'
  | 'failed'
  | 'cancelled'

export interface UploadJob {
  /** Equals the tus upload id. */
  id: string
  fileName: string
  size: number
  received: number
  status: JobStatus
  /** Extraction percentage while extracting. */
  progress?: number
  error?: string | null
  warning?: string | null
  /** Console screen the upload started from (RF-08). */
  originConsole?: string | null
  volumeIndex?: number | null
  mergedInto?: string | null
  createdAt: string
  updatedAt: string
}

export interface StagedItem {
  /** Relative to the upload; identifies the item in the review. */
  path: string
  shape: 'file' | 'disc' | 'folder'
  parts: string[]
  size: number
  ignored: boolean
  /** Candidate console slugs: one when conclusive, none when unknown. */
  consoles: string[]
  confidence: 'header' | 'extension' | 'none'
  suggestedKind?: ItemKind | null
  titleId?: string | null
  versionCode?: string | null
  displayVersion?: string | null
  discNumber?: number | null
}

export interface CommitItem {
  path: string
  skip?: boolean
  kind?: ItemKind
  label?: string
  discNumber?: number
  onDuplicate?: DuplicateAction
}

export interface CommitRequest {
  console: string
  igdbGameId: number
  items: CommitItem[]
}

export interface PlannedItem {
  path: string
  /** Final names inside the game folder (a .cue sheet first). */
  files: string[]
  action: PlannedAction
  duplicate?: LibraryItem | null
}

export interface CommitPlan {
  console: string
  title: string
  folder: string
  gameId?: number | null
  /** The game's current items. */
  existing: LibraryItem[]
  items: PlannedItem[]
}

export interface CommitResult {
  job: UploadJob
  gameId: number
  /** Library-relative game folder ("switch/Mario Kart 8 Deluxe"). */
  path: string
  stored: number
  replaced: number
  skipped: number
}

/** Statuses after which nothing else happens to a job. */
export const terminalStatuses: ReadonlySet<JobStatus> = new Set([
  'done',
  'failed',
  'cancelled',
  'merged',
])
