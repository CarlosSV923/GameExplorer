import { AppError } from '@/shared/kernel/errors'

import type { CatalogPorts } from '../../application/ports'
import { consoleExtensions, knownExtensions } from '../../domain/items'
import { extensionOf, gameFolder, itemFileName, normalizeExtension } from '../../domain/naming'
import type {
  Console,
  DuplicateAction,
  GameDetail,
  GameEditPlan,
  GameSummary,
  ItemKind,
  LibraryItem,
  PlannedAction,
  ScanReport,
  TrashEntry,
  UnassignedEntry,
  UnassignedFile,
  UnassignedReason,
} from '../../domain/types'
import { downloadNote } from './downloads'

/** What the demo knows of an IGDB game (from the fixed catalog). */
export interface DemoGameInfo {
  id: number
  name: string
  releaseYear?: number | null
  coverImageId?: string | null
  summary?: string | null
  genres: string[]
}

/** A console defined in code, as the server has it (spec §6). */
export interface DemoConsoleDef {
  slug: string
  name: string
  igdbPlatformId: number
  releaseYear: number
  logoImageId: string | null
  extensions: string[]
  kinds: ItemKind[]
  multipleFiles: boolean
}

interface Game {
  id: number
  console: string
  title: string
  folder: string
  igdbId: number | null
  releaseYear: number | null
  coverImageId: string | null
  summary: string | null
  genres: string[]
}

interface Item extends LibraryItem {
  gameId: number
  trash: number | null
}

interface Loose extends UnassignedFile {
  job: string | null
  trash: number | null
}

interface Trash {
  id: number
  gameId: number | null
  wholeGame: boolean
  reason: 'deleted' | 'replaced'
  trashedAt: string
}

/** A file handed to the library by an upload (ingestion's commit). */
export interface NewFile {
  ref: string
  /** Name of the file (its extension counts). */
  name: string
  size: number
  kind?: ItemKind
  label?: string
  onDuplicate?: DuplicateAction
  /** Set when it comes straight from the unassigned section. */
  unassigned?: number
}

export interface StoreRequest {
  console: string
  title: string
  igdbId?: number | null
  files: NewFile[]
  /** Disc numbers for files already in the game (RF-08a). */
  renumber?: { itemId: number; label: string }[]
}

export interface StorePlan {
  console: string
  title: string
  folder: string
  gameId: number | null
  existing: LibraryItem[]
  files: { path: string; file: string; action: PlannedAction; duplicate?: LibraryItem | null }[]
  renamed: { item: LibraryItem; file: string }[]
}

const retentionMs = 30 * 24 * 3600 * 1000

const invalid = (detail: string) => new AppError('invalid', detail)
const conflict = (detail: string) => new AppError('conflict', detail)
const notFound = () => new AppError('notFound')

/**
 * The library of the demo (RF-60): consoles, games, the unassigned section
 * and the trash, in memory for the session. It follows the server's rules
 * (spec §3 and §5) closely enough for every screen to behave as it would.
 */
export class DemoLibrary {
  private consoles: Console[]
  private games = new Map<number, Game>()
  private items = new Map<number, Item>()
  private loose = new Map<number, Loose>()
  private trashes = new Map<number, Trash>()
  private lastScan: ScanReport | null = null
  private nextId = 1

  constructor(
    defs: readonly DemoConsoleDef[],
    private readonly gameInfo: (id: number) => DemoGameInfo | undefined,
  ) {
    this.consoles = defs.map((d, i) => ({
      slug: d.slug,
      displayName: d.name,
      defaultName: d.name,
      igdbPlatformId: d.igdbPlatformId,
      releaseYear: d.releaseYear,
      logoImageId: d.logoImageId,
      extensions: d.extensions,
      customExtensions: [],
      kinds: d.kinds,
      multipleFiles: d.multipleFiles,
      sortOrder: i,
      gameCount: 0,
    }))
  }

  private id(): number {
    return this.nextId++
  }

  private now(): string {
    return new Date().toISOString()
  }

  // ---------- seeding ----------

