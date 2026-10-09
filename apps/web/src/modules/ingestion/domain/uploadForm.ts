import { consoleExtensions, fileExtension, looksLikeArchive } from '@/modules/catalog/domain/items'
import type { Console } from '@/modules/catalog/domain/types'

import type { UploadSpec } from './types'

/** How several files are treated (RF-03a). */
export type UploadMode = 'parts' | 'same' | 'separate'

/** A file in the form: picked from disk, or an unassigned file being assigned. */
export interface FormFile {
  name: string
  size: number
  /** Known for unassigned files (the server checked); guessed from the name otherwise. */
  archive?: boolean
}

/**
 * The pre-check of RF-03: archives are validated once extracted; any other
 * file must have an extension of the chosen console, or uploading it is
 * pointless.
 */
export type FileCheck = 'ok' | 'archive' | 'invalid'

export function checkFile(
  file: FormFile,
  target: Console | undefined,
  consoles: readonly Console[],
): FileCheck {
  if (file.archive ?? looksLikeArchive(file.name)) return 'archive'
  if (!target) return 'ok'
  const ext = fileExtension(file.name, consoles)
  return ext && consoleExtensions(target).includes(ext) ? 'ok' : 'invalid'
}

/** game.part1.rar, game.7z.001…: the parts of one multi-volume archive. */
export function looksLikeVolumes(names: readonly string[]): boolean {
  return names.length > 1 && names.every((n) => /\.part\d+\.rar$|\.(7z|zip|rar)\.\d{1,3}$/i.test(n))
}

/** The mode a set of files starts with. */
export function defaultMode(names: readonly string[]): UploadMode {
  return looksLikeVolumes(names) ? 'parts' : 'same'
}

/**
 * One spec per file. The parts of an archive share a group so the server
 * waits for all of them and extracts from the first volume.
 */
export function specsFor(
  count: number,
  mode: Exclude<UploadMode, 'separate'>,
  spec: UploadSpec,
  newGroup: () => string,
): UploadSpec[] {
  if (mode === 'same' || count < 2) return Array.from({ length: count }, () => spec)
  const group = newGroup()
  return Array.from({ length: count }, () => ({ ...spec, group, groupSize: count }))
}
