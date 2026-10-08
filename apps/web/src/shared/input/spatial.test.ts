import { pickNext, type Box } from './spatial'

const box = (left: number, top: number, w = 100, h = 40): Box => ({
  left,
  top,
  right: left + w,
  bottom: top + h,
})

describe('pickNext', () => {
  // A row of three buttons above a wide field.
  const a = box(0, 0)
  const b = box(120, 0)
  const c = box(240, 0)
  const field = box(0, 100, 340)

  it('moves along a row', () => {
    expect(pickNext(a, [b, c, field], 'right')).toBe(0)
    expect(pickNext(c, [a, b, field], 'left')).toBe(1)
  })

  it('moves to the row below and back to the nearest above', () => {
    expect(pickNext(b, [a, c, field], 'down')).toBe(2)
    expect(pickNext(field, [a, b, c], 'up')).toBe(0) // overlaps all three: the closest ahead wins, ties go to the first
  })

  it('prefers an aligned neighbour over a closer one off-axis', () => {
    const from = box(0, 0)
    const aligned = box(400, 0)
    const offAxis = box(140, 200)
    expect(pickNext(from, [offAxis, aligned], 'right')).toBe(1)
  })

  it('returns -1 when nothing lies ahead', () => {
    expect(pickNext(c, [a, b, field], 'right')).toBe(-1)
    expect(pickNext(a, [b, c], 'up')).toBe(-1)
  })
})