  /** Adds a stored game (seed). */
  seedGame(
    console: string,
    igdbId: number,
    files: { kind: ItemKind; label?: string; ext: string; size: number }[],
  ) {
    const game = this.newGame(console, { title: '', igdbId })
    for (const f of files) {
      const name = itemFileName({ title: game.title, kind: f.kind, label: f.label ?? '' }, f.ext)
      if (!name.ok) throw new Error(`seed ${game.title}: ${name.field}`)
      this.addItem(game.id, f.kind, f.label ?? null, name.name, f.size)
    }
  }

  /** Adds a file to the unassigned section (seed). */
  seedLoose(path: string, size: number, reason: UnassignedReason, console?: string) {
    this.addLoose(path, size, reason, path, console)
  }

  // ---------- helpers ----------

  console(slug: string): Console {
    const c = this.consoles.find((x) => x.slug === slug)
    if (!c) throw invalid(`La consola «${slug}» no existe.`)
    return c
  }

  allConsoles(): Console[] {
    return this.consoles
  }

  private known(): string[] {
    return knownExtensions(this.consoles)
  }

  private liveItems(gameId: number): Item[] {
    return [...this.items.values()].filter((i) => i.gameId === gameId && i.trash === null)
  }

  private gameByFolder(console: string, folder: string): Game | undefined {
    return [...this.games.values()].find(
      (g) => g.console === console && g.folder.toLowerCase() === folder.toLowerCase(),
    )
  }

  /** The game a name lands in: its title and IGDB data (RF-11). */
  private describe(name: { title: string; igdbId?: number | null }) {
    let title = name.title.trim()
    const info = name.igdbId ? this.gameInfo(name.igdbId) : undefined
    if (name.igdbId && !info) throw invalid('Ese juego no está en el catálogo de la demo.')
    if (info && !title) title = info.name
    const folder = gameFolder(title)
    if (!folder.ok) throw invalid('Escribe el nombre del juego.')
    return { title, folder: folder.name, info }
  }

  private newGame(console: string, name: { title: string; igdbId?: number | null }): Game {
    const { title, folder, info } = this.describe(name)
    const game: Game = {
      id: this.id(),
      console,
      title,
      folder,
      igdbId: info?.id ?? null,
      releaseYear: info?.releaseYear ?? null,
      coverImageId: info?.coverImageId ?? null,
      summary: info?.summary ?? null,
      genres: info?.genres ?? [],
    }
    this.games.set(game.id, game)
    return game
  }

  private addItem(
    gameId: number,
    kind: ItemKind,
    label: string | null,
    file: string,
    size: number,
  ) {
    const item: Item = {
      id: this.id(),
      gameId,
      kind,
      label,
      file,
      size,
      createdAt: this.now(),
      trash: null,
    }
    this.items.set(item.id, item)
    return item
  }

  private addLoose(
    path: string,
    size: number,
    reason: UnassignedReason,
    origin: string,
    console?: string,
    igdbId?: number | null,
  ): Loose {
    const f: Loose = {
      id: this.id(),
      path,
      name: path.slice(path.lastIndexOf('/') + 1),
      origin,
      reason,
      size,
      arrivedAt: this.now(),
      consoles: [],
      archive: /\.(zip|7z|rar)(\.\d{1,3})?$/i.test(path),
      ...(console ? { console } : {}),
      ...(igdbId ? { igdbId } : {}),
      job: null,
      trash: null,
    }
    this.loose.set(f.id, f)
    return f
  }

  private liveLoose(): Loose[] {
    return [...this.loose.values()].filter((f) => f.trash === null)
  }

  /** rel, or rel with " (2)", " (3)"… when the section already has it. */
  private freePath(rel: string): string {
    const taken = new Set(this.liveLoose().map((f) => f.path.toLowerCase()))
    const dot = rel.lastIndexOf('.')
    const slash = rel.lastIndexOf('/')
    const [stem, ext] = dot > slash + 1 ? [rel.slice(0, dot), rel.slice(dot)] : [rel, '']
    for (let i = 1; ; i++) {
      const candidate = i === 1 ? rel : `${stem} (${String(i)})${ext}`
      if (!taken.has(candidate.toLowerCase())) return candidate
    }
  }

