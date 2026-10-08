import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'

import { GamepadReader, type PadButton, type PadEvent, type PadState } from './gamepad'
import { InputContext, ownsArrows, type Action, type Handler, type InputApi } from './context'
import { moveFocus, type Direction } from './spatial'

const padActions: Record<PadButton, Action> = {
  A: 'confirm',
  B: 'back',
  X: 'action1',
  Y: 'action2',
  RT: 'upload',
  MENU: 'menu',
  LB: 'prev',
  RB: 'next',
}

const arrowKeys: Record<string, Direction> = {
  ArrowUp: 'up',
  ArrowDown: 'down',
  ArrowLeft: 'left',
  ArrowRight: 'right',
}

/** Default behaviour when no screen handled an action. */
function fallback(action: Action | 'navigate', direction?: Direction): boolean {
  switch (action) {
    case 'navigate':
      return direction ? moveFocus(direction) : false
    case 'confirm': {
      const el = document.activeElement
      if (el instanceof HTMLElement && el !== document.body) {
        el.click()
        return true
      }
      return false
    }
    case 'search': {
      const field = document.querySelector<HTMLElement>('[data-shortcut="search"]')
      field?.focus()
      return field !== null
    }
    default:
      return false
  }
}

function connectedPads(): (PadState | null)[] {
  return typeof navigator.getGamepads === 'function' ? Array.from(navigator.getGamepads()) : []
}

export function InputProvider({ children }: { children: ReactNode }) {
  const handlers = useRef(new Map<Action | 'navigate', Handler[]>())
  const [gamepad, setGamepad] = useState(() => connectedPads().some(Boolean))

  const api = useMemo<InputApi>(() => {
    const dispatch = (action: Action | 'navigate', direction?: Direction) => {
      const list = handlers.current.get(action) ?? []
      // The newest subscriber (an open dialog, the focused screen) goes first.
      for (let i = list.length - 1; i >= 0; i--) {
        if (list[i]?.(direction) !== false) return true
      }
      return fallback(action, direction)
    }
    return {
      gamepad,
      dispatch,
      subscribe: (action, handler) => {
        const list = handlers.current.get(action) ?? []
        handlers.current.set(action, [...list, handler])
        return () => {
          handlers.current.set(
            action,
            (handlers.current.get(action) ?? []).filter((h) => h !== handler),
          )
        }
      },
    }
  }, [gamepad])

  // Keyboard: Escape, "/" and the arrows; Enter and Space stay native.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.altKey || e.ctrlKey || e.metaKey) return
      let handled = false
      if (e.key === 'Escape') {
        handled = api.dispatch('back')
      } else if (e.key === '/' && !ownsArrows(document.activeElement)) {
        handled = api.dispatch('search')
      } else if (e.key in arrowKeys && !ownsArrows(document.activeElement)) {
        handled = api.dispatch('navigate', arrowKeys[e.key])
      }
      if (handled) e.preventDefault()
    }
    window.addEventListener('keydown', onKey)
    return () => {
      window.removeEventListener('keydown', onKey)
    }
  }, [api])

  // Gamepad: connection events, then polling once per frame while connected.
  useEffect(() => {
    const update = () => {
      setGamepad(connectedPads().some(Boolean))
    }
    window.addEventListener('gamepadconnected', update)
    window.addEventListener('gamepaddisconnected', update)
    return () => {
      window.removeEventListener('gamepadconnected', update)
      window.removeEventListener('gamepaddisconnected', update)
    }
  }, [])

  useEffect(() => {
    if (!gamepad) return
    const reader = new GamepadReader()
    let frame = 0
    const tick = (now: number) => {
      for (const event of reader.poll(connectedPads(), now)) handlePad(api, event)
      frame = requestAnimationFrame(tick)
    }
    frame = requestAnimationFrame(tick)
    return () => {
      cancelAnimationFrame(frame)
    }
  }, [gamepad, api])

  return <InputContext value={api}>{children}</InputContext>
}

function handlePad(api: InputApi, event: PadEvent) {
  if (event.type === 'move') {
    api.dispatch('navigate', event.direction)
  } else {
    api.dispatch(padActions[event.button])
  }
}
