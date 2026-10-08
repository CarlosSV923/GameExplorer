import { AppError } from '@/shared/kernel/errors'

import { unwrap } from './result'

const response = (status: number) => new Response(null, { status })

describe('unwrap', () => {
  it('returns the data of a successful call', async () => {
    await expect(
      unwrap(Promise.resolve({ data: [1, 2], response: response(200) })),
    ).resolves.toEqual([1, 2])
  })

  it.each([
    [400, 'invalid'],
    [401, 'unauthorized'],
    [404, 'notFound'],
    [409, 'conflict'],
    [429, 'rateLimited'],
    [502, 'igdbUnavailable'],
    [503, 'igdbUnconfigured'],
    [500, 'failed'],
  ])('maps status %i to %s with the detail', async (status, kind) => {
    const call = unwrap(
      Promise.resolve({
        error: { title: 'x', status, detail: 'Motivo' },
        response: response(status),
      }),
    )
    await expect(call).rejects.toMatchObject({ kind, detail: 'Motivo' })
  })

  it('treats a server error without a problem body as a network error', async () => {
    const call = unwrap(Promise.resolve({ error: 'Bad Gateway', response: response(502) }))
    await expect(call).rejects.toMatchObject({ kind: 'network' })
  })

  it('reports failed requests as network errors', async () => {
    const call = unwrap(Promise.reject(new TypeError('Failed to fetch')))
    await expect(call).rejects.toBeInstanceOf(AppError)
    await expect(call).rejects.toMatchObject({ kind: 'network' })
  })
})
