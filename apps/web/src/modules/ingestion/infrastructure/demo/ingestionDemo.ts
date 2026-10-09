import {
  consoleExtensions,
  knownExtensions,
  looksLikeArchive,
} from '@/modules/catalog/domain/items'
import { extensionOf } from '@/modules/catalog/domain/naming'
import type {
  Console,
  DuplicateAction,
  ItemKind,
  LibraryItem,
} from '@/modules/catalog/domain/types'
import { AppError } from '@/shared/kernel/errors'

import type { IngestionPorts, SampleFile } from '../../application/ports'
import type {
  CommitPlan,
  CommitRequest,
  InvalidReason,
  JobStatus,
  StagedFile,
  UploadJob,
  UploadSpec,
} from '../../domain/types'

/** The library as the demo's ingestion uses it (the catalog's DemoLibrary). */
export interface DemoLibraryPort {
  allConsoles(): Console[]
  plan(req: DemoStoreRequest): {
    console: string
    title: string
    folder: string
    gameId: number | null
    existing: LibraryItem[]
    files: CommitPlan['files']
  }
  store(req: DemoStoreRequest): {
    gameId: number
    path: string
    stored: number
    replaced: number
    skipped: number
  }
  putAside(req: {
    files: { path: string; size: number }[]
    folder: string
    origin: string
    console?: string
    igdbId?: number | null
    toTrash?: boolean
  }): void
  takeEntry(id: number, job: string): { name: string; files: DemoLoose[] }
  jobFiles(job: string): DemoLoose[]
  releaseJob(job: string): void
  deleteFiles(ids: number[]): void
}

export interface DemoLoose {
  id: number
  path: string
  name: string
  size: number
}

export interface DemoStoreRequest {
  console: string
  title: string
  igdbId?: number | null
  files: {
    ref: string
    name: string
    size: number
    kind?: ItemKind
    label?: string
    onDuplicate?: DuplicateAction
    unassigned?: number
  }[]
}

const GB = 1_000_000_000
const MB = 1_000_000

/** What the demo's archives hold (RF-63, RF-64): by name, lower case. */
interface Archive {
  files: { path: string; size: number }[]
  password?: string
}

/** The sample archives (RF-64); the password of the Wii one is shown as a hint. */
const sampleArchives: Record<string, Archive> = {
  'animal crossing - new horizons.rar': {
    files: [
      { path: 'Animal Crossing - New Horizons [01006F8002326000][v0].nsp', size: 6.7 * GB },
      { path: 'Animal Crossing - New Horizons [01006F8002327000][v3.0.3].nsp', size: 4.7 * GB },
      { path: 'Animal Crossing - New Horizons [DLC] Happy Home Paradise.nsp', size: 624 * MB },
      { path: 'leeme.txt', size: 2_000 },
    ],
  },
  'zelda twilight princess.7z': {
    files: [{ path: 'The Legend of Zelda - Twilight Princess.wbfs', size: 3.9 * GB }],
    password: 'demo',
  },
  'okami.zip': {
    files: [
      { path: 'Okami (PAL)/Okami.wbfs', size: 4.2 * GB },
      { path: 'Okami (PAL)/instrucciones.pdf', size: 3 * MB },
    ],
  },
  'splatoon 3.zip': {
    files: [
      { path: 'Splatoon 3 [v0].xci', size: 6.1 * GB },
      { path: 'Splatoon 3 [v2752512].nsp', size: 2.4 * GB },
      { path: 'LEEME.txt', size: 1_500 },
    ],
  },
}

/** A sample's File: no bytes, only a name and a size (RF-63). */
function fakeFile(name: string, size: number): File {
  const file = new File([], name)
  Object.defineProperty(file, 'size', { value: size })
  return file
}

function stem(name: string): string {
  return name.replace(/(\.part\d+)?\.(zip|7z|rar)(\.\d{1,3})?$/i, '')
}

interface Staged extends StagedFile {
  /** For files of an assigned entry that stay where they are. */
  unassigned?: number
}

interface JobState {
  job: UploadJob
  /** The content an extraction would find. */
  archive: Archive | null
  files: Staged[]
  /** Group of a multi-volume archive. */
  group?: string
  /** Ids of the entry's archives, deleted once stored. */
  entryArchives: number[]
  entryFolder: boolean
}

const failed = (detail: string) => new AppError('invalid', detail)

/**
 * Uploads without a server (RF-62, RF-63): the bytes never leave the
 * browser; upload, extraction, password, validation and commit follow the
 * server's state machine with timers.
 */
