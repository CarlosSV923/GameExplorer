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

import type { CommitRequest, CommitResult, UploadJob } from '../domain/types'
import { buildRows, mergeJobs } from '../domain/uploads'
import { useIngestionPorts } from './ports'
import type { UploadQueue } from './uploadQueue'

export const jobKeys = {
  all: ['jobs'] as const,
  job: (id: string) => ['jobs', id] as const,
  items: (id: string) => ['jobs', id, 'items'] as const,
}

export const [UploadQueueContext, useUploadQueue] = createPortContext<UploadQueue>('UploadQueue')

function upsert(client: QueryClient, job: UploadJob) {
  const known = client.getQueryData<UploadJob>(jobKeys.job(job.id))
  if (known && Date.parse(known.updatedAt) > Date.parse(job.updatedAt)) return // older, out of order
  client.setQueryData<UploadJob[]>(jobKeys.all, (list) => mergeJobs(list ?? [], [job]))
  client.setQueryData(jobKeys.job(job.id), job)
}

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
        if (job.status === 'done' && before?.status !== 'done') void invalidateLibrary(client)
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

export function useJobItems(id: string, enabled: boolean) {
  const ports = useIngestionPorts()
  return useQuery({
    queryKey: jobKeys.items(id),
    queryFn: () => ports.items(id),
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

export function useCancelJob() {
  const ports = useIngestionPorts()
  const client = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => ports.cancel(id),
    onSuccess: (job) => {
      upsert(client, job)
    },
  })
}

/** The preview of a commit (final names, duplicates), as the review changes. */
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
 * Stores a reviewed upload. onStored runs even if the review screen is gone
 * by then (the job's "done" event can arrive before the answer).
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
