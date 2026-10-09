import { DetailedError, Upload } from 'tus-js-client'

import type { ApiClient } from '@/shared/api/client'
import { problemError, unwrap } from '@/shared/api/result'
import { AppError } from '@/shared/kernel/errors'

import type { IngestionPorts } from '../../application/ports'
import type { UploadJob, UploadSpec } from '../../domain/types'

function uploadError(error: Error): AppError {
  if (error instanceof DetailedError && error.originalResponse) {
    return problemError(error.originalResponse.getStatus(), undefined)
  }
  return new AppError('network', error.message)
}

/** The tus Upload-Metadata keys (api/openapi.yaml). */
function metadata(fileName: string, spec: UploadSpec): Record<string, string> {
  return {
    filename: fileName,
    // Without console the upload goes to the unassigned section (RF-07b).
    ...(spec.console === '' ? {} : { consoleSlug: spec.console }),
    title: spec.title,
    ...(spec.igdbId === undefined ? {} : { igdbId: String(spec.igdbId) }),
    ...(spec.group === undefined ? {} : { group: spec.group }),
    ...(spec.groupSize === undefined ? {} : { groupSize: String(spec.groupSize) }),
  }
}

export function createIngestionHttp(client: ApiClient, baseUrl = '/api'): IngestionPorts {
  const id = (jobId: string) => ({ params: { path: { id: jobId } } })
  return {
    upload: (file, options, cb) => {
      const upload = new Upload(file, {
        endpoint: `${baseUrl}/uploads/`,
        ...(options.resumeJobId ? { uploadUrl: `${baseUrl}/uploads/${options.resumeJobId}` } : {}),
        metadata: metadata(file.name, options.spec),
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
    assign: (entryId, body) =>
      unwrap(
        client.POST('/unassigned/entries/{id}/assign', { params: { path: { id: entryId } }, body }),
      ),
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
    files: (jobId) => unwrap(client.GET('/jobs/{id}/files', id(jobId))),
    submitPassword: (jobId, password) =>
      unwrap(client.POST('/jobs/{id}/password', { ...id(jobId), body: { password } })),
    changeConsole: (jobId, console) =>
      unwrap(client.POST('/jobs/{id}/console', { ...id(jobId), body: { console } })),
    resolve: (jobId, action) =>
      unwrap(client.POST('/jobs/{id}/resolve', { ...id(jobId), body: { action } })),
    cancel: (jobId) => unwrap(client.POST('/jobs/{id}/cancel', id(jobId))),
    plan: (jobId, body) => unwrap(client.POST('/jobs/{id}/plan', { ...id(jobId), body })),
    commit: (jobId, body) => unwrap(client.POST('/jobs/{id}/commit', { ...id(jobId), body })),
  }
}
