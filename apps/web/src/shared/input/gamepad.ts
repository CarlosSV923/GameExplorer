import type { Direction } from './spatial'

/** Buttons of the standard gamepad mapping that the app uses. */
export type PadButton = 'A' | 'B' | 'X' | 'Y' | 'LB' | 'RB' | 'RT' | 'MENU'

export type PadEvent =
  { type: 'button'; button: PadButton } | { type: 'move'; direction: Direction }

// https://w3c.github.io/gamepad/#remapping
const buttonIndex: Record<PadButton, number> = {
  A: 0,
  B: 1,
  X: 2,
  Y: 3,
  LB: 4,
  RB: 5,
  RT: 7,
  MENU: 9,
}
const dpadIndex: Record<Direction, number> = { up: 12, down: 13, left: 14, right: 15 }

export const deadZone = 0.5
export const repeatDelay = 400
export const repeatInterval = 120

/** The subset of the Gamepad API the reader needs (easy to fake in tests). */
export interface PadState {
  buttons: readonly { pressed: boolean }[]
  axes: readonly number[]
}

/**
 * Turns polled gamepad state into events: a button fires when pressed;
 * a direction (d-pad or left stick) fires when pressed and then repeats
 * after repeatDelay, every repeatInterval, while held.
 */
export class GamepadReader {
  private pressed = new Set<PadButton>()
  private held: { direction: Direction; next: number } | null = null

  poll(pads: readonly (PadState | null)[], now: number): PadEvent[] {
    const events: PadEvent[] = []
    const down = new Set<PadButton>()
    let direction: Direction | null = null

    for (const pad of pads) {
      if (!pad) continue
      for (const [name, i] of Object.entries(buttonIndex) as [PadButton, number][]) {
        if (pad.buttons[i]?.pressed) down.add(name)
      }
      direction ??= padDirection(pad)
    }

    for (const b of down) {
      if (!this.pressed.has(b)) events.push({ type: 'button', button: b })
    }
    this.pressed = down

    if (!direction) {
      this.held = null
    } else if (this.held?.direction !== direction) {
      this.held = { direction, next: now + repeatDelay }
      events.push({ type: 'move', direction })
    } else if (now >= this.held.next) {
      this.held.next = now + repeatInterval
      events.push({ type: 'move', direction })
    }
    return events
  }
}

function padDirection(pad: PadState): Direction | null {
  for (const [dir, i] of Object.entries(dpadIndex) as [Direction, number][]) {
    if (pad.buttons[i]?.pressed) return dir
  }
  const x = pad.axes[0] ?? 0
  const y = pad.axes[1] ?? 0
  if (Math.max(Math.abs(x), Math.abs(y)) < deadZone) return null
  if (Math.abs(x) > Math.abs(y)) return x > 0 ? 'right' : 'left'
  return y > 0 ? 'down' : 'up'
}
