import { AppError, type AppErrorKind } from '@/shared/kernel/errors'

const kinds: Record<number, AppErrorKind> = {
  400: 'invalid',
  401: 'unauthorized',
  404: 'notFound',
  409: 'conflict',
  429: 'rateLimited',
  502: 'igdbUnavailable',
  503: 'igdbUnconfigured',
}

/**
 * Maps an HTTP status and its RFC 9457 problem to an AppError. A server
 * error without a problem body did not come from the API (a proxy, or the
 * NAS restarting): it counts as a network error.
 */
export function problemError(status: number, problem: unknown): AppError {
  const isProblem = typeof problem === 'object' && problem !== null
  const detail = isProblem && 'detail' in problem ? String(problem.detail) : undefined
  if (status >= 500 && !isProblem) return new AppError('network', detail)
  return new AppError(kinds[status] ?? 'failed', detail)
}

interface Result<T> {
  data?: T
  error?: unknown
  response: Response
}

/**
 * Awaits an openapi-fetch call: its data, or an AppError (network failures
 * included). Only the HTTP adapters use it.
 */
export async function unwrap<T>(request: Promise<Result<T>>): Promise<T> {
  let result: Result<T>
  try {
    result = await request
  } catch (error) {
    throw new AppError('network', error instanceof Error ? error.message : undefined)
  }
  if (result.error !== undefined || !result.response.ok) {
    throw problemError(result.response.status, result.error)
  }
  return result.data as T
}

/** Like unwrap, for operations that answer 204 No Content. */
export async function unwrapVoid(request: Promise<Result<unknown>>): Promise<void> {
  await unwrap(request)
}
