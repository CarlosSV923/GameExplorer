import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { useGamepadConnected } from '../input'
import { cx } from './cx'
import { ChevronLeftIcon } from './icons'

export type GlyphName = 'A' | 'B' | 'X' | 'Y' | 'RT' | 'LB' | 'RB' | 'MENU' | 'DPAD'

/** A gamepad button drawn in the help bar (own drawings, no brand marks). */
export function Glyph({ name }: { name: GlyphName }) {
  const { t } = useTranslation()
  const label = t(`glyph.${name}`)
  if (name === 'DPAD') {
    return (
      <svg role="img" aria-label={label} width="22" height="22" viewBox="0 0 22 22">
        <path d="M8 1h6v7h7v6h-7v7H8v-7H1V8h7z" fill="currentColor" />
      </svg>
    )
  }
  if (name === 'MENU') {
    return (
      <span
        role="img"
        aria-label={label}
        className="inline-flex size-6 items-center justify-center rounded-full bg-ink-1 text-bar"
      >
        <svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true">
          <path
            d="M2 3h8M2 6h8M2 9h8"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinecap="round"
          />
        </svg>
      </span>
    )
  }
  const trigger = name === 'RT' || name === 'LB' || name === 'RB'
  return (
    <span
      role="img"
      aria-label={label}
      className={cx(
        'inline-flex h-6 items-center justify-center bg-ink-1 font-bold text-bar',
        trigger ? 'rounded-t-sm rounded-b-xl px-2 text-chip' : 'w-6 rounded-full text-caption',
      )}
    >
      <span aria-hidden="true">{name}</span>
    </span>
  )
}

export interface HelpAction {
  glyph: GlyphName
  label: string
}

interface HelpBarProps {
  /** What each gamepad button does on this screen. */
  actions: readonly HelpAction[]
  /** What to show without a gamepad: real buttons, or nothing. */
  children?: ReactNode
  /** Shown at the end in both modes (the language switch on the login). */
  aside?: ReactNode
}

/**
 * The bottom bar (RNF-06): gamepad glyphs only while a gamepad is
 * connected; otherwise regular buttons (children) or no bar at all.
 */
export function HelpBar({ actions, children, aside }: HelpBarProps) {
  const gamepad = useGamepadConnected()
  if (gamepad) {
    return (
      <footer className="flex flex-wrap items-center justify-center gap-x-8 gap-y-3 bg-bar px-6 py-4 text-heading font-medium">
        {actions.map((a) => (
          <span key={a.glyph + a.label} className="inline-flex items-center gap-2.5">
            <Glyph name={a.glyph} />
            {a.label}
          </span>
        ))}
        {aside}
      </footer>
    )
  }
  if (!children && !aside) return null
  return (
    <footer className="flex flex-wrap items-center justify-between gap-x-6 gap-y-3 bg-bar px-4 py-3 sm:px-6 lg:px-10">
      {children}
      {aside}
    </footer>
  )
}

/** Classes of the 44 px "Back" link above a page title (a router link). */
export const backLinkClass =
  'inline-flex min-h-control-sm items-center gap-1.5 self-start text-body-sm text-ink-2 no-underline hover:text-ink-1'

interface PageHeaderProps {
  /** The back link (rendered by the caller: it is a router link). */
  back?: ReactNode
  title: ReactNode
  /** A line under the title (file name, counts). */
  meta?: ReactNode
  aside?: ReactNode
}

/** Back link, the page's h1 and an optional aside. */
export function PageHeader({ back, title, meta, aside }: PageHeaderProps) {
  return (
    <header className="flex flex-wrap items-end justify-between gap-4 border-b border-line px-4 pt-7 pb-5 sm:px-6 lg:px-10">
      <div className="flex min-w-0 flex-col gap-1.5">
        {back}
        <h1 className="m-0 text-display font-bold tracking-display break-words uppercase">
          {title}
        </h1>
        {meta}
      </div>
      {aside && <div className="flex flex-wrap items-center gap-4">{aside}</div>}
    </header>
  )
}

/** Back link content: chevron and label. */
export function BackLabel({ children }: { children: ReactNode }) {
  return (
    <>
      <ChevronLeftIcon size={16} />
      {children}
    </>
  )
}

/** A table container that scrolls sideways on narrow screens. */
export function TableBox({ children }: { children: ReactNode }) {
  // relative: screen-reader-only labels (position: absolute) stay inside the
  // scrolling box instead of widening the page.
  return <div className="relative overflow-x-auto rounded-lg border border-line">{children}</div>
}
