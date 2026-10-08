import type { AppError } from '@/shared/kernel/errors'
import { createPortContext } from '@/shared/kernel/ports'

import type {
  CommitPlan,
  CommitRequest,
  CommitResult,
  StagedItem,
  UploadJob,
} from '../domain/types'

export interface UploadOptions {
  /** Console screen the upload started from (RF-08). */
  consoleSlug?: string
  /** Continue an interrupted upload (the tus id) instead of starting one. */
  resumeJobId?: string
}

export interface UploadCallbacks {
  /** The server created the upload; its id is the job id. */
  onJobId: (jobId: string) => void
  onProgress: (sent: number, total: number) => void
  onSuccess: () => void
  /** Retries are over (network) or the server refused the upload. */
  onError: (error: AppError) => void
}

export interface UploadHandle {
  /** Stops sending; the server keeps what it got (it can be resumed). */
  abort: () => void
}

/** Uploads and their jobs: extraction, review and commit (RF-01 to RF-13). */
export interface IngestionPorts {
  /** Resumable upload (tus): the bytes never pass through memory twice. */
  upload(file: File, options: UploadOptions, callbacks: UploadCallbacks): UploadHandle
  jobs(): Promise<UploadJob[]>
  job(id: string): Promise<UploadJob>
  /** Live job changes. Returns a function that stops watching. */
  watchJobs(handlers: { onJob: (job: UploadJob) => void; onOpen?: () => void }): () => void
  items(jobId: string): Promise<StagedItem[]>
  submitPassword(jobId: string, password: string): Promise<UploadJob>
  cancel(jobId: string): Promise<UploadJob>
  plan(jobId: string, request: CommitRequest): Promise<CommitPlan>
  commit(jobId: string, request: CommitRequest): Promise<CommitResult>
}

export const [IngestionContext, useIngestionPorts] = createPortContext<IngestionPorts>('Ingestion')