  /** The section's folder with that name, in any case, or the name itself. */
  private entryFolder(folder: string): string {
    const match = this.liveLoose().find(
      (f) => f.path.includes('/') && f.path.split('/')[0]?.toLowerCase() === folder.toLowerCase(),
    )
    return match ? (match.path.split('/')[0] ?? folder) : folder
  }

  private summary(g: Game): GameSummary {
    const items = this.liveItems(g.id)
    return {
      id: g.id,
      igdbId: g.igdbId,
      console: g.console,
      title: g.title,
      folder: g.folder,
      releaseYear: g.releaseYear,
      coverImageId: g.coverImageId,
      itemCount: items.length,
      size: items.reduce((s, i) => s + i.size, 0),
    }
  }

  private detail(id: number): GameDetail {
    const g = this.games.get(id)
    const items = g ? this.liveItems(id) : []
    if (!g || items.length === 0) throw notFound()
    const order: Record<ItemKind, number> = { base: 0, game: 0, disc: 0, update: 1, dlc: 2 }
    items.sort(
      (a, b) =>
        order[a.kind] - order[b.kind] ||
        (a.label ?? '').localeCompare(b.label ?? '', undefined, { numeric: true }),
    )
    return {
      ...this.summary(g),
      path: `${g.console}/${g.folder}`,
      summary: g.summary,
      genres: g.genres,
      items: items.map(toItem),
    }
  }

  private trashItems(items: Item[], wholeGame: boolean, reason: Trash['reason']) {
    const first = items[0]
    if (!first) return
    const entry: Trash = {
      id: this.id(),
      gameId: first.gameId,
      wholeGame,
      reason,
      trashedAt: this.now(),
    }
    this.trashes.set(entry.id, entry)
    for (const it of items) it.trash = entry.id
  }

  private dropGameIfEmpty(gameId: number) {
    if (![...this.items.values()].some((i) => i.gameId === gameId)) this.games.delete(gameId)
  }

  private unassign(g: Game, items: Item[]) {
    const folder = this.entryFolder(g.folder)
    for (const it of items) {
      this.addLoose(
        this.freePath(`${folder}/${it.file}`),
        it.size,
        'manual',
        `${g.console}/${g.folder}/${it.file}`,
        g.console,
        g.igdbId,
      )
      this.items.delete(it.id)
    }
    this.dropGameIfEmpty(g.id)
  }

  private entryOf(id: number): Loose[] {
    const f = this.loose.get(id)
    if (!f || f.trash !== null) throw notFound()
    const key = entryKey(f.path).toLowerCase()
    const files = this.liveLoose().filter((o) => entryKey(o.path).toLowerCase() === key)
    if (files.some((o) => o.job !== null))
      throw conflict('Una asignación en curso está usando estos archivos.')
    return files
  }

  private trashLoose(files: Loose[]) {
    const entry: Trash = {
      id: this.id(),
      gameId: null,
      wholeGame: false,
      reason: 'deleted',
      trashedAt: this.now(),
    }
    this.trashes.set(entry.id, entry)
    for (const f of files) f.trash = entry.id
  }

  private freeFile(id: number): Loose {
    const f = this.loose.get(id)
    if (!f || f.trash !== null) throw notFound()
    if (f.job !== null) throw conflict('Una asignación en curso está usando este archivo.')
    return f
  }

  // ---------- ingestion side ----------

