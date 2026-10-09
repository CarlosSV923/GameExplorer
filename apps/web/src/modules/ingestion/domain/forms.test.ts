import { consoles } from '@/test/fakeServices'

import { cleanVersion, kindDraftError, previewFileName } from '@/modules/catalog/domain/naming'

import { initialDraft, toCommitFile } from './details'
import { checkFile, defaultMode, specsFor } from './uploadForm'

const [switchConsole, wii] = consoles

describe('upload form', () => {
  it('lets archives through and checks other files against the console', () => {
    expect(checkFile({ name: 'Limbo.rar', size: 1 }, switchConsole, consoles)).toBe('archive')
    expect(checkFile({ name: 'Limbo.iso', size: 1 }, switchConsole, consoles)).toBe('invalid')
    expect(checkFile({ name: 'Limbo.xcz', size: 1 }, switchConsole, consoles)).toBe('ok')
    expect(checkFile({ name: 'Okami.nkit.iso', size: 1 }, wii, consoles)).toBe('ok')
    expect(checkFile({ name: 'Okami.bin', size: 1, archive: true }, wii, consoles)).toBe('archive')
    expect(checkFile({ name: 'Okami.iso', size: 1 }, undefined, consoles)).toBe('ok')
  })

  it('starts as parts of one archive when the names say so', () => {
    expect(defaultMode(['a.part1.rar', 'a.part2.rar'])).toBe('parts')
    expect(defaultMode(['a.7z.001', 'a.7z.002'])).toBe('parts')
    expect(defaultMode(['a.nsp', 'b.nsp'])).toBe('same')
  })

  it('gives the parts of an archive one group', () => {
    const spec = { console: 'switch', title: 'Limbo' }
    expect(specsFor(2, 'parts', spec, () => 'g1')).toEqual([
      { ...spec, group: 'g1', groupSize: 2 },
      { ...spec, group: 'g1', groupSize: 2 },
    ])
    expect(specsFor(2, 'same', spec, () => 'g1')).toEqual([spec, spec])
  })
})

describe('file details', () => {
  it('needs nothing on a one-file console and a kind on Switch', () => {
    expect(initialDraft({ kinds: ['game'] }, 1).kind).toBe('game')
    expect(initialDraft({ kinds: ['base', 'update', 'dlc'] }, 1).kind).toBe('base')
    expect(initialDraft({ kinds: ['base', 'update', 'dlc'] }, 3).kind).toBeUndefined()
  })

  it('asks for the version of updates and the name of DLC', () => {
    const d = { kind: 'update' as const, version: '', dlcName: '' }
    expect(kindDraftError(d)).toBe('version')
    expect(kindDraftError({ ...d, version: '1.0.4' })).toBeNull()
    expect(kindDraftError({ ...d, kind: 'dlc', dlcName: ' ? ' })).toBe('dlcName')
    expect(cleanVersion('v1.0.4b')).toBe('1.0.4')
  })

  it('previews the final name, with a gap for missing data', () => {
    const d = { kind: 'update' as const, version: '', dlcName: '' }
    expect(previewFileName('Limbo', d, '.NSP')).toEqual({
      name: 'Limbo [UPDATE v…].nsp',
      complete: false,
    })
    expect(previewFileName('Limbo', { ...d, version: '1.0.4' }, '.nsp')).toEqual({
      name: 'Limbo [UPDATE v1.0.4].nsp',
      complete: true,
    })
    expect(previewFileName('Ōkami', { ...d, kind: 'game' }, '.nkit.iso').name).toBe(
      'Ōkami.nkit.iso',
    )
  })

  it('sends the clean label', () => {
    expect(
      toCommitFile('a.nsp', {
        kind: 'dlc',
        version: '',
        dlcName: ' Fuga   Maestra ',
        onDuplicate: 'replace',
      }),
    ).toEqual({ path: 'a.nsp', kind: 'dlc', label: 'Fuga Maestra', onDuplicate: 'replace' })
  })
})
