import type { UploadJob } from './types'

/** An upload this browser is sending (or will send) right now. */
export interface LocalUpload {
  key: string
  fileName: string
  size: number
  sent: number
  state: 'queued' | 'uploading' | 'error' | 'finished'
  /** Known once the server created the upload (the tus id). */
  jobId?: string
  createdAt: number
}

export type UploadPhase =
  | 'queued'
  | 'uploading'
  | 'uploadingElsewhere'
  | 'interrupted'
  | 'uploadError'
  | 'uploaded'
  | 'waitingParts'
  | 'extracting'
  | 'needsPassword'
  | 'review'
  | 'committing'
  | 'done'
  | 'failed'

/** One card of the uploads panel: a local upload, a server job, or both. */
export interface UploadRow {
  key: string
  jobId?: string
  localKey?: string
  fileName: string
  size: number
  sent: number
  phase: UploadPhase
  progress?: number
  error?: string | null
  warning?: string | null
  volumeIndex?: number | null
}

/**
 * Merges job lists keeping, for each job, its most recent version: a list
 * fetched before a live event must not overwrite the event.
 */
export function mergeJobs(
  current: readonly UploadJob[],
  incoming: readonly UploadJob[],
): UploadJob[] {
  const byId = new Map(current.map((j) => [j.id, j]))
  for (const job of incoming) {
    const known = byId.get(job.id)
    if (!known || Date.parse(known.updatedAt) <= Date.parse(job.updatedAt)) byId.set(job.id, job)
  }
  return [...byId.values()].sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))
}

/** A job still "uploading" without progress for this long was interrupted. */
export const staleAfterMs = 15_000
/** Finished and failed uploads stay in the panel for a day. */
export const keepFinishedMs = 24 * 60 * 60 * 1000

function serverPhase(job: UploadJob): UploadPhase {
  switch (job.status) {
    case 'uploaded':
      return 'uploaded'
    case 'waiting_parts':
      return 'waitingParts'
    case 'extracting':
      return 'extracting'
    case 'needs_password':
      return 'needsPassword'
    case 'review':
      return 'review'
    case 'committing':
      return 'committing'
    case 'done':
      return 'done'
    default:
      return 'failed'
  }
}

/**
 * Merges what this browser is sending with the jobs on the server
 * (RF-13). A job stuck in "uploading" that nobody here is sending was
 * interrupted (the tab closed): it can be resumed by picking the file again.
 */
export function buildRows(
  local: readonly LocalUpload[],
  jobs: readonly UploadJob[],
  now: number,
  dismissed: ReadonlySet<string>,
): UploadRow[] {
  const byJob = new Map(local.filter((l) => l.jobId).map((l) => [l.jobId, l]))
  const seen = new Set<string>()
  const rows: UploadRow[] = []

  const sorted = [...jobs].sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))
  for (const job of sorted) {
    seen.add(job.id)
    if (job.status === 'merged' || job.status === 'cancelled' || dismissed.has(job.id)) continue
    const finished = job.status === 'done' || job.status === 'failed'
    if (finished && now - Date.parse(job.updatedAt) > keepFinishedMs) continue

    const mine = byJob.get(job.id)
    let phase: UploadPhase
    let sent = job.received
    if (job.status === 'uploading') {
      if (mine?.state === 'uploading' || mine?.state === 'queued') {
        phase = mine.state
        sent = Math.max(sent, mine.sent)
      } else if (mine?.state === 'error') {
        phase = 'uploadError'
      } else if (mine?.state === 'finished') {
        phase = 'uploaded'
        sent = job.size
      } else {
        phase =
          now - Date.parse(job.updatedAt) < staleAfterMs ? 'uploadingElsewhere' : 'interrupted'
      }
    } else {
      phase = serverPhase(job)
    }
    rows.push({
      key: job.id,
      jobId: job.id,
      ...(mine ? { localKey: mine.key } : {}),
      fileName: job.fileName,
      size: job.size,
      sent,
      phase,
      ...(job.progress === undefined ? {} : { progress: job.progress }),
      error: job.error ?? null,
      warning: job.warning ?? null,
      volumeIndex: job.volumeIndex ?? null,
    })
  }

  // Uploads the server does not list yet (queued, or just created).
  const pending = local
    .filter((l) => !(l.jobId && seen.has(l.jobId)) && l.state !== 'finished')
    .sort((a, b) => b.createdAt - a.createdAt)
    .map<UploadRow>((l) => ({
      key: l.key,
      localKey: l.key,
      ...(l.jobId ? { jobId: l.jobId } : {}),
      fileName: l.fileName,
      size: l.size,
      sent: l.sent,
      phase: l.state === 'queued' ? 'queued' : l.state === 'error' ? 'uploadError' : 'uploading',
    }))
  return [...pending, ...rows]
}

const settled: ReadonlySet<UploadPhase> = new Set(['done', 'failed'])
const waitingForUser: ReadonlySet<UploadPhase> = new Set([
  'review',
  'needsPassword',
  'interrupted',
  'uploadError',
])

/** Uploads still in progress or waiting for the user (the header badge). */
export function activeRows(rows: readonly UploadRow[]): UploadRow[] {
  return rows.filter((r) => !settled.has(r.phase))
}

export function needsUser(row: UploadRow): boolean {
  return waitingForUser.has(row.phase)
}
