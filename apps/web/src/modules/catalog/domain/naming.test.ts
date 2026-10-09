import vectors from '../../../../../../contracts/naming-cases.json'

import {
  extensionOf,
  gameFolder,
  itemFileName,
  normalizeExtension,
  normalizeVersion,
  sanitizeTitle,
} from './naming'
import type { ItemKind } from './types'

interface NamingCase {
  name: string
  input: { title: string; kind: string; label?: string; extension: string }
  expected: { sanitizedTitle?: string; fileName?: string; error?: string }
}

describe('naming vectors (contracts/naming-cases.json)', () => {
  it.each((vectors.cases as NamingCase[]).map((c) => [c.name, c] as const))('%s', (_, c) => {
    const item = {
      title: c.input.title,
      kind: c.input.kind as ItemKind,
      ...(c.input.label === undefined ? {} : { label: c.input.label }),
    }
    const file = itemFileName(item, c.input.extension)
    if (c.expected.error) {
      expect(file).toEqual({ ok: false, field: c.expected.error })
      return
    }
    expect(gameFolder(c.input.title)).toEqual({ ok: true, name: c.expected.sanitizedTitle })
    expect(file).toEqual({ ok: true, name: c.expected.fileName })
  })

  it.each(vectors.extensionCases.map((c) => [c.name, c] as const))('extension of %s', (_, c) => {
    expect(extensionOf(c.name, c.known)).toBe(c.expected)
  })
})

describe('naming helpers', () => {
  it('normalizes versions and extensions typed by the user', () => {
    expect(normalizeVersion(' V1.2 ')).toBe('1.2')
    expect(normalizeVersion('1.')).toBeNull()
    expect(normalizeExtension('NKIT.ISO')).toBe('.nkit.iso')
    expect(normalizeExtension('.a-b')).toBeNull()
  })

  it('leaves nothing for a title of symbols only', () => {
    expect(sanitizeTitle(' ?*. ')).toBe('')
  })
})
