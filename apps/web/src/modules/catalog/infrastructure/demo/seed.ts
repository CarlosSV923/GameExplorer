import type { ItemKind } from '../../domain/types'
import { DemoLibrary, type DemoConsoleDef, type DemoGameInfo } from './libraryDemo'

const GB = 1_000_000_000
const MB = 1_000_000

/** The consoles defined in code (spec §6), as the server has them. */
export const demoConsoles: Omit<DemoConsoleDef, 'logoImageId'>[] = [
  {
    slug: 'n64',
    name: 'Nintendo 64',
    igdbPlatformId: 4,
    releaseYear: 1996,
    extensions: ['.z64', '.n64', '.v64'],
    kinds: ['game'],
    multipleFiles: false,
  },
  {
    slug: 'gc',
    name: 'Nintendo GameCube',
    igdbPlatformId: 21,
    releaseYear: 2001,
    extensions: ['.iso', '.rvz'],
    kinds: ['game', 'disc'],
    multipleFiles: true,
  },
  {
    slug: 'wii',
    name: 'Wii',
    igdbPlatformId: 5,
    releaseYear: 2006,
    extensions: ['.iso', '.wbfs', '.rvz', '.nkit.iso'],
    kinds: ['game'],
    multipleFiles: false,
  },
  {
    slug: 'switch',
    name: 'Nintendo Switch',
    igdbPlatformId: 130,
    releaseYear: 2017,
    extensions: ['.nsp', '.xci'],
    kinds: ['base', 'update', 'dlc'],
    multipleFiles: true,
  },
  {
    slug: 'ps2',
    name: 'PlayStation 2',
    igdbPlatformId: 8,
    releaseYear: 2000,
    extensions: ['.iso'],
    kinds: ['game', 'disc'],
    multipleFiles: true,
  },
  {
    slug: 'psp',
    name: 'PlayStation Portable',
    igdbPlatformId: 38,
    releaseYear: 2004,
    extensions: ['.iso', '.cso'],
    kinds: ['game'],
    multipleFiles: false,
  },
]

type Seed = [igdbId: number, files: { kind: ItemKind; label?: string; ext: string; size: number }[]]

const one = (ext: string, size: number) => [{ kind: 'game' as const, ext, size }]

/** The library the demo opens with (RF-60), from the fixed catalog's games. */
const seeds: Record<string, Seed[]> = {
  n64: [
    [1074, one('.z64', 8 * MB)],
    [1029, one('.z64', 32 * MB)],
    [1638, one('.n64', 12 * MB)],
    [2591, one('.v64', 12 * MB)],
  ],
  gc: [
    // A game of two discs (spec §5).
    [
      974,
      [
        { kind: 'disc', label: '1', ext: '.iso', size: 1.4 * GB },
        { kind: 'disc', label: '2', ext: '.iso', size: 1.4 * GB },
      ],
    ],
    [1105, one('.rvz', 0.9 * GB)],
    [1627, one('.iso', 1.4 * GB)],
    [1033, one('.rvz', 0.8 * GB)],
  ],
  ps2: [
    [2207, one('.iso', 2.7 * GB)],
    [379, one('.iso', 4.1 * GB)],
    [418, one('.iso', 3.6 * GB)],
    [732, one('.iso', 4.4 * GB)],
  ],
  switch: [
    [
      26764,
      [
        { kind: 'base', ext: '.nsp', size: 7.1 * GB },
        { kind: 'update', label: '3.0.3', ext: '.nsp', size: 1.6 * GB },
        { kind: 'dlc', label: 'Booster Course Pass', ext: '.nsp', size: 2.3 * GB },
      ],
    ],
    // Only its base: the Switch sample brings it again (a duplicate) with an update and a DLC.
    [109462, [{ kind: 'base', ext: '.nsp', size: 6.7 * GB }]],
    [
      7346,
      [
        { kind: 'base', ext: '.xci', size: 13.5 * GB },
        { kind: 'update', label: '1.6.0', ext: '.nsp', size: 1.1 * GB },
      ],
    ],
    [26758, [{ kind: 'base', ext: '.xci', size: 5.6 * GB }]],
    [14593, [{ kind: 'base', ext: '.nsp', size: 6.8 * GB }]],
    [113112, [{ kind: 'base', ext: '.nsp', size: 6.4 * GB }]],
    [
      17000,
      [
        { kind: 'base', ext: '.nsp', size: 0.9 * GB },
        { kind: 'update', label: '1.6.9', ext: '.nsp', size: 0.4 * GB },
      ],
    ],
  ],
  wii: [
    [1077, one('.wbfs', 4.3 * GB)],
    [1078, one('.wbfs', 4.1 * GB)],
    [1103, one('.iso', 0.9 * GB)],
    [885, one('.rvz', 0.6 * GB)],
  ],
  psp: [
    [1533, one('.cso', 0.9 * GB)],
    [375, one('.iso', 0.7 * GB)],
    [427, one('.cso', 1.4 * GB)],
    [1192, one('.iso', 0.5 * GB)],
    [480, one('.cso', 0.6 * GB)],
  ],
}

/** A fresh demo library: the seeded games and two unassigned entries. */
export function seededLibrary(
  logos: ReadonlyMap<number, string>,
  gameInfo: (id: number) => DemoGameInfo | undefined,
): DemoLibrary {
  const library = new DemoLibrary(
    demoConsoles.map((c) => ({ ...c, logoImageId: logos.get(c.igdbPlatformId) ?? null })),
    gameInfo,
  )
  for (const [console, games] of Object.entries(seeds)) {
    for (const [igdbId, files] of games) library.seedGame(console, igdbId, files)
  }
  library.seedLoose('Inside - Edicion Mod/Inside [UPDATE v2.0].nsp', 1.5 * GB, 'samba', 'switch')
  library.seedLoose('Inside - Edicion Mod/notas.txt', 4_000, 'samba', 'switch')
  library.seedLoose('Mario.z64', 8 * MB, 'samba')
  return library
}
