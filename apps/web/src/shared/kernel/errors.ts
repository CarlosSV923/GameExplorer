/**
 * Errors every adapter (HTTP or demo) reports in the same terms, so the UI
 * can show its own translated text for each case (docs/design-handoff.md §6)
 * and keep the server's `detail` only as a fallback.
 */
export type AppErrorKind =
  | 'unauthorized' // no session, or a wrong password
  | 'rateLimited' // too many login attempts
  | 'invalid' // the request was rejected (400)
  | 'notFound'
  | 'conflict' // the state does not allow it (409)
  | 'igdbUnconfigured' // IGDB credentials missing (503)
  | 'igdbUnavailable' // IGDB failed or did not answer (502)
  | 'failed' // the operation failed on the server (500)
  | 'network' // the request did not reach the server

export class AppError extends Error {
  readonly kind: AppErrorKind
  readonly detail: string | undefined

  constructor(kind: AppErrorKind, detail?: string) {
    super(detail ?? kind)
    this.name = 'AppError'
    this.kind = kind
    this.detail = detail
  }
}

export function isAppError(error: unknown, kind?: AppErrorKind): error is AppError {
  return error instanceof AppError && (kind === undefined || error.kind === kind)
}

/** Wraps anything thrown into an AppError (unknown errors count as network). */
export function toAppError(error: unknown): AppError {
  if (error instanceof AppError) return error
  return new AppError('network', error instanceof Error ? error.message : undefined)
}
