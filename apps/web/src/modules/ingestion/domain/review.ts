import type { DuplicateAction, ItemKind } from '@/modules/catalog/domain/types'

import type { CommitItem, CommitPlan, StagedItem, UploadJob } from './types'

/** The user's choices for one staged item in the review (RF-09). */
export interface ItemDraft {
  /** Leave it out of the library. */
  skip: boolean
  kind: ItemKind | undefined
  version: string
  dlcName: string
  disc: string
  /** What to do if it turns out to be a duplicate (RF-10). */
  onDuplicate: DuplicateAction
}

export interface Detection {
  /** The console when every item agrees on exactly one; otherwise null. */
  slug: string | null
  /** Every candidate seen, for an "it may be A or B" hint. */
  candidates: string[]
  confidence: StagedItem['confidence']
}

/** What the detectors concluded for the items of an upload (RF-07). */
export function detect(items: readonly StagedItem[]): Detection {
  const kept = items.filter((i) => !i.ignored)
  const candidates = [...new Set(kept.flatMap((i) => i.consoles))]
  const first = kept[0]?.consoles
  const unanimous =
    first?.length === 1 && kept.every((i) => i.consoles.length === 1 && i.consoles[0] === first[0])
  const confidence = kept.some((i) => i.confidence === 'header')
    ? 'header'
    : kept.some((i) => i.confidence === 'extension')
      ? 'extension'
      : 'none'
  return { slug: unanimous ? (first[0] ?? null) : null, candidates, confidence }
}

/**
 * The console the review starts with (RF-08): the console screen the upload
 * came from; otherwise the detected one; otherwise none (the field is
 * required).
 */
export function initialConsole(
  job: Pick<UploadJob, 'originConsole'>,
  detection: Detection,
  known: readonly string[],
): string {
  if (job.originConsole && known.includes(job.originConsole)) return job.originConsole
  if (detection.slug && known.includes(detection.slug)) return detection.slug
  return ''
}

/** The starting choices for an item, from the detectors' suggestions. */
export function initialDraft(item: StagedItem, only: boolean): ItemDraft {
  let kind = item.suggestedKind ?? undefined
  if (!kind && item.discNumber) kind = 'disc'
  if (!kind && only) kind = 'base'
  return {
    skip: false,
    kind,
    version: item.displayVersion ? `v${item.displayVersion}` : '',
    dlcName: '',
    disc: item.discNumber ? String(item.discNumber) : '',
    onDuplicate: 'skip',
  }
}

export type DraftError = 'kind' | 'version' | 'dlcName' | 'disc'

/** What is missing for an item to be stored. */
export function draftError(d: ItemDraft): DraftError | null {
  if (d.skip) return null
  switch (d.kind) {
    case undefined:
      return 'kind'
    case 'update':
      return d.version.trim() ? null : 'version'
    case 'dlc':
      return d.dlcName.trim() ? null : 'dlcName'
    case 'disc': {
      const n = Number(d.disc)
      return Number.isInteger(n) && n >= 1 && n <= 99 ? null : 'disc'
    }
    default:
      return null
  }
}

/** The commit (or plan) entry for an item. */
export function toCommitItem(path: string, d: ItemDraft): CommitItem {
  if (d.skip || !d.kind) return { path, skip: true }
  const item: CommitItem = { path, kind: d.kind, onDuplicate: d.onDuplicate }
  if (d.kind === 'update') item.label = d.version.trim()
  if (d.kind === 'dlc') item.label = d.dlcName.trim()
  if (d.kind === 'disc') item.discNumber = Number(d.disc)
  return item
}

/** How many items the commit would store, after the duplicate decisions. */
export function storedCount(
  drafts: Readonly<Record<string, ItemDraft>>,
  plan: CommitPlan | undefined,
): number {
  return Object.entries(drafts).filter(([path, d]) => {
    if (d.skip) return false
    const planned = plan?.items.find((p) => p.path === path)
    return planned?.action !== 'skip'
  }).length
}

const noise = new Set([
  'nsp',
  'xci',
  'nsz',
  'xcz',
  'base',
  'game',
  'update',
  'dlc',
  'switch',
  'nintendo',
  'psx',
  'ps1',
  'ps2',
  'ps3',
  'gc',
  'ngc',
  'wii',
  'usa',
  'eur',
  'europe',
  'jpn',
  'japan',
  'pal',
  'ntsc',
  'multi',
  'rip',
])

/**
 * A first IGDB query from an upload's file name:
 * "Okami_HD_[0100…][v0].part1.rar" → "Okami HD".
 */
export function guessTitle(fileName: string): string {
  const stem = fileName
    .replace(/(\.part\d+)?\.(rar|zip|7z)(\.\d+)?$/i, '')
    .replace(/\.(nsp|xci|nsz|xcz|iso|gcm|ciso|rvz|wbfs|cue|bin|chd|pbp|pkg)$/i, '')
  // "INSIDE-Switch-NSP-Base-Game": without spaces, dashes separate words.
  const spaced = /[\s_]/.test(stem) ? stem : stem.replace(/-/g, ' ')
  const words = spaced
    .replace(/\[[^\]]*\]|\([^)]*\)|\{[^}]*\}/g, ' ')
    .replace(/[_.]+/g, ' ')
    .split(/\s+/)
    .filter((w) => w && !noise.has(w.toLowerCase()) && !/^v\d+$/i.test(w))
  return words
    .join(' ')
    .replace(/\s*-\s*$/, '')
    .trim()
}
