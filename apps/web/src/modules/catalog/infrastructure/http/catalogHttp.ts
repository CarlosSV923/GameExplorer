import type { ApiClient } from '@/shared/api/client'
import { unwrap, unwrapVoid } from '@/shared/api/result'

import type { CatalogPorts } from '../../application/ports'

export function createCatalogHttp(client: ApiClient, baseUrl = '/api'): CatalogPorts {
  const id = (n: number) => ({ params: { path: { id: n } } })
  const slugExt = (slug: string, extension: string) => ({
    params: { path: { slug }, query: { extension } },
  })
  return {
    consoles: {
      list: () => unwrap(client.GET('/consoles')),
      rename: (slug, displayName) =>
        unwrap(
          client.PATCH('/consoles/{slug}', { params: { path: { slug } }, body: { displayName } }),
        ),
      reorder: (slugs) => unwrap(client.PUT('/consoles/order', { body: { slugs } })),
      addExtension: (slug, extension) =>
        unwrap(
          client.POST('/consoles/{slug}/extensions', {
            params: { path: { slug } },
            body: { extension },
          }),
        ),
      removeExtension: (slug, extension) =>
        unwrap(client.DELETE('/consoles/{slug}/extensions', slugExt(slug, extension))),
    },
    library: {
      consoleGames: (slug) =>
        unwrap(client.GET('/consoles/{slug}/games', { params: { path: { slug } } })),
      search: (q) => unwrap(client.GET('/games', { params: { query: { q } } })),
      game: (n) => unwrap(client.GET('/games/{id}', id(n))),
      trashGame: (n) => unwrapVoid(client.POST('/games/{id}/trash', id(n))),
      trashItem: (n) => unwrapVoid(client.POST('/items/{id}/trash', id(n))),
      unassignGame: (n) => unwrapVoid(client.POST('/games/{id}/unassign', id(n))),
      unassignItem: (n) => unwrapVoid(client.POST('/items/{id}/unassign', id(n))),
      planEdit: (n, body) => unwrap(client.POST('/games/{id}/edit/plan', { ...id(n), body })),
      edit: (n, body) => unwrap(client.POST('/games/{id}/edit', { ...id(n), body })),
      editItem: (n, body) => unwrap(client.PATCH('/items/{id}', { ...id(n), body })),
      lastScan: async () => (await unwrap(client.GET('/library/scan'))).lastScan ?? null,
      scan: () => unwrap(client.POST('/library/scan')),
      gameDownloadUrl: (n) => `${baseUrl}/games/${String(n)}/download`,
      itemDownloadUrl: (n) => `${baseUrl}/items/${String(n)}/download`,
    },
    unassigned: {
      list: () => unwrap(client.GET('/unassigned')),
      trash: (n) => unwrapVoid(client.POST('/unassigned/{id}/trash', id(n))),
      remove: (n) => unwrapVoid(client.DELETE('/unassigned/{id}', id(n))),
      downloadUrl: (n) => `${baseUrl}/unassigned/${String(n)}/download`,
      trashEntry: (n) => unwrapVoid(client.POST('/unassigned/entries/{id}/trash', id(n))),
      removeEntry: (n) => unwrapVoid(client.DELETE('/unassigned/entries/{id}', id(n))),
      entryDownloadUrl: (n) => `${baseUrl}/unassigned/entries/${String(n)}/download`,
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
