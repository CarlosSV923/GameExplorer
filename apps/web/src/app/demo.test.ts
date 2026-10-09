import type { SampleFile } from '@/modules/ingestion/application/ports'
import type { UploadJob } from '@/modules/ingestion/domain/types'

import { createDemoServices } from './services'

type Demo = ReturnType<typeof createDemoServices>

/** Uploads a sample through the demo and lets every timer run (RF-62). */
async function uploadSample(demo: Demo, id: SampleFile['id'], console?: string) {
  const sample = demo.ingestion.samples?.().find((s) => s.id === id)
  if (!sample) throw new Error(`no sample ${id}`)
  let jobId = ''
  demo.ingestion.upload(
    sample.file,
    {
      spec: {
        console: console ?? sample.console ?? '',
        title: sample.file.name.replace(/\.\w+$/, ''),
      },
    },
    {
      onJobId: (j) => {
        jobId = j
      },
      onProgress: () => undefined,
      onSuccess: () => undefined,
      onError: (e) => {
        throw e
      },
    },
  )
  await vi.runAllTimersAsync()
  return demo.ingestion.job(jobId)
}

/** Uploads a file of the visitor's own (RF-63): only its name and size count. */
async function uploadOwn(demo: Demo, name: string, console: string, title: string) {
  let jobId = ''
  demo.ingestion.upload(
    new File(['x'], name),
    { spec: { console, title } },
    {
      onJobId: (j) => {
        jobId = j
      },
      onProgress: () => undefined,
      onSuccess: () => undefined,
      onError: (e) => {
        throw e
      },
    },
  )
  await vi.runAllTimersAsync()
  return demo.ingestion.job(jobId)
}

async function settle(demo: Demo, job: UploadJob) {
  await vi.runAllTimersAsync()
  return demo.ingestion.job(job.id)
}

