import type { TFunction } from 'i18next'

import { toAppError, type AppErrorKind } from '@/shared/kernel/errors'

/**
 * The text for an error (docs/design-handoff.md §6): our own translation
 * for the cases the screen knows, the server's detail as a fallback, and a
 * generic text per kind last.
 */
export function describeError(
  t: TFunction,
  error: unknown,
  known: Partial<Record<AppErrorKind, string>> = {},
): string {
  const e = toAppError(error)
  return known[e.kind] ?? e.detail ?? t(`errors.${e.kind}`)
}
