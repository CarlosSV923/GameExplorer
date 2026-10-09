import { consoles } from '@/test/fakeServices'

import { acceptingConsoles, groupByConsole, itemChip, looksLikeArchive, moveTargets } from './items'

describe('itemChip', () => {
  it('shows the version of updates only', () => {
    expect(itemChip({ kind: 'update', label: '1.0.4' })).toEqual({
      kind: 'update',
      key: 'kind.update',
      label: 'v1.0.4',
    })
    expect(itemChip({ kind: 'dlc', label: 'Pass' })).toEqual({ kind: 'dlc', key: 'kind.dlc' })
  })
})

describe('groupByConsole', () => {
  it('keeps the carousel order and puts unknown consoles last', () => {
    const games = [
      { console: 'psp', title: 'A' },
      { console: 'n64', title: 'B' },
      { console: 'switch', title: 'C' },
      { console: 'psp', title: 'D' },
    ]
    expect(groupByConsole(games, ['switch', 'psp'])).toEqual([
      { console: 'switch', games: [games[2]] },
      { console: 'psp', games: [games[0], games[3]] },
      { console: 'n64', games: [games[1]] },
    ])
  })
})

describe('extensions', () => {
  it('picks the longest known extension, custom ones included', () => {
    expect(acceptingConsoles('Ookami.nkit.iso', consoles).map((c) => c.slug)).toEqual(['wii'])
    expect(acceptingConsoles('Daxter.ISO', consoles).map((c) => c.slug)).toEqual(['wii', 'psp'])
    expect(acceptingConsoles('Limbo.xcz', consoles).map((c) => c.slug)).toEqual(['switch'])
    expect(acceptingConsoles('Mario.z64', consoles)).toEqual([])
  })

  it('guesses archives and their volumes from the name', () => {
    expect(looksLikeArchive('a.part1.rar')).toBe(true)
    expect(looksLikeArchive('a.7z.001')).toBe(true)
    expect(looksLikeArchive('a.iso')).toBe(false)
  })
})

describe('moveTargets', () => {
  it('allows only consoles that take every file and kind', () => {
    const iso = [{ file: 'Daxter.iso', kind: 'game' as const }]
    expect(
      moveTargets(iso, 'psp', consoles).map((t) => [t.console.slug, t.blockedBy ?? null]),
    ).toEqual([
      ['switch', '.iso'],
      ['wii', null],
    ])
    const nsp = [{ file: 'Limbo [BASE].nsp', kind: 'base' as const }]
    expect(moveTargets(nsp, 'switch', consoles).every((t) => t.blockedBy === '.nsp')).toBe(true)
  })
})
