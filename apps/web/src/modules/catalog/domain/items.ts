import { extensionOf } from './naming'
import type { Console, GameSummary, ItemKind, LibraryItem } from './types'

/** Slug of the unassigned section in the carousel (never a console). */
export const unassignedSlug = '_unassigned'

/** The chip of an item: its kind and, for updates and discs, the version or number. */
export function itemChip(item: Pick<LibraryItem, 'kind' | 'label'>): {
  kind: Exclude<ItemKind, 'disc'>
  key: `kind.${ItemKind}`
  label?: string
} {
  if (item.kind === 'update' && item.label) {
    return { kind: 'update', key: 'kind.update', label: `v${item.label}` }
  }
  // A disc looks like the game it is part of.
  if (item.kind === 'disc')
    return { kind: 'game', key: 'kind.disc', ...(item.label ? { label: item.label } : {}) }
  return { kind: item.kind, key: `kind.${item.kind}` }
}

/** Games of a search grouped by console, in carousel order. */
export function groupByConsole<T extends Pick<GameSummary, 'console'>>(
  games: readonly T[],
  order: readonly string[],
): { console: string; games: T[] }[] {
  const groups = new Map<string, T[]>()
  for (const g of games) {
    const list = groups.get(g.console) ?? []
    list.push(g)
    groups.set(g.console, list)
  }
  const rank = (slug: string) => {
    const i = order.indexOf(slug)
    return i < 0 ? order.length : i
  }
  return [...groups.entries()]
    .sort(([a], [b]) => rank(a) - rank(b))
    .map(([console, list]) => ({ console, games: list }))
}

/** Every extension a console accepts: the fixed ones and the custom ones. */
export function consoleExtensions(c: Pick<Console, 'extensions' | 'customExtensions'>): string[] {
  return [...c.extensions, ...c.customExtensions.map((e) => e.extension)]
}

/** Every extension of every console: a file's extension is the longest match among them. */
export function knownExtensions(consoles: readonly Console[]): string[] {
  return [...new Set(consoles.flatMap(consoleExtensions))]
}

/** A file's extension (RF-41): ".nkit.iso" rather than ".iso" when some console knows it. */
export function fileExtension(name: string, consoles: readonly Console[]): string {
  return extensionOf(name, knownExtensions(consoles))
}

/** The consoles that accept a file by its extension. */
export function acceptingConsoles(name: string, consoles: readonly Console[]): Console[] {
  const ext = fileExtension(name, consoles)
  return ext ? consoles.filter((c) => consoleExtensions(c).includes(ext)) : []
}

/**
 * Archives the server extracts (zip, 7z, rar and their volumes). The server
 * recognizes them by their bytes; the forms only guess from the name.
 */
export function looksLikeArchive(name: string): boolean {
  return /\.(zip|7z|rar)(\.\d{1,3})?$/i.test(name)
}

/** Whether a console takes one file per game (Wii, PSP) rather than base, update and DLC. */
export function singleFile(c: Pick<Console, 'kinds'>): boolean {
  return !c.kinds.some((k) => k !== 'game')
}

/**
 * The consoles a game can move to (RF-24): every file's extension and kind
 * must be valid there. Others come with the extension that blocks them.
 */
export function moveTargets(
  items: readonly Pick<LibraryItem, 'file' | 'kind'>[],
  from: string,
  consoles: readonly Console[],
): { console: Console; blockedBy?: string }[] {
  return consoles
    .filter((c) => c.slug !== from)
    .map((c) => {
      const exts = consoleExtensions(c)
      const bad = items.find((it) => {
        const ext = fileExtension(it.file, consoles)
        return !ext || !exts.includes(ext) || !c.kinds.includes(it.kind)
      })
      if (!bad) return { console: c }
      return { console: c, blockedBy: fileExtension(bad.file, consoles) || bad.file }
    })
}
