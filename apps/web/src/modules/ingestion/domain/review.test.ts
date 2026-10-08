import {
  detect,
  draftError,
  guessTitle,
  initialConsole,
  initialDraft,
  storedCount,
  toCommitItem,
  type ItemDraft,
} from './review'
import type { CommitPlan, StagedItem } from './types'

const staged = (over: Partial<StagedItem>): StagedItem => ({
  path: 'game.nsp',
  shape: 'file',
  parts: ['game.nsp'],
  size: 1,
  ignored: false,
  consoles: ['switch'],
  confidence: 'extension',
  ...over,
})

const draft = (over: Partial<ItemDraft>): ItemDraft => ({
  skip: false,
  kind: 'base',
  version: '',
  dlcName: '',
  disc: '',
  onDuplicate: 'skip',
  ...over,
})

describe('detect', () => {
  it('concludes when every kept item agrees on one console', () => {
    const d = detect([
      staged({ confidence: 'header' }),
      staged({ path: 'u.nsp' }),
      staged({ path: 'readme.txt', ignored: true, consoles: [] }),
    ])
    expect(d).toEqual({ slug: 'switch', candidates: ['switch'], confidence: 'header' })
  })

  it('stays open when items disagree or are ambiguous', () => {
    expect(detect([staged({ consoles: ['gc', 'wii'] })]).slug).toBeNull()
    expect(detect([staged({}), staged({ consoles: ['ps2'] })])).toMatchObject({
      slug: null,
      candidates: ['switch', 'ps2'],
    })
    expect(detect([staged({ consoles: [], confidence: 'none' })])).toEqual({
      slug: null,
      candidates: [],
      confidence: 'none',
    })
  })
})

describe('initialConsole', () => {
  const known = ['switch', 'ps2']
  const detection = { slug: 'switch', candidates: ['switch'], confidence: 'header' as const }

  it('prefers the console screen the upload came from (RF-08)', () => {
    expect(initialConsole({ originConsole: 'ps2' }, detection, known)).toBe('ps2')
  })

  it('falls back to the detection, then to nothing', () => {
    expect(initialConsole({ originConsole: null }, detection, known)).toBe('switch')
    expect(initialConsole({}, { ...detection, slug: null }, known)).toBe('')
    expect(initialConsole({ originConsole: 'gone' }, { ...detection, slug: 'n64' }, known)).toBe('')
  })
})

describe('initialDraft', () => {
  it('uses the suggestions: kind, update version and disc number', () => {
    expect(
      initialDraft(staged({ suggestedKind: 'update', displayVersion: '1.0.3' }), false),
    ).toMatchObject({ kind: 'update', version: 'v1.0.3' })
    expect(initialDraft(staged({ discNumber: 2 }), false)).toMatchObject({
      kind: 'disc',
      disc: '2',
    })
  })

  it('assumes Base for a lone item without a suggestion', () => {
    expect(initialDraft(staged({}), true).kind).toBe('base')
    expect(initialDraft(staged({}), false).kind).toBeUndefined()
  })
})

describe('draftError and toCommitItem', () => {
  it.each([
    [draft({ kind: undefined }), 'kind'],
    [draft({ kind: 'update', version: ' ' }), 'version'],
    [draft({ kind: 'dlc' }), 'dlcName'],
    [draft({ kind: 'disc', disc: '0' }), 'disc'],
    [draft({ kind: 'disc', disc: '2' }), null],
    [draft({ kind: undefined, skip: true }), null],
  ])('%o → %s', (d, want) => {
    expect(draftError(d)).toBe(want)
  })

  it('sends the label that matches the kind', () => {
    expect(
      toCommitItem('u.nsp', draft({ kind: 'update', version: ' v3.0.1 ', dlcName: 'x' })),
    ).toEqual({
      path: 'u.nsp',
      kind: 'update',
      label: 'v3.0.1',
      onDuplicate: 'skip',
    })
    expect(
      toCommitItem('d.cue', draft({ kind: 'disc', disc: '3', onDuplicate: 'replace' })),
    ).toEqual({
      path: 'd.cue',
      kind: 'disc',
      discNumber: 3,
      onDuplicate: 'replace',
    })
    expect(toCommitItem('x', draft({ skip: true }))).toEqual({ path: 'x', skip: true })
  })
})

describe('storedCount', () => {
  it('leaves out skipped items and duplicates kept as they are', () => {
    const plan: CommitPlan = {
      console: 'switch',
      title: 'T',
      folder: 'T',
      existing: [],
      items: [
        { path: 'a', files: ['T.nsp'], action: 'skip' },
        { path: 'b', files: ['T [Update v1].nsp'], action: 'store' },
      ],
    }
    const drafts = { a: draft({}), b: draft({ kind: 'update' }), c: draft({ skip: true }) }
    expect(storedCount(drafts, plan)).toBe(1)
    expect(storedCount(drafts, undefined)).toBe(2)
  })
})

describe('guessTitle', () => {
  it.each([
    ['Okami_HD_[0100A4400B582000][v0].part1.rar', 'Okami HD'],
    ['INSIDE-Switch-NSP-Base-Game.rar', 'INSIDE'],
    ['Spider-Man Miles Morales.iso', 'Spider-Man Miles Morales'],
    ['Metroid Prime (USA).iso', 'Metroid Prime'],
    ['FF7_USA_PSX.7z', 'FF7'],
    ['Mario Kart 8 Deluxe [0100152000022000][v0].nsp', 'Mario Kart 8 Deluxe'],
    ['zelda_ww.7z.001', 'zelda ww'],
  ])('%s → %s', (name, want) => {
    expect(guessTitle(name)).toBe(want)
  })
})
