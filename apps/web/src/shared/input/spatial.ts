/**
 * Spatial navigation (docs/design-handoff.md §4): the d-pad (and the arrow
 * keys outside text fields) move focus to the nearest focusable element in
 * that direction. Works on native elements: nothing has to be registered.
 */

export type Direction = 'up' | 'down' | 'left' | 'right'

export interface Box {
  left: number
  top: number
  right: number
  bottom: number
}

/** How much a sideways offset costs compared with distance ahead. */
const sidewaysWeight = 2

/**
 * Picks the candidate to move to from `from` in `direction`, or -1. A
 * candidate must lie ahead (its near edge beyond the current center); among
 * those, the one closest ahead wins, with sideways misalignment penalized so
 * a row neighbour beats a closer element one row off.
 */
export function pickNext(from: Box, candidates: readonly Box[], direction: Direction): number {
  const cx = (from.left + from.right) / 2
  const cy = (from.top + from.bottom) / 2
  let best = -1
  let bestScore = Infinity
  candidates.forEach((c, i) => {
    let ahead: number
    let sideways: number
    switch (direction) {
      case 'right':
        if (c.left < cx || c.right <= from.right) return
        ahead = Math.max(0, c.left - from.right)
        sideways = gap(from.top, from.bottom, c.top, c.bottom)
        break
      case 'left':
        if (c.right > cx || c.left >= from.left) return
        ahead = Math.max(0, from.left - c.right)
        sideways = gap(from.top, from.bottom, c.top, c.bottom)
        break
      case 'down':
        if (c.top < cy || c.bottom <= from.bottom) return
        ahead = Math.max(0, c.top - from.bottom)
        sideways = gap(from.left, from.right, c.left, c.right)
        break
      case 'up':
        if (c.bottom > cy || c.top >= from.top) return
        ahead = Math.max(0, from.top - c.bottom)
        sideways = gap(from.left, from.right, c.left, c.right)
        break
    }
    const score = ahead + sidewaysWeight * sideways
    if (score < bestScore) {
      bestScore = score
      best = i
    }
  })
  return best
}

/** Distance between two intervals on one axis (0 when they overlap). */
function gap(a1: number, a2: number, b1: number, b2: number): number {
  if (b2 < a1) return a1 - b2
  if (b1 > a2) return b1 - a2
  return 0
}

const focusableSelector = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

/** The element whose box represents a focusable (a hidden input's label). */
function visualOf(el: HTMLElement): HTMLElement {
  if (el instanceof HTMLInputElement && el.offsetWidth <= 1 && el.offsetHeight <= 1) {
    return el.closest('label') ?? el
  }
  return el
}

/** Focusable elements inside root, in document order (no layout needed). */
export function tabbables(root: ParentNode): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(focusableSelector)).filter(
    (el) => !el.closest('[inert], [aria-hidden="true"]'),
  )
}

/** Focusable, visible elements inside root, with the box to compare. */
export function focusables(root: ParentNode): { el: HTMLElement; box: Box }[] {
  const out: { el: HTMLElement; box: Box }[] = []
  for (const el of tabbables(root)) {
    const rect = visualOf(el).getBoundingClientRect()
    if (rect.width === 0 && rect.height === 0) continue
    out.push({ el, box: rect })
  }
  return out
}

/** The region focus may move in: the open modal dialog, or the document. */
export function navigationRoot(doc: Document = document): ParentNode {
  const dialogs = doc.querySelectorAll('[aria-modal="true"]')
  return dialogs[dialogs.length - 1] ?? doc.body
}

/** Moves focus in a direction; returns whether it moved. */
export function moveFocus(direction: Direction, doc: Document = document): boolean {
  const items = focusables(navigationRoot(doc))
  const current = doc.activeElement instanceof HTMLElement ? doc.activeElement : null
  const index = current ? items.findIndex((i) => i.el === current) : -1
  if (index < 0) {
    // Nothing focused yet: start at the first element.
    items[0]?.el.focus()
    return items.length > 0
  }
  const from = items[index]
  if (!from) return false
  const others = items.filter((_, i) => i !== index)
  const next = pickNext(
    from.box,
    others.map((o) => o.box),
    direction,
  )
  const target = others[next]
  if (!target) return false
  target.el.focus()
  // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition -- jsdom has no scrollIntoView
  target.el.scrollIntoView?.({ block: 'nearest', inline: 'nearest' })
  return true
}