  /** Names, folder and duplicates of a commit (RF-08, RF-09). */
  plan(req: StoreRequest): StorePlan {
    const console = this.console(req.console)
    const { title, folder } = this.describe(req)
    const game = this.gameByFolder(console.slug, folder)
    const existing = game ? this.liveItems(game.id) : []
    const exts = consoleExtensions(console)
    // Stored files that become numbered discs keep their place under the new name.
    const renamed = (req.renumber ?? []).map((r) => {
      const it = existing.find((e) => e.id === r.itemId)
      if (!it || !console.kinds.includes('disc')) {
        throw invalid('Solo los discos de un juego guardado se pueden numerar.')
      }
      const name = itemFileName(
        { title: game?.title ?? title, kind: 'disc', label: r.label },
        extensionOf(it.file, this.known()),
      )
      if (!name.ok) throw invalid('Escribe un número de disco del 1 al 99.')
      return { item: toItem(it), file: name.name }
    })
    const current = existing.map((e) => {
      const r = renamed.find((x) => x.item.id === e.id)
      return r ? { ...e, file: r.file } : e
    })
    const replaced = new Set<number>()
    const files = req.files.map((f) => {
      const ext = extensionOf(f.name, this.known())
      if (!exts.includes(ext))
        throw invalid(`«${f.ref}» no es un archivo de ${console.displayName}.`)
      // One disc is the game itself (spec §5).
      const kind =
        f.kind ??
        (console.kinds.length === 1 || console.kinds.includes('game')
          ? console.kinds[0]
          : undefined)
      if (!kind || !console.kinds.includes(kind)) throw invalid(`Falta el tipo de «${f.ref}».`)
      const name = itemFileName({ title: game?.title ?? title, kind, label: f.label ?? '' }, ext)
      if (!name.ok) throw invalid(`Revisa los datos de «${f.ref}».`)
      const duplicate = current.find((e) => e.file.toLowerCase() === name.name.toLowerCase())
      let action: PlannedAction = 'store'
      if (duplicate) {
        action = f.onDuplicate ?? 'undecided'
        if (action === 'replace') {
          if (replaced.has(duplicate.id)) throw invalid('Dos archivos reemplazarían al mismo.')
          replaced.add(duplicate.id)
        }
      }
      return {
        path: f.ref,
        file: name.name,
        action,
        duplicate: duplicate ? toItem(duplicate) : null,
      }
    })
    const names = files.filter((f) => f.action !== 'skip').map((f) => f.file.toLowerCase())
    if (new Set(names).size !== names.length)
      throw invalid('Dos archivos tendrían el mismo nombre.')
    return {
      console: console.slug,
      title: game?.title ?? title,
      folder: game?.folder ?? folder,
      gameId: game?.id ?? null,
      existing: existing.map(toItem),
      files,
      renamed,
    }
  }

  /** Stores a commit (RF-10); replaced files go to the trash. */
  store(req: StoreRequest) {
    const plan = this.plan(req)
    if (plan.files.some((f) => f.action === 'undecided')) {
      throw conflict('Hay archivos que ya existen: elige Reemplazar u Omitir.')
    }
    // A game of several files has every disc numbered (spec §5).
    if (this.console(plan.console).kinds.includes('disc')) {
      const renamedIds = new Set(plan.renamed.map((r) => r.item.id))
      const replacedIds = new Set(
        plan.files.flatMap((f) => (f.action === 'replace' && f.duplicate ? [f.duplicate.id] : [])),
      )
      const kinds = [
        ...plan.existing
          .filter((e) => !replacedIds.has(e.id))
          .map((e) => (renamedIds.has(e.id) ? 'disc' : e.kind)),
        ...plan.files.flatMap((f, i) =>
          f.action === 'skip' ? [] : [req.files[i]?.kind ?? 'game'],
        ),
      ]
      if (kinds.length > 1 && kinds.some((k) => k !== 'disc')) {
        throw invalid(
          'Un juego de varios discos necesita el número de cada disco, también del que ya está guardado.',
        )
      }
    }
    let game = plan.gameId === null ? undefined : this.games.get(plan.gameId)
    if (!game) game = this.newGame(plan.console, req)
    else if (game.igdbId === null && req.igdbId) {
      const info = this.gameInfo(req.igdbId)
      if (info) {
        Object.assign(game, {
          igdbId: info.id,
          releaseYear: info.releaseYear ?? null,
          coverImageId: info.coverImageId ?? null,
          summary: info.summary ?? null,
          genres: info.genres,
        })
      }
    }
    for (const r of plan.renamed) {
      const it = this.items.get(r.item.id)
      const label = (req.renumber ?? []).find((x) => x.itemId === r.item.id)?.label.trim() ?? '1'
      if (it) Object.assign(it, { kind: 'disc', label: String(Number(label)), file: r.file })
    }
    let stored = 0
    let replaced = 0
    let skipped = 0
    plan.files.forEach((p, i) => {
      const f = req.files[i]
      if (!f) return
      if (p.action === 'skip') {
        skipped++
        return
      }
      if (p.action === 'replace' && p.duplicate) {
        const old = this.items.get(p.duplicate.id)
        if (old) this.trashItems([old], false, 'replaced')
        replaced++
      }
      const kind = f.kind ?? 'game'
      this.addItem(game.id, kind, f.label?.trim() || null, p.file, f.size)
      if (f.unassigned !== undefined) this.loose.delete(f.unassigned)
      stored++
    })
    return { gameId: game.id, path: `${game.console}/${game.folder}`, stored, replaced, skipped }
  }

