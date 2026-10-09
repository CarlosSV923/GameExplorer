import type {
  DuplicateAction,
  ItemKind,
  LibraryItem,
  PlannedAction,
} from '@/modules/catalog/domain/types'

/**
 * uploading → uploaded → (extracting ⇄ needs_password) → confirm | invalid →
 * committing → done. Parts of a multi-volume archive wait in waiting_parts;
 * the first volume continues and the rest become merged. An invalid job can
 * change console or be set aside (unassigned, trashed, cancelled). done,
 * failed, cancelled, merged, unassigned and trashed are terminal.
 */
export type JobStatus =
  | 'uploading'
  | 'uploaded'
  | 'waiting_parts'
  | 'merged'
  | 'extracting'
  | 'needs_password'
  | 'confirm'
  | 'invalid'
  | 'committing'
  | 'done'
  | 'unassigned'
  | 'trashed'
  | 'failed'
  | 'cancelled'

/** none: no file fits the console; many: more than one on a one-file console. */
export type InvalidReason = 'none' | 'many'

export interface UploadJob {
  /** Equals the tus upload id. */
  id: string
  fileName: string
  size: number
  received: number
  status: JobStatus
  /** Console slug and game name chosen in the form. */
  console: string
  title: string
  /** IGDB game the name was picked from. */
  igdbId?: number | null
  invalidReason?: InvalidReason | null
  /** Assigns an unassigned entry (RF-27a). */
  fromUnassigned?: boolean
  /** Extraction percentage while extracting. */
  progress?: number
  error?: string | null
  warning?: string | null
  /** Parts of the multi-volume archive this job belongs to. */
  groupSize?: number | null
  mergedInto?: string | null
  createdAt: string
  updatedAt: string
}

/** What the form decided for an upload (RF-03): it travels with the upload. */
export interface UploadSpec {
  /** "" sends the upload to the unassigned section (RF-07b). */
  console: string
  title: string
  igdbId?: number
  /** The parts of one multi-volume archive share a group (RF-03a). */
  group?: string
  groupSize?: number
}

/** A file found in an upload (RF-07). */
export interface StagedFile {
  /** Relative to the upload; identifies the file in the commit. */
  path: string
  size: number
  /** A game file for the job's console; the others are discarded. */
  valid: boolean
  /** Every console whose extensions accept it. */
  consoles: string[]
  /** Of an assigned unassigned entry, not an archive: it stays there until stored (RF-27a). */
  inPlace?: boolean
}

export interface CommitFile {
  path: string
  /** Required on consoles with several kinds; omitted means game. */
  kind?: ItemKind
  label?: string
  onDuplicate?: DuplicateAction
  /** "No guardar": stays in the unassigned section (assigned entries only, RF-27a). */
  skip?: boolean
}

export interface CommitRequest {
  files: CommitFile[]
}

export interface PlannedFile {
  path: string
  /** Final name inside the game folder. */
  file: string
  action: PlannedAction
  duplicate?: LibraryItem | null
}

export interface CommitPlan {
  console: string
  title: string
  folder: string
  /** Set when the game is already in the library. */
  gameId?: number | null
  /** The game's current files. */
  existing: LibraryItem[]
  files: PlannedFile[]
  /** Files that are not game files for the console. */
  discarded: string[]
}

export interface CommitResult {
  job: UploadJob
  gameId: number
  /** Library-relative game folder ("switch/Limbo"). */
  path: string
  stored: number
  replaced: number
  skipped: number
}

export type ResolveAction = 'unassigned' | 'trash' | 'delete'

/** Statuses after which nothing else happens to a job. */
export const terminalStatuses: ReadonlySet<JobStatus> = new Set([
  'done',
  'failed',
  'cancelled',
  'merged',
  'unassigned',
  'trashed',
])