export function createIngestionDemo(library: DemoLibraryPort): IngestionPorts {
  const jobs = new Map<string, JobState>()
  const watchers = new Set<(job: UploadJob) => void>()

  const now = () => new Date().toISOString()
  const newId = () => crypto.randomUUID().replaceAll('-', '')
  const copy = (j: UploadJob): UploadJob => ({ ...j })
  const publish = (s: JobState) => {
    s.job.updatedAt = now()
    for (const w of watchers) w(copy(s.job))
  }
  const state = (id: string) => {
    const s = jobs.get(id)
    if (!s) throw new AppError('notFound')
    return s
  }
  const move = (s: JobState, status: JobStatus, patch: Partial<UploadJob> = {}) => {
    Object.assign(s.job, { status }, patch)
    publish(s)
  }
  const later = (ms: number, fn: () => void) => setTimeout(fn, ms)
  const consoles = () => library.allConsoles()
  const known = () => knownExtensions(consoles())

  /** The archive an upload holds: a sample, or one game file for the console (RF-63). */
  const archiveOf = (name: string, console: string): Archive | null => {
    const sample = sampleArchives[name.toLowerCase()]
    if (sample) return sample
    if (!looksLikeArchive(name)) return null
    const target = consoles().find((c) => c.slug === console)
    const ext = target ? (consoleExtensions(target)[0] ?? '.bin') : '.bin'
    return { files: [{ path: stem(name) + ext, size: 1 }] }
  }

  const validity = (s: JobState): InvalidReason | null => {
    const target = consoles().find((c) => c.slug === s.job.console)
    const exts = target ? consoleExtensions(target) : []
    const valid = s.files.filter((f) => exts.includes(extensionOf(f.path, known())))
    if (valid.length === 0) return 'none'
    if (valid.length > 1 && target && !target.multipleFiles && !s.job.fromUnassigned) return 'many'
    return null
  }

  /** The files found: to the unassigned section without console (RF-07b), else validated (RF-07). */
  const staged = (s: JobState) => {
    if (s.job.console === '' && !s.job.fromUnassigned) {
      library.putAside({
        files: s.files.map((f) => ({ path: f.path, size: f.size })),
        folder: s.job.title,
        origin: s.job.fileName,
        igdbId: s.job.igdbId ?? null,
      })
      move(s, 'unassigned', { progress: 100 })
      return
    }
    const reason = validity(s)
    move(s, reason ? 'invalid' : 'confirm', { invalidReason: reason, progress: 100, error: null })
  }

  const extract = (s: JobState, password: string) => {
    const archive = s.archive
    if (!archive) return
    if (archive.password && archive.password !== password) {
      move(s, 'needs_password', {
        progress: 0,
        error: password ? 'Contraseña incorrecta.' : 'El comprimido está protegido con contraseña.',
      })
      return
    }
    move(s, 'extracting', { progress: 0, error: null })
    const total = archive.files.reduce((n, f) => n + f.size, 0)
    const ms = Math.min(4000, Math.max(1200, (total / GB) * 400))
    let pct = 0
    const tick = () => {
      if (s.job.status !== 'extracting') return
      pct = Math.min(100, pct + 20)
      s.job.progress = pct
      publish(s)
      if (pct < 100) {
        later(ms / 5, tick)
        return
      }
      const prefix = s.entryFolder && s.job.fromUnassigned ? `${s.job.fileName}/` : ''
      s.files = [
        ...archive.files.map((f) => ({
          path: prefix + f.path,
          size: f.size,
          valid: false,
          consoles: [],
        })),
        ...s.files.filter((f) => f.unassigned !== undefined),
      ]
      staged(s)
    }
    later(ms / 5, tick)
  }

  /** After the upload: wait for the other parts, extract, or take the file as it is. */
  const process = (s: JobState) => {
    if (s.group) {
      const parts = [...jobs.values()].filter((o) => o.group === s.group)
      const size = s.job.groupSize ?? 0
      move(s, 'waiting_parts')
      if (parts.filter((o) => o.job.status === 'waiting_parts').length < size) return
      parts.sort((a, b) => a.job.fileName.localeCompare(b.job.fileName))
      const [first, ...rest] = parts
      if (!first) return
      for (const p of rest) move(p, 'merged', { mergedInto: first.job.id })
      first.archive = archiveOf(first.job.fileName, first.job.console)
      move(first, 'uploaded')
      extract(first, '')
      return
    }
    if (s.archive) {
      extract(s, '')
      return
    }
    s.files = [{ path: s.job.fileName, size: s.job.size, valid: false, consoles: [] }]
    staged(s)
  }

  const filesOf = (s: JobState): StagedFile[] => {
    const all = consoles()
    const target = all.find((c) => c.slug === s.job.console)
    const exts = target ? consoleExtensions(target) : []
    return s.files.map((f) => {
      const ext = extensionOf(f.path, known())
      return {
        path: f.path,
        size: f.size,
        valid: exts.includes(ext),
        consoles: all.filter((c) => consoleExtensions(c).includes(ext)).map((c) => c.slug),
        ...(f.unassigned === undefined ? {} : { inPlace: true }),
      }
    })
  }

  const storeRequest = (s: JobState, req: CommitRequest): DemoStoreRequest => {
    const valid = filesOf(s).filter((f) => f.valid)
    const byPath = new Map(s.files.map((f) => [f.path, f]))
    for (const f of valid) {
      if (!req.files.some((c) => c.path === f.path))
        throw failed(`Faltan los datos de «${f.path}».`)
    }
    const kept = req.files.filter((c) => !c.skip)
    if (req.files.some((c) => c.skip) && !s.job.fromUnassigned) {
      throw failed('Solo los archivos de No asignados se pueden dejar sin guardar.')
    }
    const target = consoles().find((c) => c.slug === s.job.console)
    if (s.job.fromUnassigned) {
      if (kept.length === 0) throw failed('Elige al menos un archivo para guardar.')
      if (kept.length > 1 && target && !target.multipleFiles) {
        throw failed('Esta consola guarda un solo archivo por juego.')
      }
    }
    return {
      console: s.job.console,
      title: s.job.title,
      igdbId: s.job.igdbId ?? null,
      files: kept.map((c) => {
        const f = byPath.get(c.path)
        if (!f || !valid.some((v) => v.path === c.path))
          throw failed(`«${c.path}» no es de esta subida.`)
        return {
          ref: c.path,
          name: f.path,
          size: f.size,
          ...(c.kind ? { kind: c.kind } : {}),
          ...(c.label ? { label: c.label } : {}),
          ...(c.onDuplicate ? { onDuplicate: c.onDuplicate } : {}),
          ...(f.unassigned === undefined ? {} : { unassigned: f.unassigned }),
        }
      }),
    }
  }

  /** A stored assignment: extracted leftovers stay in the entry, its archives go (RF-27a). */
  const finishEntry = (s: JobState, stored: Set<string>) => {
    const id = s.job.id
    const leftovers = s.files.filter((f) => f.unassigned === undefined && !stored.has(f.path))
    if (leftovers.length > 0) {
      library.putAside({
        files: leftovers.map((f) => ({ path: f.path, size: f.size })),
        folder: s.entryFolder ? '' : s.job.title,
        origin: s.job.fileName,
        console: s.job.console,
        igdbId: s.job.igdbId ?? null,
      })
    }
    library.deleteFiles(s.entryArchives)
    library.releaseJob(id)
  }

  const run = <T>(fn: () => T): Promise<T> => {
    try {
      return Promise.resolve(fn())
    } catch (e) {
      return Promise.reject(e instanceof Error ? e : new Error(String(e)))
    }
  }

  const newJob = (
    fileName: string,
    size: number,
    spec: UploadSpec,
    status: JobStatus,
  ): UploadJob => ({
    id: newId(),
    fileName,
    size,
    received: 0,
    status,
    console: spec.console,
    title: spec.title.trim(),
    igdbId: spec.igdbId ?? null,
    invalidReason: null,
    progress: 0,
    error: null,
    warning: null,
    groupSize: spec.groupSize ?? null,
    mergedInto: null,
    createdAt: now(),
    updatedAt: now(),
  })

  const samples: SampleFile[] = [
    {
      id: 'switch',
      console: 'switch',
      file: fakeFile('Animal Crossing - New Horizons.rar', 11.2 * GB),
    },
    { id: 'wii', console: 'wii', file: fakeFile('Zelda Twilight Princess.7z', 3.1 * GB) },
    { id: 'psp', console: 'psp', file: fakeFile('Okami.zip', 3.8 * GB) },
    { id: 'unassigned', file: fakeFile('Splatoon 3.zip', 7.9 * GB) },
  ]

  return {
    upload: (file, options, cb) => {
      let s = options.resumeJobId ? jobs.get(options.resumeJobId) : undefined
      if (!s) {
        const job = newJob(file.name, file.size, options.spec, 'uploading')
        s = {
          job,
          archive: options.spec.group ? null : archiveOf(file.name, options.spec.console),
          files: [],
          entryArchives: [],
          entryFolder: false,
          ...(options.spec.group ? { group: options.spec.group } : {}),
        }
        jobs.set(job.id, s)
        publish(s)
      }
      const current = s
      cb.onJobId(current.job.id)
      // Proportional to the size, a few seconds at most (RF-62).
      const ms = Math.min(5000, Math.max(1500, (file.size / GB) * 500))
      const step = Math.max(1, Math.ceil(file.size / (ms / 100)))
      let stopped = false
      const tick = () => {
        if (stopped || current.job.status !== 'uploading') return
        current.job.received = Math.min(current.job.size, current.job.received + step)
        cb.onProgress(current.job.received, current.job.size)
        if (current.job.received < current.job.size) {
          later(100, tick)
          return
        }
        move(current, 'uploaded')
        cb.onSuccess()
        later(300, () => {
          process(current)
        })
      }
      later(100, tick)
      return {
        abort: () => {
          stopped = true
        },
      }
    },
    assign: (entryId, spec) =>
      run(() => {
        if (!spec.console) throw failed('Elige una consola.')
        const id = newId()
        const entry = library.takeEntry(entryId, id)
        const job: UploadJob = {
          ...newJob(
            entry.name,
            entry.files.reduce((n, f) => n + f.size, 0),
            spec,
            'uploaded',
          ),
          id,
          fromUnassigned: true,
        }
        const archives = entry.files.filter((f) => looksLikeArchive(f.name))
        const s: JobState = {
          job,
          archive: null,
          files: entry.files
            .filter((f) => !looksLikeArchive(f.name))
            .map((f) => ({
              path: f.path,
              size: f.size,
              valid: false,
              consoles: [],
              unassigned: f.id,
            })),
          entryArchives: archives.map((a) => a.id),
          entryFolder: entry.files.some((f) => f.path.includes('/')),
        }
        if (archives.length > 0) {
          s.archive = {
            files: archives.flatMap((a) => archiveOf(a.name, spec.console)?.files ?? []),
          }
        }
        jobs.set(id, s)
        publish(s)
        later(300, () => {
          if (s.archive) extract(s, '')
          else staged(s)
        })
        return copy(job)
      }),
    jobs: () =>
      Promise.resolve(
        [...jobs.values()]
          .map((s) => copy(s.job))
          .sort((a, b) => b.createdAt.localeCompare(a.createdAt)),
      ),
    job: (id) => run(() => copy(state(id).job)),
    watchJobs: ({ onJob, onOpen }) => {
      watchers.add(onJob)
      onOpen?.()
      return () => {
        watchers.delete(onJob)
      }
    },
    files: (id) => run(() => filesOf(state(id))),
    submitPassword: (id, password) =>
      run(() => {
        const s = state(id)
        if (s.job.status !== 'needs_password') throw new AppError('conflict')
        later(200, () => {
          extract(s, password)
        })
        return copy(s.job)
      }),
    changeConsole: (id, console) =>
      run(() => {
        const s = state(id)
        if (s.job.status !== 'confirm' && s.job.status !== 'invalid') throw new AppError('conflict')
        s.job.console = console
        const reason = validity(s)
        move(s, reason ? 'invalid' : 'confirm', { invalidReason: reason })
        return copy(s.job)
      }),
    resolve: (id, action) =>
      run(() => {
        const s = state(id)
        if (s.job.status !== 'invalid') throw new AppError('conflict')
        if (s.job.fromUnassigned || action === 'delete') {
          library.releaseJob(id)
          move(s, 'cancelled')
          return copy(s.job)
        }
        library.putAside({
          files: s.files.map((f) => ({ path: f.path, size: f.size })),
          folder: s.job.title,
          origin: s.job.fileName,
          console: s.job.console,
          igdbId: s.job.igdbId ?? null,
          toTrash: action === 'trash',
        })
        move(s, action === 'trash' ? 'trashed' : 'unassigned')
        return copy(s.job)
      }),
    cancel: (id) =>
      run(() => {
        const s = state(id)
        if (
          ['done', 'committing', 'failed', 'cancelled', 'merged', 'unassigned', 'trashed'].includes(
            s.job.status,
          )
        ) {
          throw new AppError('conflict')
        }
        library.releaseJob(id)
        move(s, 'cancelled')
        return copy(s.job)
      }),
    plan: (id, req) =>
      run(() => {
        const s = state(id)
        if (s.job.status !== 'confirm') throw new AppError('conflict')
        const plan = library.plan(storeRequest(s, req))
        const kept = new Set(plan.files.map((f) => f.path))
        return {
          ...plan,
          discarded: filesOf(s)
            .filter((f) => !f.valid && !kept.has(f.path))
            .map((f) => f.path),
        }
      }),
    commit: (id, req) =>
      run(() => {
        const s = state(id)
        if (s.job.status !== 'confirm') throw new AppError('conflict')
        const request = storeRequest(s, req)
        const result = library.store(request)
        move(s, 'done')
        if (s.job.fromUnassigned) finishEntry(s, new Set(request.files.map((f) => f.ref)))
        return { job: copy(s.job), ...result }
      }),
    samples: () => samples,
  }
}