  /** Puts files in the unassigned section, under a folder (RF-07, RF-07b). */
  putAside(req: {
    files: { path: string; size: number }[]
    folder: string
    origin: string
    console?: string
    igdbId?: number | null
    toTrash?: boolean
  }) {
    const folder = gameFolder(req.folder)
    const base = folder.ok ? this.entryFolder(folder.name) : ''
    const added = req.files.map((f) =>
      this.addLoose(
        this.freePath(base ? `${base}/${f.path}` : f.path),
        f.size,
        'upload',
        req.origin,
        req.console,
        req.igdbId,
      ),
    )
    if (req.toTrash) this.trashLoose(added)
  }

  /** Marks an entry as used by an assignment (RF-27a). */
  takeEntry(id: number, job: string): { name: string; files: Loose[] } {
    const files = this.entryOf(id)
    for (const f of files) f.job = job
    return { name: entryKey(files[0]?.path ?? ''), files }
  }

  jobFiles(job: string): Loose[] {
    return this.liveLoose().filter((f) => f.job === job)
  }

  releaseJob(job: string) {
    for (const f of this.jobFiles(job)) f.job = null
  }

  deleteFiles(ids: number[]) {
    for (const id of ids) this.loose.delete(id)
  }

  // ---------- ports ----------

  ports(): CatalogPorts {
    const ok = <T>(fn: () => T): Promise<T> => {
      try {
        return Promise.resolve(fn())
      } catch (e) {
        return Promise.reject(e instanceof Error ? e : new Error(String(e)))
      }
    }
    const consoles = () =>
      this.consoles
        .map((c) => ({
          ...c,
          gameCount: [...this.games.values()].filter(
            (g) => g.console === c.slug && this.liveItems(g.id).length > 0,
          ).length,
          customExtensions: c.customExtensions.map((e) => ({
            ...e,
            fileCount: [...this.items.values()].filter(
              (i) => i.trash === null && i.file.toLowerCase().endsWith(e.extension),
            ).length,
          })),
        }))
        .sort((a, b) => a.sortOrder - b.sortOrder)
    const one = (slug: string) => {
      const c = consoles().find((x) => x.slug === slug)
      if (!c) throw notFound()
      return c
    }
    const live = () => [...this.games.values()].filter((g) => this.liveItems(g.id).length > 0)
    const byTitle = (a: Game, b: Game) => a.title.localeCompare(b.title)

    return {
      consoles: {
        list: () => ok(consoles),
        rename: (slug, name) =>
          ok(() => {
            const c = this.console(slug)
            c.displayName = name.trim() || c.defaultName
            return one(slug)
          }),
        reorder: (slugs) =>
          ok(() => {
            if (slugs.length !== this.consoles.length) throw invalid('Faltan consolas en el orden.')
            slugs.forEach((s, i) => {
              this.console(s).sortOrder = i
            })
            return consoles()
          }),
        addExtension: (slug, raw) =>
          ok(() => {
            const ext = normalizeExtension(raw)
            if (!ext) throw invalid('Esa no es una extensión válida.')
            if (this.known().includes(ext)) throw conflict('Esa extensión ya la tiene una consola.')
            this.console(slug).customExtensions.push({ extension: ext, fileCount: 0 })
            return one(slug)
          }),
        removeExtension: (slug, ext) =>
          ok(() => {
            const c = one(slug)
            const custom = c.customExtensions.find((e) => e.extension === ext)
            if (!custom) throw conflict('Esa extensión no se puede quitar desde la app.')
            if (custom.fileCount > 0) throw conflict('Hay archivos con esa extensión.')
            const own = this.console(slug)
            own.customExtensions = own.customExtensions.filter((e) => e.extension !== ext)
            return one(slug)
          }),
      },
      library: {
        consoleGames: (slug) =>
          ok(() =>
            live()
              .filter((g) => g.console === slug)
              .sort(byTitle)
              .map((g) => this.summary(g)),
          ),
        search: (query) =>
          ok(() => {
            const words = fold(query).split(/\s+/).filter(Boolean)
            return live()
              .filter((g) => words.every((w) => fold(g.title).includes(w)))
              .sort(byTitle)
              .map((g) => this.summary(g))
          }),
        game: (id) => ok(() => this.detail(id)),
        trashGame: (id) =>
          ok(() => {
            this.trashItems(this.liveItems(this.detail(id).id), true, 'deleted')
          }),
        trashItem: (id) =>
          ok(() => {
            const it = this.items.get(id)
            if (!it || it.trash !== null) throw notFound()
            this.trashItems([it], false, 'deleted')
          }),
        unassignGame: (id) =>
          ok(() => {
            const g = this.games.get(this.detail(id).id)
            if (g) this.unassign(g, this.liveItems(g.id))
          }),
        unassignItem: (id) =>
          ok(() => {
            const it = this.items.get(id)
            const g = it ? this.games.get(it.gameId) : undefined
            if (!it || !g || it.trash !== null) throw notFound()
            this.unassign(g, [it])
          }),
        planEdit: (gameId, edit) => ok(() => this.planEdit(gameId, edit)),
        edit: (gameId, edit) =>
          ok(() => {
            const plan = this.planEdit(gameId, edit)
            if (plan.items.some((i) => i.action === 'undecided')) {
              throw conflict('Hay archivos que ya existen en el destino.')
            }
            const g = this.games.get(gameId)
            if (!g) throw notFound()
            let target = g
            if (plan.mergeInto) {
              const other = this.games.get(plan.mergeInto)
              if (other) target = other
            } else {
              const { info } = this.describe(edit)
              Object.assign(g, {
                console: plan.console,
                title: plan.title,
                folder: plan.folder,
                igdbId: info?.id ?? null,
                releaseYear: info?.releaseYear ?? null,
                coverImageId: info?.coverImageId ?? null,
                summary: info?.summary ?? null,
                genres: info?.genres ?? [],
              })
            }
            for (const p of plan.items) {
              const it = this.items.get(p.item.id)
              if (!it) continue
              if (p.action === 'skip') {
                this.trashItems([it], false, 'replaced')
                continue
              }
              if (p.action === 'replace' && p.duplicate) {
                const old = this.items.get(p.duplicate.id)
                if (old) this.trashItems([old], false, 'replaced')
              }
              it.gameId = target.id
              it.file = p.file
            }
            for (const t of this.trashes.values()) if (t.gameId === g.id) t.gameId = target.id
            if (target.id !== g.id) {
              for (const it of this.items.values()) if (it.gameId === g.id) it.gameId = target.id
              this.games.delete(g.id)
            }
            return {
              gameId: target.id,
              path: `${target.console}/${target.folder}`,
              merged: target.id !== g.id,
            }
          }),
        editItem: (itemId, edit) =>
          ok(() => {
            const it = this.items.get(itemId)
            const g = it ? this.games.get(it.gameId) : undefined
            if (!it || !g || it.trash !== null) throw notFound()
            const c = this.console(g.console)
            if (!c.kinds.includes(edit.kind)) throw invalid('Ese tipo no existe en esta consola.')
            const ext = extensionOf(it.file, this.known())
            const name = itemFileName(
              { title: g.title, kind: edit.kind, label: edit.label ?? '' },
              ext,
            )
            if (!name.ok) throw invalid('Revisa la versión o el nombre del DLC.')
            const dup = this.liveItems(g.id).find(
              (o) => o.id !== it.id && o.file.toLowerCase() === name.name.toLowerCase(),
            )
            if (dup) {
              if (!edit.onDuplicate) throw conflict('Ya hay un archivo con ese nombre.')
              if (edit.onDuplicate === 'skip') return this.detail(g.id)
              this.trashItems([dup], false, 'replaced')
            }
            Object.assign(it, {
              kind: edit.kind,
              label: edit.label?.trim() || null,
              file: name.name,
            })
            return this.detail(g.id)
          }),
        lastScan: () => ok(() => this.lastScan),
        scan: () =>
          ok(() => {
            // Without a share there is nothing to find (RF-60).
            this.lastScan = { scannedAt: this.now(), unassigned: 0, removed: 0, pending: 0 }
            return this.lastScan
          }),
        gameDownloadUrl: (id) => {
          const g = this.games.get(id)
          return downloadNote(`${g?.folder ?? 'juego'}.zip`)
        },
        itemDownloadUrl: (id) => downloadNote(this.items.get(id)?.file ?? 'archivo'),
      },
      unassigned: {
        list: () => ok(() => this.entries()),
        trash: (id) =>
          ok(() => {
            this.trashLoose([this.freeFile(id)])
          }),
        remove: (id) =>
          ok(() => {
            this.loose.delete(this.freeFile(id).id)
          }),
        downloadUrl: (id) => downloadNote(this.loose.get(id)?.name ?? 'archivo'),
        trashEntry: (id) =>
          ok(() => {
            this.trashLoose(this.entryOf(id))
          }),
        removeEntry: (id) =>
          ok(() => {
            this.deleteFiles(this.entryOf(id).map((f) => f.id))
          }),
        entryDownloadUrl: (id) => {
          const f = this.loose.get(id)
          return downloadNote(
            f ? entryKey(f.path) + (f.path.includes('/') ? '.zip' : '') : 'archivo',
          )
        },
      },
      trash: {
        list: () => ok(() => this.trashList()),
        restore: (id, replace) => ok(() => this.restore(id, replace)),
        remove: (id) =>
          ok(() => {
            this.dropTrash(id)
          }),
        empty: () =>
          ok(() => {
            for (const id of [...this.trashes.keys()]) this.dropTrash(id)
          }),
      },
    }
  }

