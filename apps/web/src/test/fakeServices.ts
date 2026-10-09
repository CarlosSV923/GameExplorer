import type { Services } from '@/app/services'
import type { Console } from '@/modules/catalog/domain/types'
import { AppError } from '@/shared/kernel/errors'

/** The three consoles of docs/spec.md §6; Switch has a custom extension. */
export const consoles: Console[] = [
  {
    slug: 'switch',
    displayName: 'Nintendo Switch',
    defaultName: 'Nintendo Switch',
    igdbPlatformId: 130,
    releaseYear: 2017,
    extensions: ['.nsp', '.xci'],
    customExtensions: [{ extension: '.xcz', fileCount: 0 }],
    kinds: ['base', 'update', 'dlc'],
    multipleFiles: true,
    sortOrder: 0,
    gameCount: 2,
  },
  {
    slug: 'wii',
    displayName: 'Wii',
    defaultName: 'Wii',
    igdbPlatformId: 5,
    releaseYear: 2006,
    extensions: ['.iso', '.wbfs', '.rvz', '.nkit.iso'],
    customExtensions: [],
    kinds: ['game'],
    multipleFiles: false,
    sortOrder: 1,
    gameCount: 1,
  },
  {
    slug: 'psp',
    displayName: 'PlayStation Portable',
    defaultName: 'PlayStation Portable',
    igdbPlatformId: 38,
    releaseYear: 2004,
    extensions: ['.iso', '.cso'],
    customExtensions: [],
    kinds: ['game'],
    multipleFiles: false,
    sortOrder: 2,
    gameCount: 0,
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
        rename: vi.fn(never),
        reorder: vi.fn(never),
        addExtension: vi.fn(never),
        removeExtension: vi.fn(never),
      },
      library: {
        consoleGames: vi.fn(() => Promise.resolve([])),
        search: vi.fn(() => Promise.resolve([])),
        game: vi.fn(never),
        trashGame: vi.fn(never),
        trashItem: vi.fn(never),
        unassignGame: vi.fn(never),
        unassignItem: vi.fn(never),
        planEdit: vi.fn(never),
        edit: vi.fn(never),
        editItem: vi.fn(never),
        lastScan: vi.fn(() => Promise.resolve(null)),
        scan: vi.fn(never),
        gameDownloadUrl: (id) => `/api/games/${String(id)}/download`,
        itemDownloadUrl: (id) => `/api/items/${String(id)}/download`,
      },
      unassigned: {
        list: vi.fn(() => Promise.resolve([])),
        trash: vi.fn(never),
        remove: vi.fn(never),
        downloadUrl: (id) => `/api/unassigned/${String(id)}/download`,
        trashEntry: vi.fn(never),
        removeEntry: vi.fn(never),
        entryDownloadUrl: (id) => `/api/unassigned/entries/${String(id)}/download`,
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
      status: vi.fn(() => Promise.resolve({ configured: true })),
      imageUrl: (size, id) => `/api/images/${size}/${id}`,
    },
    ingestion: {
      upload: vi.fn(() => ({ abort: vi.fn() })),
      assign: vi.fn(never),
      jobs: vi.fn(() => Promise.resolve([])),
      job: vi.fn(never),
      watchJobs: vi.fn(() => () => undefined),
      files: vi.fn(never),
      submitPassword: vi.fn(never),
      changeConsole: vi.fn(never),
      resolve: vi.fn(never),
      cancel: vi.fn(never),
      plan: vi.fn(never),
      commit: vi.fn(never),
    },
  }
}
