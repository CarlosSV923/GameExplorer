import { kindDraftLabel, type KindDraft } from '@/modules/catalog/domain/naming'
import type { Console, DuplicateAction } from '@/modules/catalog/domain/types'

import type { CommitFile, CommitPlan } from './types'

/** The user's data for one valid file before storing it (RF-08, RF-09). */
export interface FileDraft extends KindDraft {
  /** What to do if its final name is taken (RF-09). */
  onDuplicate: DuplicateAction
  /** "No guardar": it stays in the unassigned section (assigned entries only, RF-27a). */
  skip?: boolean
}

/**
 * The starting data: consoles with one file per game need nothing; on
 * Switch a lone file starts as the base, several start empty.
 */
export function initialDraft(console: Pick<Console, 'kinds'>, validCount: number): FileDraft {
  const only = console.kinds.length === 1 ? console.kinds[0] : undefined
  return {
    kind: only ?? (validCount === 1 && console.kinds.includes('base') ? 'base' : undefined),
    version: '',
    dlcName: '',
    onDuplicate: 'skip',
  }
}

/**
 * Which valid files of an assigned entry start as "No guardar": on a
 * one-file console only the first one is kept (RF-27a).
 */
export function initialSkips(
  console: Pick<Console, 'kinds'>,
  paths: readonly string[],
): ReadonlySet<string> {
  return console.kinds.length === 1 ? new Set(paths.slice(1)) : new Set()
}

/** The commit (or plan) entry for a complete file. */
export function toCommitFile(path: string, d: FileDraft): CommitFile {
  if (d.skip) return { path, skip: true }
  const label = kindDraftLabel(d)
  return {
    path,
    ...(d.kind ? { kind: d.kind } : {}),
    ...(label ? { label } : {}),
    onDuplicate: d.onDuplicate,
  }
}

/** How many files the commit stores, after the duplicate decisions. */
export function storedCount(paths: readonly string[], plan: CommitPlan | undefined): number {
  return paths.filter((path) => plan?.files.find((p) => p.path === path)?.action !== 'skip').length
}

/** The disc number a file's name says ("RE4 (Disc 2).iso", "disc2"), if any. */
export function discFromName(name: string): number | undefined {
  const m = /dis[ck]\s*[-_]?\s*(\d{1,2})(?!\d)/i.exec(name)
  const n = m ? Number(m[1]) : NaN
  return n >= 1 && n <= 99 ? n : undefined
}

/**
 * The disc numbers a form starts with (RF-08): the one each name says, then
 * 1, 2, 3… by name for the rest; numbers already taken are skipped.
 */
export function suggestDiscs(
  paths: readonly string[],
  taken: Iterable<number> = [],
): Record<string, string> {
  const used = new Set(taken)
  const out: Record<string, string> = {}
  const sorted = [...paths].sort((a, b) => a.localeCompare(b, undefined, { numeric: true }))
  for (const p of sorted) {
    const n = discFromName(p.slice(p.lastIndexOf('/') + 1))
    if (n !== undefined && !used.has(n)) {
      out[p] = String(n)
      used.add(n)
    }
  }
  let next = 1
  for (const p of sorted) {
    if (out[p] !== undefined) continue
    while (used.has(next)) next++
    out[p] = String(next)
    used.add(next)
  }
  return out
}