  private entries(): UnassignedEntry[] {
    const byKey = new Map<string, UnassignedEntry>()
    for (const f of this.liveLoose()) {
      const key = entryKey(f.path)
      let e = byKey.get(key.toLowerCase())
      if (!e) {
        e = {
          id: f.id,
          name: key,
          folder: f.path.includes('/'),
          size: 0,
          arrivedAt: f.arrivedAt,
          copying: false,
          busy: false,
          files: [],
        }
        byKey.set(key.toLowerCase(), e)
      }
      e.files.push(this.view(f))
      e.size += f.size
      e.id = Math.min(e.id, f.id)
      e.busy = e.busy || f.job !== null
      if (f.arrivedAt > e.arrivedAt) e.arrivedAt = f.arrivedAt
      if (!e.console && f.console) e.console = f.console
      if (!e.igdbId && f.igdbId) e.igdbId = f.igdbId
    }
    return [...byKey.values()]
      .map((e) => ({ ...e, files: e.files.sort((a, b) => a.path.localeCompare(b.path)) }))
      .sort((a, b) => b.arrivedAt.localeCompare(a.arrivedAt) || a.name.localeCompare(b.name))
  }

  /** A file of the section as the screens see it. */
  private view(f: Loose): UnassignedFile {
    return {
      id: f.id,
      path: f.path,
      name: f.name,
      origin: f.origin,
      reason: f.reason,
      size: f.size,
      arrivedAt: f.arrivedAt,
      consoles: this.accepting(f.name),
      archive: f.archive,
      ...(f.console ? { console: f.console } : {}),
      ...(f.igdbId ? { igdbId: f.igdbId } : {}),
    }
  }

