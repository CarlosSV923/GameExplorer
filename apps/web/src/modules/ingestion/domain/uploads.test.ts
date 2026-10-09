import type { UploadJob } from './types'
import { activeRows, buildRows, mergeJobs, staleAfterMs, type LocalUpload } from './uploads'

const now = Date.parse('2026-10-07T12:00:00Z')
const at = (msAgo: number) => new Date(now - msAgo).toISOString()

const job = (over: Partial<UploadJob>): UploadJob => ({
  id: 'j1',
  fileName: 'mk8.rar',
  size: 100,
  received: 40,
  status: 'uploading',
  console: 'switch',
  title: 'Mario Kart 8 Deluxe',
  createdAt: at(60_000),
  updatedAt: at(1_000),
  ...over,
})

const local = (over: Partial<LocalUpload>): LocalUpload => ({
  key: 'l1',
  fileName: 'mk8.rar',
  size: 100,
  sent: 60,
  state: 'uploading',
  console: 'switch',
  title: 'Mario Kart 8 Deluxe',
  createdAt: now - 60_000,
  ...over,
})

describe('mergeJobs', () => {
  it('keeps the newest version of each job, newest job first', () => {
    const live = job({ id: 'a', status: 'confirm', updatedAt: at(1_000), createdAt: at(5_000) })
    const stale = job({ id: 'a', status: 'uploaded', updatedAt: at(3_000), createdAt: at(5_000) })
    const other = job({ id: 'b', createdAt: at(1_000) })
    expect(mergeJobs([live], [stale, other])).toEqual([other, live])
    expect(mergeJobs([stale], [live])).toEqual([live])
  })
})

describe('buildRows', () => {
  it('uses the local progress while this browser sends the file', () => {
    const [row] = buildRows([local({ jobId: 'j1' })], [job({})], now, new Set())
    expect(row).toMatchObject({ jobId: 'j1', localKey: 'l1', phase: 'uploading', sent: 60 })
  })

  it('marks a stale upload nobody here is sending as interrupted', () => {
    const fresh = buildRows([], [job({})], now, new Set())
    const stale = buildRows([], [job({ updatedAt: at(staleAfterMs + 1) })], now, new Set())
    expect(fresh[0]?.phase).toBe('uploadingElsewhere')
    expect(stale[0]?.phase).toBe('interrupted')
  })

  it('follows the server once the upload is complete', () => {
    const rows = buildRows(
      [local({ jobId: 'j1', state: 'finished', sent: 100 })],
      [job({ status: 'extracting', progress: 34 })],
      now,
      new Set(),
    )
    expect(rows).toEqual([expect.objectContaining({ phase: 'extracting', progress: 34 })])
  })

  it('lists queued uploads first and hides merged, cancelled, dismissed and old jobs', () => {
    const rows = buildRows(
      [local({ key: 'q', state: 'queued', sent: 0 })],
      [
        job({ id: 'a', status: 'confirm', createdAt: at(10) }),
        job({ id: 'b', status: 'merged' }),
        job({ id: 'c', status: 'cancelled' }),
        job({ id: 'd', status: 'failed' }),
        job({ id: 'e', status: 'done', updatedAt: at(25 * 3600 * 1000) }),
        job({ id: 'f', status: 'needs_password', createdAt: at(20) }),
        job({ id: 'g', status: 'trashed', updatedAt: at(25 * 3600 * 1000) }),
        job({ id: 'h', status: 'invalid', createdAt: at(30) }),
      ],
      now,
      new Set(['d']),
    )
    expect(rows.map((r) => [r.key, r.phase])).toEqual([
      ['q', 'queued'],
      ['a', 'confirm'],
      ['f', 'needsPassword'],
      ['h', 'invalid'],
    ])
  })

  it('counts as active everything but finished, set aside and failed uploads', () => {
    const rows = buildRows(
      [],
      [
        job({ id: 'a', status: 'done' }),
        job({ id: 'b', status: 'confirm' }),
        job({ id: 'c', status: 'unassigned' }),
      ],
      now,
      new Set(),
    )
    expect(activeRows(rows).map((r) => r.key)).toEqual(['b'])
  })
})
