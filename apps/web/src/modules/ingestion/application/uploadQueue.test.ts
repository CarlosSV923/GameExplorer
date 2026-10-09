import { AppError } from '@/shared/kernel/errors'

import type { UploadCallbacks, UploadOptions } from './ports'
import { UploadQueue } from './uploadQueue'

function fakePorts() {
  const started: { file: File; options: UploadOptions; cb: UploadCallbacks; abort: () => void }[] =
    []
  const ports = {
    upload: vi.fn((file: File, options: UploadOptions, cb: UploadCallbacks) => {
      const abort = vi.fn()
      started.push({ file, options, cb, abort })
      return { abort }
    }),
    cancel: vi.fn(() => Promise.resolve({} as never)),
  }
  return { ports, started }
}

const file = (name: string, size = 10) => new File([new Uint8Array(size)], name)
const spec = { console: 'switch', title: 'Limbo' }

describe('UploadQueue', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('sends two files at a time and starts the next when one finishes', () => {
    const { ports, started } = fakePorts()
    const queue = new UploadQueue(ports, 2)
    queue.add([file('a'), file('b'), file('c')].map((f) => ({ file: f, spec })))

    expect(started.map((s) => s.file.name)).toEqual(['a', 'b'])
    expect(started[0]?.options).toEqual({ spec })
    expect(queue.getSnapshot().map((u) => u.state)).toEqual(['uploading', 'uploading', 'queued'])
    expect(queue.busy()).toBe(true)

    started[0]?.cb.onJobId('job-a')
    started[0]?.cb.onProgress(5, 10)
    expect(queue.getSnapshot()[0]).toMatchObject({ jobId: 'job-a', sent: 5 })

    started[0]?.cb.onSuccess()
    expect(started.map((s) => s.file.name)).toEqual(['a', 'b', 'c'])
    expect(queue.getSnapshot()[0]).toMatchObject({ state: 'finished', sent: 10 })
  })

  it('keeps a failed upload for a retry with the same file', () => {
    const { ports, started } = fakePorts()
    const queue = new UploadQueue(ports, 1)
    queue.add([{ file: file('a'), spec }])
    started[0]?.cb.onJobId('job-a')
    started[0]?.cb.onError(new AppError('network'))
    expect(queue.getSnapshot()[0]?.state).toBe('error')
    expect(queue.busy()).toBe(false)

    queue.retry(queue.getSnapshot()[0]?.key ?? '')
    expect(started).toHaveLength(2)
    expect(started[1]?.options).toEqual({ spec, resumeJobId: 'job-a' })
  })

  it('resumes an interrupted job only with the same file', () => {
    const { ports, started } = fakePorts()
    const queue = new UploadQueue(ports)
    const job = { id: 'job-x', fileName: 'big.rar', size: 10, console: 'wii', title: 'Ōkami' }

    expect(queue.resume(job, file('other.rar', 10))).toBe(false)
    expect(queue.resume(job, file('big.rar', 9))).toBe(false)
    expect(queue.resume(job, file('big.rar', 10))).toBe(true)
    expect(started[0]?.options).toEqual({
      spec: { console: 'wii', title: 'Ōkami' },
      resumeJobId: 'job-x',
    })
  })

  it('cancels: stops sending, frees the slot and deletes the server copy', async () => {
    const { ports, started } = fakePorts()
    const queue = new UploadQueue(ports, 1)
    queue.add([file('a'), file('b')].map((f) => ({ file: f, spec })))
    started[0]?.cb.onJobId('job-a')

    await queue.cancel(queue.getSnapshot()[0]?.key ?? '')
    expect(started[0]?.abort).toHaveBeenCalled()
    expect(ports.cancel).toHaveBeenCalledWith('job-a')
    expect(started[1]?.file.name).toBe('b')
    expect(queue.getSnapshot()).toHaveLength(1)
  })

  it('remembers dismissed jobs in this browser', () => {
    const { ports } = fakePorts()
    new UploadQueue(ports).dismiss('job-a')
    expect(new UploadQueue(ports).dismissedJobs().has('job-a')).toBe(true)
  })
})