  private accepting(name: string): string[] {
    const ext = extensionOf(name, this.known())
    return ext
      ? this.consoles.filter((c) => consoleExtensions(c).includes(ext)).map((c) => c.slug)
      : []
  }

  private planEdit(
    gameId: number,
    edit: {
      console: string
      title: string
      igdbId?: number | null
      decisions?: { itemId: number; onDuplicate: DuplicateAction }[]
    },
  ): GameEditPlan {
    const g = this.games.get(gameId)
    const items = g ? this.liveItems(gameId) : []
    if (!g || items.length === 0) throw notFound()
    const c = this.console(edit.console)
    const { title, folder } = this.describe(edit)
    const other = this.gameByFolder(c.slug, folder)
    const merge = other && other.id !== g.id ? other : undefined
    const finalTitle = merge?.title ?? title
    const existing = merge ? this.liveItems(merge.id) : []
    const decide = new Map((edit.decisions ?? []).map((d) => [d.itemId, d.onDuplicate]))
    return {
      console: c.slug,
      title: finalTitle,
      folder: merge?.folder ?? folder,
      mergeInto: merge?.id ?? null,
      items: items.map((it) => {
        const ext = extensionOf(it.file, this.known())
        if (!consoleExtensions(c).includes(ext) || !c.kinds.includes(it.kind)) {
          throw invalid(`«${it.file}» no vale para ${c.displayName}.`)
        }
        const name = itemFileName({ title: finalTitle, kind: it.kind, label: it.label ?? '' }, ext)
        if (!name.ok) throw invalid('Revisa el nombre del juego.')
        const dup = existing.find((e) => e.file.toLowerCase() === name.name.toLowerCase())
        const action: PlannedAction = dup ? (decide.get(it.id) ?? 'undecided') : 'store'
        return { item: toItem(it), file: name.name, action, duplicate: dup ? toItem(dup) : null }
      }),
    }
  }

