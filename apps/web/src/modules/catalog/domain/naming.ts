import type { ItemKind } from './types'

/**
 * File and folder names (docs/spec.md §5). Go is the authority
 * (catalog/domain/naming.go); this copy previews names in the forms and
 * names files in the demo. Both pass contracts/naming-cases.json.
 */

/** The field a name was rejected for. */
export type NameField = 'title' | 'label' | 'version' | 'disc' | 'kind' | 'extension'

export type NameResult = { ok: true; name: string } | { ok: false; field: NameField }

/** NAME_MAX on Linux file systems (ZFS included). */
const maxNameBytes = 255

// Go's unicode.IsSpace.
const space = /[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]/gu
const dropped = /[\p{Cc}?<>"|*]/gu
const extensionPattern = /^(\.[a-z0-9]{1,10}){1,3}$/
const versionPattern = /^[0-9]+(\.[0-9]+)*$/

function bytes(s: string): number {
  return new TextEncoder().encode(s).length
}

/**
 * A game title (or a DLC name) that every file system and SMB client
 * accepts; "" when nothing is left.
 */
export function sanitizeTitle(raw: string): string {
  return raw
    .normalize('NFC')
    .replace(/[:꞉]/g, ' -')
    .replace(/[/\\]/g, ' ')
    .replace(space, ' ')
    .replace(dropped, '')
    .split(' ')
    .filter(Boolean)
    .join(' ')
    .replace(/[ .]+$/, '')
}

/** "1.0.4", "v1.0.4", "122345" → the stored version, without the v; null if invalid. */
export function normalizeVersion(raw: string): string | null {
  const v = raw.trim().replace(/^v/, '').replace(/^V/, '')
  return v.length <= 32 && versionPattern.test(v) ? v : null
}

/** " 01 " → "1": a disc number from 1 to 99, without leading zeros; null if invalid. */
export function normalizeDisc(raw: string): string | null {
  const d = raw.trim()
  if (!/^[0-9]{1,3}$/.test(d)) return null
  const n = d.replace(/^0+/, '')
  return n && n.length <= 2 ? n : null
}

/** ".Z64", "z64" → ".z64"; null when it is not an extension. Several dots are fine. */
export function normalizeExtension(raw: string): string | null {
  let ext = raw.trim().toLowerCase()
  if (!ext.startsWith('.')) ext = `.${ext}`
  return extensionPattern.test(ext) ? ext : null
}

/**
 * The longest of `known` the file name ends with ("Game.nkit.iso" is
 * ".nkit.iso" when it is known, even if ".iso" is too), or "" (RF-41).
 */
export function extensionOf(name: string, known: readonly string[]): string {
  const lower = name.toLowerCase()
  let best = ''
  for (const ext of known) {
    if (ext.length > best.length && lower.length > ext.length && lower.endsWith(ext)) best = ext
  }
  return best
}

/** The game's folder inside its console folder: the sanitized title (RF-11). */
export function gameFolder(title: string): NameResult {
  const folder = sanitizeTitle(title)
  if (!folder || bytes(folder) > maxNameBytes) return { ok: false, field: 'title' }
  return { ok: true, name: folder }
}

export interface ItemName {
  title: string
  kind: ItemKind
  /** Update version or DLC name, as typed; ignored for base and game. */
  label?: string
}

/** The file name without extension: "Limbo [UPDATE v1.0.4]". */
export function itemStem({ title, kind, label = '' }: ItemName): NameResult {
  const clean = sanitizeTitle(title)
  if (!clean) return { ok: false, field: 'title' }
  switch (kind) {
    case 'game':
      return { ok: true, name: clean }
    case 'base':
      return { ok: true, name: `${clean} [BASE]` }
    case 'update': {
      const v = normalizeVersion(label)
      return v === null
        ? { ok: false, field: 'version' }
        : { ok: true, name: `${clean} [UPDATE v${v}]` }
    }
    case 'dlc': {
      const name = sanitizeTitle(label)
      return name ? { ok: true, name: `${clean} [DLC ${name}]` } : { ok: false, field: 'label' }
    }
    case 'disc': {
      const d = normalizeDisc(label)
      return d === null ? { ok: false, field: 'disc' } : { ok: true, name: `${clean} (Disc ${d})` }
    }
    default:
      return { ok: false, field: 'kind' }
  }
}

/** The file name inside the game folder: the stem plus the lower-cased extension. */
export function itemFileName(item: ItemName, extension: string): NameResult {
  const stem = itemStem(item)
  if (!stem.ok) return stem
  const ext = extension.toLowerCase()
  if (!extensionPattern.test(ext)) return { ok: false, field: 'extension' }
  const name = stem.name + ext
  return bytes(name) > maxNameBytes ? { ok: false, field: 'title' } : { ok: true, name }
}

/** The data of a file's kind as typed in a form (FileTypeChips + VersionField). */
export interface KindDraft {
  kind: ItemKind | undefined
  /** Update version as typed: digits and dots. */
  version: string
  dlcName: string
  /** Disc number as typed (GameCube, PS2). */
  disc?: string
}

export type KindDraftError = 'kind' | 'version' | 'dlcName' | 'disc'

/** What is missing for the file to be named. */
export function kindDraftError(d: KindDraft): KindDraftError | null {
  switch (d.kind) {
    case undefined:
      return 'kind'
    case 'update':
      return normalizeVersion(d.version) === null ? 'version' : null
    case 'dlc':
      return sanitizeTitle(d.dlcName) ? null : 'dlcName'
    case 'disc':
      return normalizeDisc(d.disc ?? '') === null ? 'disc' : null
    default:
      return null
  }
}

/** The label sent to the server: the version without v, or the DLC name. */
export function kindDraftLabel(d: KindDraft): string | undefined {
  if (d.kind === 'update') return normalizeVersion(d.version) ?? undefined
  if (d.kind === 'dlc') return d.dlcName.trim().replace(/\s+/g, ' ') || undefined
  if (d.kind === 'disc') return normalizeDisc(d.disc ?? '') ?? undefined
  return undefined
}

/** The version field keeps digits and dots only; a typed "v" goes away. */
export function cleanVersion(raw: string): string {
  return raw.replace(/[^0-9.]/g, '')
}

const kindTags: Record<ItemKind, string> = {
  base: ' [BASE]',
  update: ' [UPDATE v…]',
  dlc: ' [DLC …]',
  game: '',
  disc: ' (Disc …)',
}

/** What the user typed for the draft's kind. */
function draftText(d: KindDraft): string {
  if (d.kind === 'update') return d.version
  if (d.kind === 'disc') return d.disc ?? ''
  return d.dlcName
}

/**
 * A file's final name as the data stands; a missing version or name shows
 * as "…" (PathPreview, docs/design-handoff.md §9).
 */
export function previewFileName(
  title: string,
  d: KindDraft,
  extension: string,
): { name: string; complete: boolean } {
  if (d.kind) {
    const result = itemFileName({ title, kind: d.kind, label: draftText(d) }, extension)
    if (result.ok) return { name: result.name, complete: true }
  }
  const tag = d.kind ? kindTags[d.kind] : ' […]'
  return {
    name: `${sanitizeTitle(title) || '…'}${tag}${extension.toLowerCase()}`,
    complete: false,
  }
}
