import { createContext } from 'react'

import type { Direction } from './spatial'

/**
 * Actions the screens react to (docs/design-handoff.md §4). Screens subscribe
 * to actions, never to keys or gamepad buttons.
 */
export type Action =
  'confirm' | 'back' | 'action1' | 'action2' | 'upload' | 'menu' | 'prev' | 'next' | 'search'

/** A handler returns false to let the next one (or the default) run. */
export type Handler = (direction?: Direction) => boolean | undefined

export interface InputApi {
  gamepad: boolean
  subscribe: (action: Action | 'navigate', handler: Handler) => () => void
  dispatch: (action: Action | 'navigate', direction?: Direction) => boolean
}

export const InputContext = createContext<InputApi | null>(null)

const nonTextInputs = new Set(['button', 'checkbox', 'submit', 'reset', 'file', 'image', 'color'])

/** Whether the element uses arrow keys itself (text fields, radios, selects). */
export function ownsArrows(el: Element | null): boolean {
  if (!el) return false
  if (el instanceof HTMLInputElement) return !nonTextInputs.has(el.type)
  return (
    el instanceof HTMLTextAreaElement ||
    el instanceof HTMLSelectElement ||
    (el as HTMLElement).isContentEditable
  )
}
