import { createApiClient } from './client'

describe('createApiClient', () => {
  it('calls the same-origin /api with the session cookie and typed responses', async () => {
    const fetchMock = vi.fn<typeof fetch>(() =>
      Promise.resolve(
        new Response(
          JSON.stringify([
            {
              id: 1,
              slug: 'ps2',
              displayName: 'PlayStation 2',
              extensions: ['.iso'],
              sortOrder: 50,
            },
          ]),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      ),
    )
    const client = createApiClient({ baseUrl: 'http://localhost/api', fetch: fetchMock })

    const { data, error } = await client.GET('/consoles')

    expect(error).toBeUndefined()
    expect(data?.[0]?.displayName).toBe('PlayStation 2')
    const request = fetchMock.mock.calls[0]?.[0] as Request
    expect(request.url).toBe('http://localhost/api/consoles')
    expect(request.credentials).toBe('same-origin')
  })

  it('exposes RFC 9457 problems as typed errors', async () => {
    const fetchMock = vi.fn<typeof fetch>(() =>
      Promise.resolve(
        new Response(
          JSON.stringify({ title: 'Unauthorized', status: 401, detail: 'Contraseña incorrecta.' }),
          {
            status: 401,
            headers: { 'Content-Type': 'application/problem+json' },
          },
        ),
      ),
    )
    const client = createApiClient({ baseUrl: 'http://localhost/api', fetch: fetchMock })

    const { error, response } = await client.POST('/auth/login', { body: { password: 'nope' } })

    expect(response.status).toBe(401)
    expect(error?.detail).toBe('Contraseña incorrecta.')
  })
})
