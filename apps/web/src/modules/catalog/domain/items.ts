import type { GameSummary, ItemKind, LibraryItem } from './types'

/** The chip of an item: its kind and, for updates and discs, the label. */
export function itemChip(item: Pick<LibraryItem, 'kind' | 'label' | 'discNumber'>): {
  kind: ItemKind
  key: `kind.${ItemKind}` | 'kind.discNumber'
  label?: string
  number?: number
} {
  switch (item.kind) {
    case 'update':
      return { kind: 'update', key: 'kind.update', ...(item.label ? { label: item.label } : {}) }
    case 'disc':
      return item.discNumber
        ? { kind: 'disc', key: 'kind.discNumber', number: item.discNumber }
        : { kind: 'disc', key: 'kind.disc' }
    default:
      return { kind: item.kind, key: `kind.${item.kind}` }
  }
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

/** Normalizes an extension typed by the user: ".Z64", "z64" → ".z64". */
export function normalizeExtension(raw: string): string | null {
  const ext = raw.trim().toLowerCase().replace(/^\.*/, '.')
  return /^\.[a-z0-9][a-z0-9._-]{0,15}$/.test(ext) ? ext : null
}

/** A folder name suggested from a platform: "Wii U" → "wiiu". */
export function suggestSlug(name: string): string {
  return name
    .normalize('NFD')
    .replace(/\p{M}/gu, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '')
    .slice(0, 32)
}

export const slugPattern = /^[a-z0-9][a-z0-9_-]{0,31}$/