describe('demo', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('opens with a seeded library and no login (RF-60, RF-65)', async () => {
    const demo = createDemoServices()
    await expect(demo.identity.session()).resolves.toBeDefined()
    const consoles = await demo.catalog.consoles.list()
    expect(consoles.map((c) => [c.slug, c.gameCount])).toEqual([
      ['n64', 4],
      ['gc', 4],
      ['wii', 4],
      ['switch', 7],
      ['ps2', 4],
      ['psp', 5],
    ])
    const entries = await demo.catalog.unassigned.list()
    expect(entries.map((e) => e.name).sort()).toEqual(['Inside - Edicion Mod', 'Mario.z64'])
    const suggestions = await demo.metadata.searchGames('zelda', 130)
    expect(suggestions[0]?.name).toBe('The Legend of Zelda: Breath of the Wild')
  })

  it('stores the Switch sample with its kinds and the duplicate base (RF-64)', async () => {
    const demo = createDemoServices()
    const job = await uploadSample(demo, 'switch')
    expect(job.status).toBe('confirm')
    const files = await demo.ingestion.files(job.id)
    const valid = files.filter((f) => f.valid).map((f) => f.path)
    expect(valid).toHaveLength(3)
    const [base, update, dlc] = valid as [string, string, string]
    const request = {
      files: [
        { path: base, kind: 'base' as const },
        { path: update, kind: 'update' as const, label: '3.0.3' },
        { path: dlc, kind: 'dlc' as const, label: 'Happy Home Paradise' },
      ],
    }
    // The title matches the seeded game: its base is a duplicate (RF-09).
    const plan = await demo.ingestion.plan(job.id, request)
    expect(plan.files.map((f) => f.action)).toEqual(['undecided', 'store', 'store'])
    await expect(demo.ingestion.commit(job.id, request)).rejects.toMatchObject({ kind: 'conflict' })
    const result = await demo.ingestion.commit(job.id, {
      files: request.files.map((f) =>
        f.kind === 'base' ? { ...f, onDuplicate: 'replace' as const } : f,
      ),
    })
    expect(result).toMatchObject({ stored: 3, replaced: 1 })
    expect(result.job.status).toBe('done')
  })

  it('asks the Wii sample for its password', async () => {
    const demo = createDemoServices()
    let job = await uploadSample(demo, 'wii')
    expect(job.status).toBe('needs_password')
    await demo.ingestion.submitPassword(job.id, 'nope')
    job = await settle(demo, job)
    expect(job).toMatchObject({ status: 'needs_password', error: 'Contraseña incorrecta.' })
    await demo.ingestion.submitPassword(job.id, 'demo')
    job = await settle(demo, job)
    expect(job.status).toBe('confirm')
  })

  it('lets the PSP sample change to Wii', async () => {
    const demo = createDemoServices()
    const job = await uploadSample(demo, 'psp')
    expect(job).toMatchObject({ status: 'invalid', invalidReason: 'none' })
    await expect(demo.ingestion.changeConsole(job.id, 'wii')).resolves.toMatchObject({
      status: 'confirm',
    })
  })

  it('sends an upload without console to the section and assigns it whole (RF-07b, RF-27a)', async () => {
    const demo = createDemoServices()
    const upload = await uploadSample(demo, 'unassigned')
    expect(upload.status).toBe('unassigned')
    const entry = (await demo.catalog.unassigned.list()).find((e) => e.name === 'Splatoon 3')
    expect(entry?.files).toHaveLength(3)
    if (!entry) return

    let job = await demo.ingestion.assign(entry.id, { console: 'switch', title: 'Splatoon 3' })
    job = await settle(demo, job)
    expect(job).toMatchObject({ status: 'confirm', fromUnassigned: true })
    expect((await demo.catalog.unassigned.list()).find((e) => e.id === entry.id)?.busy).toBe(true)
    const result = await demo.ingestion.commit(job.id, {
      files: [
        { path: 'Splatoon 3/Splatoon 3 [v0].xci', kind: 'base' },
        { path: 'Splatoon 3/Splatoon 3 [v2752512].nsp', skip: true },
      ],
    })
    expect(result.stored).toBe(1)
    const left = (await demo.catalog.unassigned.list()).find((e) => e.name === 'Splatoon 3')
    expect(left).toMatchObject({ busy: false })
    expect(left?.files.map((f) => f.name).sort()).toEqual([
      'LEEME.txt',
      'Splatoon 3 [v2752512].nsp',
    ])
  })

  it('refuses "No guardar" outside the unassigned section', async () => {
    const demo = createDemoServices()
    const job = await uploadSample(demo, 'switch')
    const files = await demo.ingestion.files(job.id)
    await expect(
      demo.ingestion.commit(job.id, {
        files: files.filter((f) => f.valid).map((f) => ({ path: f.path, skip: true })),
      }),
    ).rejects.toMatchObject({ kind: 'invalid' })
  })

  it('numbers the stored disc when another one arrives (RF-08a)', async () => {
    const demo = createDemoServices()
    const job = await uploadOwn(demo, 'sotc-2.iso', 'ps2', 'Shadow of the Colossus')
    expect(job.status).toBe('confirm')
    const disc2 = { path: 'sotc-2.iso', kind: 'disc' as const, label: '2' }
    await expect(demo.ingestion.commit(job.id, { files: [disc2] })).rejects.toMatchObject({
      kind: 'invalid',
    })
    const plan = await demo.ingestion.plan(job.id, { files: [disc2] })
    const stored = plan.existing[0]
    if (!stored) throw new Error('no stored disc')
    const result = await demo.ingestion.commit(job.id, {
      files: [disc2],
      renumber: [{ itemId: stored.id, label: '1' }],
    })
    const game = await demo.catalog.library.game(result.gameId)
    expect(game.items.map((i) => i.file)).toEqual([
      'Shadow of the Colossus (Disc 1).iso',
      'Shadow of the Colossus (Disc 2).iso',
    ])
  })
})
