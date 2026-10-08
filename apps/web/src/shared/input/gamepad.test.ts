import { GamepadReader, repeatDelay, repeatInterval, type PadState } from './gamepad'

function pad(pressed: number[] = [], axes: number[] = [0, 0]): PadState {
  return { buttons: Array.from({ length: 17 }, (_, i) => ({ pressed: pressed.includes(i) })), axes }
}

describe('GamepadReader', () => {
  it('fires a button once per press', () => {
    const r = new GamepadReader()
    expect(r.poll([pad([0])], 0)).toEqual([{ type: 'button', button: 'A' }])
    expect(r.poll([pad([0])], 16)).toEqual([])
    expect(r.poll([pad()], 32)).toEqual([])
    expect(r.poll([pad([0, 7])], 48)).toEqual([
      { type: 'button', button: 'A' },
      { type: 'button', button: 'RT' },
    ])
  })

  it('repeats a held direction after a delay', () => {
    const r = new GamepadReader()
    const right = pad([15])
    expect(r.poll([right], 0)).toEqual([{ type: 'move', direction: 'right' }])
    expect(r.poll([right], repeatDelay - 1)).toEqual([])
    expect(r.poll([right], repeatDelay)).toEqual([{ type: 'move', direction: 'right' }])
    expect(r.poll([right], repeatDelay + repeatInterval - 1)).toEqual([])
    expect(r.poll([right], repeatDelay + repeatInterval)).toHaveLength(1)
  })

  it('reads the left stick outside the dead zone', () => {
    const r = new GamepadReader()
    expect(r.poll([pad([], [0.3, 0.2])], 0)).toEqual([])
    expect(r.poll([pad([], [0.1, 0.9])], 10)).toEqual([{ type: 'move', direction: 'down' }])
    expect(r.poll([pad([], [-0.8, 0.2])], 20)).toEqual([{ type: 'move', direction: 'left' }])
  })

  it('ignores empty slots', () => {
    expect(new GamepadReader().poll([null, pad([1])], 0)).toEqual([{ type: 'button', button: 'B' }])
  })
})
