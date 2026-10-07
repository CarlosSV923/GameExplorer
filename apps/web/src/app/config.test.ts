import { resolveDataSource } from './config'

describe('resolveDataSource', () => {
  it('selects the demo adapters only when explicitly requested', () => {
    expect(resolveDataSource('demo')).toBe('demo')
  })

  it.each([undefined, '', 'api', 'DEMO', 'mock'])('falls back to the API for %j', (raw) => {
    expect(resolveDataSource(raw)).toBe('api')
  })
})
