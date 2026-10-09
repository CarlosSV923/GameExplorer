import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from '@tanstack/react-query'
import { useEffect, useMemo, useSyncExternalStore } from 'react'

import { invalidateLibrary } from '@/modules/catalog/application/queries'
import { createPortContext } from '@/shared/kernel/ports'
import { useDebounced, useNow } from '@/shared/kernel/hooks'

import type { UnassignedFile } from '@/modules/catalog/domain/types'

import type {
  CommitRequest,
  CommitResult,
  ResolveAction,
  UploadJob,
  UploadSpec,
} from '../domain/types'
import { buildRows, mergeJobs } from '../domain/uploads'
import { useIngestionPorts } from './ports'
import type { UploadQueue } from './uploadQueue'

export const jobKeys = {
  all: ['jobs'] as const,
  job: (id: string) => ['jobs', id] as const,
  files: (id: string) => ['jobs', id, 'files'] as const,
}

export const [UploadQueueContext, useUploadQueue] = createPortContext<UploadQueue>('UploadQueue')

/**
 * What the upload form opens with (RF-03): picked or dropped files with the
 * console of the screen, or an unassigned file to assign (RF-27).
 */
export type UploadFormRequest =
  { kind: 'files'; files: File[]; console?: string } | { kind: 'unassigned'; file: UnassignedFile }

/** Opens the upload form over the current screen (the app shell hosts it). */
export const [UploadFormContext, useOpenUploadForm] =
  createPortContext<(request: UploadFormRequest) => void>('UploadForm')

function upsert(client: QueryClient, job: UploadJob) {
  const known = client.getQueryData<UploadJob>(jobKeys.job(job.id))
  if (known && Date.parse(known.updatedAt) > Date.parse(job.updatedAt)) return // older, out of order
  client.setQueryData<UploadJob[]>(jobKeys.all, (list) => mergeJobs(list ?? [], [job]))
  client.setQueryData(jobKeys.job(job.id), job)
}

/** Statuses that change the library or the unassigned section when a job reaches them. */
const changesLibrary: ReadonlySet<UploadJob['status']> = new Set(['done', 'unassigned', 'trashed'])

/** Recent jobs, kept live by useJobFeed. */
export function useJobs() {
  const ports = useIngestionPorts()
  const client = useQueryClient()
  return useQuery({
    queryKey: jobKeys.all,
    // A list fetched before a live event must not overwrite it.
    queryFn: async () =>
      mergeJobs(client.getQueryData<UploadJob[]>(jobKeys.all) ?? [], await ports.jobs()),
    refetchInterval: 30_000,
  })
}

/**
 * Keeps the job queries live from the server's event stream (mounted once,
 * by the app shell). When a job is stored the library changed.
 */
export function useJobFeed(enabled: boolean) {
  const ports = useIngestionPorts()
  const client = useQueryClient()
  useEffect(() => {
    if (!enabled) return
    return ports.watchJobs({
      onJob: (job) => {
        const before = client.getQueryData<UploadJob>(jobKeys.job(job.id))
        upsert(client, job)
        if (changesLibrary.has(job.status) && before?.status !== job.status) {
          void invalidateLibrary(client)
        }
      },
      onOpen: () => {
        void client.invalidateQueries({ queryKey: jobKeys.all, exact: true })
      },
    })
  }, [enabled, ports, client])
}

export function useJob(id: string) {
  const ports = useIngestionPorts()
  return useQuery({ queryKey: jobKeys.job(id), queryFn: () => ports.job(id), retry: false })
}

/** The files of an upload; their validity changes only with the console. */
export function useJobFiles(id: string, enabled: boolean) {
  const ports = useIngestionPorts()
  return useQuery({
    queryKey: jobKeys.files(id),
    queryFn: () => ports.files(id),
    enabled,
    staleTime: Infinity,
  })
}

export function useSubmitPassword() {
  const ports = useIngestionPorts()
  const client = useQueryClient()
  return useMutation({
    mutationFn: ({ id, password }: { id: string; password: string }) =>
      ports.submitPassword(id, password),
    onSuccess: (job) => {
      upsert(client, job)
    },
  })
}

/** Validates the files for another console, without extracting again (RF-07). */
export function useChangeConsole(jobId: string) {
  const ports = useIngestionPorts()
  const client = useQueryClient()
  return useMutation({
    mutationFn: (console: string) => ports.changeConsole(jobId, console),
    onSuccess: async (job) => {
      // The files' validity changed: have it before the job shows its new state.
      await client.invalidateQueries({ queryKey: jobKeys.files(jobId) })
      upsert(client, job)
    },
  })
}

/** Sets aside an upload that does not fit (RF-07): the library may change. */
export function useResolveJob(jobId: string) {
  const ports = useIngestionPorts()
  const client = useQueryClient()
  return useMutation({
    mutationFn: (action: ResolveAction) => ports.resolve(jobId, action),
    onSuccess: async (job) => {
      upsert(client, job)
      await invalidateLibrary(client)
    },
  })
}

/** Starts a job from an unassigned file (RF-27). */
export function useAssign() {
  const ports = useIngestionPorts()
  const client = useQueryClient()
  return useMutation({
    mutationFn: ({ id, spec }: { id: number; spec: UploadSpec }) => ports.assign(id, spec),
    onSuccess: async (job) => {
      upsert(client, job)
      await invalidateLibrary(client)
    },
  })
}

export function useCancelJob() {
  const ports = useIngestionPorts()
  const client = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => ports.cancel(id),
    onSuccess: async (job) => {
      upsert(client, job)
      // An assigned file goes back to the unassigned section.
      if (job.fromUnassigned) await invalidateLibrary(client)
    },
  })
}

/** The preview of a commit (final names, duplicates), as the data changes. */
export function useCommitPlan(jobId: string, request: CommitRequest | undefined) {
  const ports = useIngestionPorts()
  const debounced = useDebounced(request, 400)
  return useQuery({
    queryKey: ['jobs', jobId, 'plan', debounced],
    queryFn: () => ports.plan(jobId, debounced as CommitRequest),
    enabled: debounced !== undefined,
    placeholderData: keepPreviousData,
    retry: false,
  })
}

/**
 * Stores a confirmed upload. onStored runs even if the screen is gone by
 * then (the job's "done" event can arrive before the answer).
 */
export function useCommit(jobId: string, onStored: (result: CommitResult) => void) {
  const ports = useIngestionPorts()
  const client = useQueryClient()
  return useMutation({
    mutationFn: (request: CommitRequest) => ports.commit(jobId, request),
    onSuccess: async (result) => {
      onStored(result)
      upsert(client, result.job)
      await invalidateLibrary(client)
    },
  })
}

/** The uploads panel: this browser's uploads merged with the server's jobs. */
export function useUploadRows() {
  const queue = useUploadQueue()
  const local = useSyncExternalStore(queue.subscribe, queue.getSnapshot)
  const jobs = useJobs()
  const now = useNow(5_000)
  const dismissed = queue.dismissedJobs()
  return useMemo(
    () => buildRows(local, jobs.data ?? [], now, dismissed),
    [local, jobs.data, now, dismissed],
  )
}
