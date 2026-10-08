import type { ApiClient } from '@/shared/api/client'
import { unwrap, unwrapVoid } from '@/shared/api/result'

import type { CatalogPorts } from '../../application/ports'

export function createCatalogHttp(client: ApiClient, baseUrl = '/api'): CatalogPorts {
  const id = (n: number) => ({ params: { path: { id: n } } })
  return {
    consoles: {
      list: () => unwrap(client.GET('/consoles')),
      create: (body) => unwrap(client.POST('/consoles', { body })),
      update: (n, body) => unwrap(client.PATCH('/consoles/{id}', { ...id(n), body })),
      remove: (n) => unwrapVoid(client.DELETE('/consoles/{id}', id(n))),
      reorder: (ids) => unwrap(client.PUT('/consoles/order', { body: { ids } })),
    },
    library: {
      consoleGames: (slug) =>
        unwrap(client.GET('/consoles/{slug}/games', { params: { path: { slug } } })),
      search: (q) => unwrap(client.GET('/games', { params: { query: { q } } })),
      game: (n) => unwrap(client.GET('/games/{id}', id(n))),
      trashGame: (n) => unwrapVoid(client.POST('/games/{id}/trash', id(n))),
      trashItem: (n) => unwrapVoid(client.POST('/items/{id}/trash', id(n))),
      forgetItem: (n) => unwrapVoid(client.POST('/items/{id}/forget', id(n))),
      planRematch: (n, body) => unwrap(client.POST('/games/{id}/rematch/plan', { ...id(n), body })),
      rematch: (n, body) => unwrap(client.POST('/games/{id}/rematch', { ...id(n), body })),
      lastCheck: async () => (await unwrap(client.GET('/library/check'))).lastCheck ?? null,
      check: () => unwrap(client.POST('/library/check')),
      gameDownloadUrl: (n) => `${baseUrl}/games/${String(n)}/download`,
      itemDownloadUrl: (n) => `${baseUrl}/items/${String(n)}/download`,
    },
    trash: {
      list: () => unwrap(client.GET('/trash')),
      restore: (n, replace) =>
        unwrap(
          client.POST('/trash/{id}/restore', {
            ...id(n),
            body: replace ? { onConflict: 'replace' } : {},
          }),
        ),
      remove: (n) => unwrapVoid(client.DELETE('/trash/{id}', id(n))),
      empty: () => unwrapVoid(client.DELETE('/trash')),
    },
  }
}
