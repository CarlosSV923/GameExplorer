import type { AppError } from '@/shared/kernel/errors'
import { createPortContext } from '@/shared/kernel/ports'

import type {
  CommitPlan,
  CommitRequest,
  CommitResult,
  ResolveAction,
  StagedFile,
  UploadJob,
  UploadSpec,
} from '../domain/types'

export interface UploadOptions {
  /** What the form decided (RF-03); ignored when resuming. */
  spec: UploadSpec
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

/** A ready-made file to try the flow with (the demo's, RF-64). */
export interface SampleFile {
  /** Names its texts: samples.<id>.title / .detail. */
  id: 'switch' | 'wii' | 'psp' | 'unassigned'
  file: File
  /** The console the form starts with (none: to the unassigned section). */
  console?: string
}

export interface UploadHandle {
  /** Stops sending; the server keeps what it got (it can be resumed). */
  abort: () => void
}

/** Uploads and their jobs: extraction, validation and commit (RF-01 to RF-13, RF-27). */
export interface IngestionPorts {
  /** Resumable upload (tus): the bytes never pass through memory twice. */
  upload(file: File, options: UploadOptions, callbacks: UploadCallbacks): UploadHandle
  /** Starts a job that assigns an unassigned entry (RF-27a). */
  assign(entryId: number, spec: Omit<UploadSpec, 'group' | 'groupSize'>): Promise<UploadJob>
  jobs(): Promise<UploadJob[]>
  job(id: string): Promise<UploadJob>
  /** Live job changes. Returns a function that stops watching. */
  watchJobs(handlers: { onJob: (job: UploadJob) => void; onOpen?: () => void }): () => void
  files(jobId: string): Promise<StagedFile[]>
  submitPassword(jobId: string, password: string): Promise<UploadJob>
  /** Validates the files for another console without extracting again (RF-07). */
  changeConsole(jobId: string, console: string): Promise<UploadJob>
  /** Sets aside an upload that does not fit its console (RF-07). */
  resolve(jobId: string, action: ResolveAction): Promise<UploadJob>
  cancel(jobId: string): Promise<UploadJob>
  plan(jobId: string, request: CommitRequest): Promise<CommitPlan>
  commit(jobId: string, request: CommitRequest): Promise<CommitResult>
  /** Sample files to try the flow with; only adapters that have them (the demo). */
  samples?(): SampleFile[]
}

export const [IngestionContext, useIngestionPorts] = createPortContext<IngestionPorts>('Ingestion')
