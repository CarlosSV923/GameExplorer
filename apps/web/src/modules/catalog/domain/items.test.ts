import { groupByConsole, itemChip, normalizeExtension, suggestSlug } from './items'

describe('itemChip', () => {
  it('labels updates and numbered discs', () => {
    expect(itemChip({ kind: 'update', label: 'v3.0.1' })).toEqual({
      kind: 'update',
      key: 'kind.update',
      label: 'v3.0.1',
    })
    expect(itemChip({ kind: 'disc', discNumber: 2 })).toEqual({
      kind: 'disc',
      key: 'kind.discNumber',
      number: 2,
    })
    expect(itemChip({ kind: 'dlc', label: 'Pass' })).toEqual({ kind: 'dlc', key: 'kind.dlc' })
  })
})

describe('groupByConsole', () => {
  it('keeps the carousel order and puts unknown consoles last', () => {
    const games = [
      { console: 'ps2', title: 'A' },
      { console: 'n64', title: 'B' },
      { console: 'switch', title: 'C' },
      { console: 'ps2', title: 'D' },
    ]
    expect(groupByConsole(games, ['switch', 'ps2'])).toEqual([
      { console: 'switch', games: [games[2]] },
      { console: 'ps2', games: [games[0], games[3]] },
      { console: 'n64', games: [games[1]] },
    ])
  })
})

describe('console fields', () => {
  it.each([
    ['z64', '.z64'],
    ['.Z64', '.z64'],
    ['  ..wbfs ', '.wbfs'],
    ['bad ext', null],
    ['', null],
  ])('normalizes extension %j', (raw, want) => {
    expect(normalizeExtension(raw)).toBe(want)
  })

  it('suggests a folder from the platform name', () => {
    expect(suggestSlug('Wii U')).toBe('wiiu')
    expect(suggestSlug('Nintendo 64')).toBe('nintendo64')
    expect(suggestSlug('Pokémon mini')).toBe('pokemonmini')
  })
})
