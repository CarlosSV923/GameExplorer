import { DetailedError, Upload } from 'tus-js-client'

import type { ApiClient } from '@/shared/api/client'
import { problemError, unwrap } from '@/shared/api/result'
import { AppError } from '@/shared/kernel/errors'

import type { IngestionPorts } from '../../application/ports'
import type { UploadJob } from '../../domain/types'

function uploadError(error: Error): AppError {
  if (error instanceof DetailedError && error.originalResponse) {
    return problemError(error.originalResponse.getStatus(), undefined)
  }
  return new AppError('network', error.message)
}

export function createIngestionHttp(client: ApiClient, baseUrl = '/api'): IngestionPorts {
  const id = (jobId: string) => ({ params: { path: { id: jobId } } })
  return {
    upload: (file, options, cb) => {
      const upload = new Upload(file, {
        endpoint: `${baseUrl}/uploads/`,
        ...(options.resumeJobId ? { uploadUrl: `${baseUrl}/uploads/${options.resumeJobId}` } : {}),
        metadata: {
          filename: file.name,
          ...(options.consoleSlug ? { consoleSlug: options.consoleSlug } : {}),
        },
        // The server keeps what it got: a dropped connection resumes from there.
        retryDelays: [0, 1000, 3000, 5000, 10_000, 20_000],
        storeFingerprintForResuming: false,
        onUploadUrlAvailable: () => {
          const url = upload.url
          if (url) cb.onJobId(url.slice(url.lastIndexOf('/') + 1))
        },
        onProgress: cb.onProgress,
        onSuccess: () => {
          cb.onSuccess()
        },
        onError: (error) => {
          cb.onError(uploadError(error))
        },
      })
      upload.start()
      return {
        abort: () => {
          void upload.abort(false)
        },
      }
    },
    jobs: () => unwrap(client.GET('/jobs')),
    job: (jobId) => unwrap(client.GET('/jobs/{id}', id(jobId))),
    watchJobs: ({ onJob, onOpen }) => {
      const source = new EventSource(`${baseUrl}/jobs/events`, { withCredentials: true })
      const listener = (event: MessageEvent<string>) => {
        onJob(JSON.parse(event.data) as UploadJob)
      }
      source.addEventListener('job', listener)
      if (onOpen) source.addEventListener('open', onOpen)
      return () => {
        source.close()
      }
    },
    items: (jobId) => unwrap(client.GET('/jobs/{id}/items', id(jobId))),
    submitPassword: (jobId, password) =>
      unwrap(client.POST('/jobs/{id}/password', { ...id(jobId), body: { password } })),
    cancel: (jobId) => unwrap(client.POST('/jobs/{id}/cancel', id(jobId))),
    plan: (jobId, body) => unwrap(client.POST('/jobs/{id}/plan', { ...id(jobId), body })),
    commit: (jobId, body) => unwrap(client.POST('/jobs/{id}/commit', { ...id(jobId), body })),
  }
}
