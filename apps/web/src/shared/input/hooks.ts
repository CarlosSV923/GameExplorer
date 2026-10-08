import { use, useEffect, useRef } from 'react'

import { InputContext, type Action, type Handler, type InputApi } from './context'

function useInput(): InputApi {
  const api = use(InputContext)
  if (!api) throw new Error('useInput must be used inside <InputProvider>')
  return api
}

/** Whether a gamepad is connected: show glyphs only then (RNF-06). */
export function useGamepadConnected(): boolean {
  return useInput().gamepad
}

/**
 * Runs handler for an action while the component is mounted (and enabled).
 * Return false from the handler to let others handle it.
 */
export function useAction(action: Action | 'navigate', handler: Handler, enabled = true) {
  const { subscribe } = useInput()
  const latest = useRef(handler)
  useEffect(() => {
    latest.current = handler
  })
  useEffect(() => {
    if (!enabled) return
    return subscribe(action, (direction) => latest.current(direction))
  }, [action, enabled, subscribe])
}

/** Dispatches an action programmatically (e.g. a help bar button). */
export function useDispatch() {
  return useInput().dispatch
}
