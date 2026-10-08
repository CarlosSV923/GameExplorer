import type { Services } from '@/app/services'
import type { Console } from '@/modules/catalog/domain/types'
import { AppError } from '@/shared/kernel/errors'

export const consoles: Console[] = [
  {
    id: 1,
    slug: 'switch',
    displayName: 'Nintendo Switch',
    releaseYear: 2017,
    extensions: ['.nsp', '.xci'],
    sortOrder: 10,
    gameCount: 2,
    builtIn: true,
    detection: 'titleId',
  },
  {
    id: 2,
    slug: 'ps2',
    displayName: 'PlayStation 2',
    releaseYear: 2000,
    extensions: ['.iso'],
    sortOrder: 20,
    gameCount: 0,
    builtIn: true,
    detection: 'header',
  },
  {
    id: 3,
    slug: 'gc',
    displayName: 'Nintendo GameCube',
    releaseYear: 2001,
    extensions: ['.iso'],
    sortOrder: 30,
    gameCount: 1,
    builtIn: true,
    detection: 'header',
  },
]

const never = () => new Promise<never>(() => undefined)

/** Ports that answer like a signed-in API with an empty-ish library. */
export function fakeServices(overrides: { signedIn?: boolean } = {}): Services {
  const signedIn = overrides.signedIn ?? true
  return {
    identity: {
      session: vi.fn(() =>
        signedIn
          ? Promise.resolve({ expiresAt: '2030-01-01T00:00:00Z' })
          : Promise.reject(new AppError('unauthorized')),
      ),
      login: vi.fn((password: string) =>
        password === 'gameexplorer'
          ? Promise.resolve()
          : Promise.reject(new AppError('unauthorized', 'Contraseña incorrecta.')),
      ),
      logout: vi.fn(() => Promise.resolve()),
    },
    system: { health: vi.fn(() => Promise.resolve({ status: 'ok' as const, checks: [] })) },
    catalog: {
      consoles: {
        list: vi.fn(() => Promise.resolve(consoles)),
        create: vi.fn(never),
        update: vi.fn(never),
        remove: vi.fn(never),
        reorder: vi.fn(never),
      },
      library: {
        consoleGames: vi.fn(() => Promise.resolve([])),
        search: vi.fn(() => Promise.resolve([])),
        game: vi.fn(never),
        trashGame: vi.fn(never),
        trashItem: vi.fn(never),
        forgetItem: vi.fn(never),
        planRematch: vi.fn(never),
        rematch: vi.fn(never),
        lastCheck: vi.fn(() => Promise.resolve(null)),
        check: vi.fn(never),
        gameDownloadUrl: (id) => `/api/games/${String(id)}/download`,
        itemDownloadUrl: (id) => `/api/items/${String(id)}/download`,
      },
      trash: {
        list: vi.fn(() => Promise.resolve([])),
        restore: vi.fn(never),
        remove: vi.fn(never),
        empty: vi.fn(never),
      },
    },
    metadata: {
      searchGames: vi.fn(() => Promise.resolve([])),
      searchPlatforms: vi.fn(() => Promise.resolve([])),
      imageUrl: (size, id) => `/api/images/${size}/${id}`,
    },
    ingestion: {
      upload: vi.fn(() => ({ abort: vi.fn() })),
      jobs: vi.fn(() => Promise.resolve([])),
      job: vi.fn(never),
      watchJobs: vi.fn(() => () => undefined),
      items: vi.fn(never),
      submitPassword: vi.fn(never),
      cancel: vi.fn(never),
      plan: vi.fn(never),
      commit: vi.fn(never),
    },
  }
}
