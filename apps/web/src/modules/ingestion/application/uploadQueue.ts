import type { LocalUpload } from '../domain/uploads'
import type { IngestionPorts, UploadHandle } from './ports'

interface Entry extends LocalUpload {
  file: File | null
  consoleSlug?: string
  handle?: UploadHandle
}

const dismissedKey = 'ge.dismissedUploads'

function loadDismissed(): Set<string> {
  try {
    const raw = localStorage.getItem(dismissedKey)
    const list: unknown = raw ? JSON.parse(raw) : []
    return new Set(Array.isArray(list) ? list.filter((v) => typeof v === 'string') : [])
  } catch {
    return new Set()
  }
}

/**
 * The uploads this browser sends: at most `limit` at a time, the rest wait
 * in order. Screens read it with useSyncExternalStore.
 */
export class UploadQueue {
  private entries: Entry[] = []
  private snapshot: readonly LocalUpload[] = []
  private dismissed: Set<string> = loadDismissed()
  private listeners = new Set<() => void>()
  private seq = 0

  constructor(
    private readonly ports: Pick<IngestionPorts, 'upload' | 'cancel'>,
    private readonly limit = 2,
    private readonly now: () => number = Date.now,
  ) {}

  subscribe = (listener: () => void) => {
    this.listeners.add(listener)
    return () => {
      this.listeners.delete(listener)
    }
  }

  getSnapshot = (): readonly LocalUpload[] => this.snapshot

  dismissedJobs = (): ReadonlySet<string> => this.dismissed

  /** Queues files; consoleSlug is the console screen they came from. */
  add(files: readonly File[], consoleSlug?: string) {
    for (const file of files) {
      this.entries.push({
        key: `local-${String(++this.seq)}`,
        fileName: file.name,
        size: file.size,
        sent: 0,
        state: 'queued',
        createdAt: this.now(),
        file,
        ...(consoleSlug ? { consoleSlug } : {}),
      })
    }
    this.pump()
  }

  /**
   * Continues an interrupted upload with the file picked again. Returns
   * false when it is not the same file (name and size must match).
   */
  resume(job: { id: string; fileName: string; size: number }, file: File): boolean {
    if (file.name !== job.fileName || file.size !== job.size) return false
    this.entries = this.entries.filter((e) => e.jobId !== job.id)
    this.entries.push({
      key: `local-${String(++this.seq)}`,
      fileName: file.name,
      size: file.size,
      sent: 0,
      state: 'queued',
      jobId: job.id,
      createdAt: this.now(),
      file,
    })
    this.pump()
    return true
  }

  /** Tries again after the retries ran out (the file is still at hand). */
  retry(key: string) {
    this.update(key, { state: 'queued' })
    this.pump()
  }

  /** Stops an upload and deletes what the server got. */
  async cancel(key: string) {
    const entry = this.entries.find((e) => e.key === key)
    if (!entry) return
    entry.handle?.abort()
    this.entries = this.entries.filter((e) => e !== entry)
    this.emit()
    this.pump()
    if (entry.jobId) await this.ports.cancel(entry.jobId)
  }

  /** Hides a finished or failed upload from the panel, on this browser. */
  dismiss(jobId: string) {
    this.dismissed = new Set(this.dismissed).add(jobId)
    try {
      localStorage.setItem(dismissedKey, JSON.stringify([...this.dismissed].slice(-200)))
    } catch {
      // Still hidden for this visit.
    }
    this.emit()
  }

  /** Whether closing the tab now would interrupt an upload. */
  busy(): boolean {
    return this.entries.some((e) => e.state === 'queued' || e.state === 'uploading')
  }

  private pump() {
    let running = this.entries.filter((e) => e.state === 'uploading').length
    for (const entry of this.entries) {
      if (running >= this.limit) break
      if (entry.state !== 'queued' || !entry.file) continue
      running++
      this.start(entry, entry.file)
    }
    this.emit()
  }

  private start(entry: Entry, file: File) {
    entry.state = 'uploading'
    const key = entry.key
    entry.handle = this.ports.upload(
      file,
      {
        ...(entry.consoleSlug ? { consoleSlug: entry.consoleSlug } : {}),
        ...(entry.jobId ? { resumeJobId: entry.jobId } : {}),
      },
      {
        onJobId: (jobId) => {
          this.update(key, { jobId })
        },
        onProgress: (sent) => {
          this.update(key, { sent })
        },
        onSuccess: () => {
          this.update(key, { state: 'finished', sent: file.size })
          const done = this.entries.find((e) => e.key === key)
          if (done) {
            done.file = null
            delete done.handle
          }
          this.pump()
        },
        onError: () => {
          this.update(key, { state: 'error' })
          this.pump()
        },
      },
    )
  }

  private update(key: string, patch: Partial<LocalUpload>) {
    const entry = this.entries.find((e) => e.key === key)
    if (!entry) return
    Object.assign(entry, patch)
    this.emit()
  }

  private emit() {
    this.snapshot = this.entries.map(
      ({ key, fileName, size, sent, state, jobId, createdAt }): LocalUpload => ({
        key,
        fileName,
        size,
        sent,
        state,
        createdAt,
        ...(jobId ? { jobId } : {}),
      }),
    )
    for (const listener of this.listeners) listener()
  }
}