  private trashList(): TrashEntry[] {
    return [...this.trashes.values()]
      .map((t): TrashEntry => {
        const items = [...this.items.values()].filter((i) => i.trash === t.id).map(toItem)
        const files = [...this.loose.values()]
          .filter((f) => f.trash === t.id)
          .map((f) => this.view(f))
        const g = t.gameId === null ? undefined : this.games.get(t.gameId)
        return {
          id: t.id,
          kind: g ? 'game' : 'unassigned',
          gameId: g?.id ?? null,
          console: g?.console ?? null,
          title: g?.title ?? '',
          folder: g?.folder ?? null,
          wholeGame: t.wholeGame,
          reason: t.reason,
          trashedAt: t.trashedAt,
          expiresAt: new Date(Date.parse(t.trashedAt) + retentionMs).toISOString(),
          size: [...items, ...files].reduce((s, x) => s + x.size, 0),
          items,
          files,
        }
      })
      .sort((a, b) => b.trashedAt.localeCompare(a.trashedAt))
  }

  private restore(id: number, replace: boolean) {
    const t = this.trashes.get(id)
    if (!t) throw notFound()
    const items = [...this.items.values()].filter((i) => i.trash === id)
    const files = [...this.loose.values()].filter((f) => f.trash === id)
    if (t.gameId === null || items.length === 0) {
      for (const f of files) {
        f.path = this.freePath(f.path)
        f.trash = null
      }
      this.trashes.delete(id)
      return { path: '_unassigned' }
    }
    const g = this.games.get(t.gameId)
    if (!g) throw notFound()
    const live = this.liveItems(g.id)
    const taken = items.flatMap((it) =>
      live.filter((l) => l.file.toLowerCase() === it.file.toLowerCase()),
    )
    if (taken.length > 0) {
      if (!replace) throw conflict('Su lugar está ocupado.')
      this.trashItems(taken, false, 'replaced')
    }
    for (const it of items) it.trash = null
    this.trashes.delete(id)
    return { gameId: g.id, path: `${g.console}/${g.folder}` }
  }

  private dropTrash(id: number) {
    const t = this.trashes.get(id)
    if (!t) throw notFound()
    for (const it of [...this.items.values()]) if (it.trash === id) this.items.delete(it.id)
    for (const f of [...this.loose.values()]) if (f.trash === id) this.loose.delete(f.id)
    this.trashes.delete(id)
    if (t.gameId !== null) this.dropGameIfEmpty(t.gameId)
  }
}

function toItem(it: Item): LibraryItem {
  return {
    id: it.id,
    kind: it.kind,
    label: it.label ?? null,
    file: it.file,
    size: it.size,
    createdAt: it.createdAt,
  }
}

/** The entry of the section a path belongs to: its first folder, or the file. */
export function entryKey(path: string): string {
  const i = path.indexOf('/')
  return i < 0 ? path : path.slice(0, i)
}

function fold(text: string): string {
  return text.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase()
}
