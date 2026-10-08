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
}

/**
 * The bottom bar (RNF-06): gamepad glyphs only while a gamepad is
 * connected; otherwise regular buttons (children) or no bar at all.
 */
export function HelpBar({ actions, children }: HelpBarProps) {
  const gamepad = useGamepadConnected()
  if (gamepad) {
    return (
      <footer className="flex flex-wrap items-center justify-center gap-x-8 gap-y-3 bg-bar px-6 py-4 text-heading font-medium">
        {actions.map((a) => (
          <span key={a.glyph} className="inline-flex items-center gap-2.5">
            <Glyph name={a.glyph} />
            {a.label}
          </span>
        ))}
      </footer>
    )
  }
  if (!children) return null
  return (
    <footer className="flex flex-wrap items-center justify-between gap-x-6 gap-y-3 bg-bar px-4 py-3 sm:px-6 lg:px-10">
      {children}
    </footer>
  )
}

interface PageHeaderProps {
  back?: { href: string; label: string }
  title: ReactNode
  aside?: ReactNode
}

/** Back link (44 px), the page's h1 and an optional aside. */
export function PageHeader({ back, title, aside }: PageHeaderProps) {
  return (
    <header className="flex flex-wrap items-end justify-between gap-4 border-b border-line px-4 pt-7 pb-5 sm:px-6 lg:px-10">
      <div className="flex flex-col gap-1.5">
        {back && (
          <a
            href={back.href}
            className="inline-flex min-h-control-sm items-center gap-1.5 text-body-sm text-ink-2 no-underline hover:text-ink-1"
          >
            <ChevronLeftIcon size={16} />
            {back.label}
          </a>
        )}
        <h1 className="m-0 text-display font-bold tracking-display uppercase">{title}</h1>
      </div>
      {aside}
    </header>
  )
}

/** A table container that scrolls sideways on narrow screens. */
export function TableBox({ children }: { children: ReactNode }) {
  return <div className="overflow-x-auto rounded-lg border border-line">{children}</div>
}
