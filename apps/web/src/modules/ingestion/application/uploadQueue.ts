import type { UploadJob, UploadSpec } from '../domain/types'
import type { LocalUpload } from '../domain/uploads'
import type { IngestionPorts, UploadHandle } from './ports'

interface Entry extends LocalUpload {
  file: File | null
  spec: UploadSpec
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

  /** Queues files, each with what the form decided for it (RF-03). */
  add(uploads: readonly { file: File; spec: UploadSpec }[]) {
    for (const { file, spec } of uploads) {
      this.entries.push({
        key: `local-${String(++this.seq)}`,
        fileName: file.name,
        size: file.size,
        sent: 0,
        state: 'queued',
        console: spec.console,
        title: spec.title,
        createdAt: this.now(),
        file,
        spec,
      })
    }
    this.pump()
  }

  /**
   * Continues an interrupted upload with the file picked again. Returns
   * false when it is not the same file (name and size must match).
   */
  resume(
    job: Pick<UploadJob, 'id' | 'fileName' | 'size' | 'console' | 'title'>,
    file: File,
  ): boolean {
    if (file.name !== job.fileName || file.size !== job.size) return false
    this.entries = this.entries.filter((e) => e.jobId !== job.id)
    this.entries.push({
      key: `local-${String(++this.seq)}`,
      fileName: file.name,
      size: file.size,
      sent: 0,
      state: 'queued',
      console: job.console,
      title: job.title,
      jobId: job.id,
      createdAt: this.now(),
      file,
      spec: { console: job.console, title: job.title },
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
      { spec: entry.spec, ...(entry.jobId ? { resumeJobId: entry.jobId } : {}) },
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
      ({ key, fileName, size, sent, state, console, title, jobId, createdAt }): LocalUpload => ({
        key,
        fileName,
        size,
        sent,
        state,
        console,
        title,
        createdAt,
        ...(jobId ? { jobId } : {}),
      }),
    )
    for (const listener of this.listeners) listener()
  }
}
