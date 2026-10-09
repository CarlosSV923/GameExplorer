import { useEffect, useId, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'

import { useAction } from '../input'
import { buttonClass } from './buttonClass'
import { cx } from './cx'

interface CardOption<T extends string> {
  value: T
  title: string
  /** Extensions (tile) or an explanation (row) under the title. */
  detail?: ReactNode
  /** Why it cannot be chosen goes in detail. */
  disabled?: boolean
}

interface OptionCardsProps<T extends string> {
  legend: string
  hideLegend?: boolean
  name: string
  options: readonly CardOption<T>[]
  value: T | undefined
  onChange: (value: T) => void
  /**
   * tile: a grid of cards filled when chosen (ConsolePicker); row: one per
   * line with a visible radio (UploadModePicker, target consoles).
   */
  appearance?: 'tile' | 'row'
  /** Under the cards, tied to the group. */
  hint?: ReactNode
}

/** Radios drawn as cards (docs/design-handoff.md §9). Arrow keys stay native. */
export function OptionCards<T extends string>({
  legend,
  hideLegend = false,
  name,
  options,
  value,
  onChange,
  appearance = 'tile',
  hint,
}: OptionCardsProps<T>) {
  const tile = appearance === 'tile'
  return (
    <fieldset className="m-0 flex min-w-0 flex-col gap-2.5 border-0 p-0">
      <legend
        className={cx(
          hideLegend
            ? 'sr-only'
            : 'mb-2.5 p-0 text-body-sm font-bold tracking-label text-ink-2 uppercase',
        )}
      >
        {legend}
      </legend>
      <div
        className={
          tile
            ? 'grid grid-cols-[repeat(auto-fit,minmax(170px,1fr))] gap-2.5'
            : 'flex flex-col gap-2'
        }
      >
        {options.map((o) => {
          const checked = o.value === value
          return (
            <label
              key={o.value}
              className={cx(
                'relative flex rounded-lg border has-focus-visible:outline-3 has-focus-visible:outline-offset-2 has-focus-visible:outline-accent',
                tile ? 'flex-col gap-1 px-4 py-3.5' : 'min-h-13 items-start gap-3.5 px-4 py-3.5',
                o.disabled
                  ? 'cursor-default border-line text-ink-3'
                  : checked
                    ? tile
                      ? 'cursor-pointer border-accent bg-accent text-on-accent'
                      : 'cursor-pointer border-accent bg-warning-surface text-ink-1'
                    : 'cursor-pointer border-control text-ink-1 hover:bg-hover',
              )}
            >
              <input
                type="radio"
                name={name}
                value={o.value}
                checked={checked}
                disabled={o.disabled}
                onChange={() => {
                  onChange(o.value)
                }}
                className={
                  tile ? 'absolute size-px opacity-0' : 'mt-0.5 size-5 shrink-0 accent-accent'
                }
              />
              {tile ? (
                <>
                  <span className="text-body-lg font-bold">{o.title}</span>
                  {o.detail && <span className="font-mono text-chip opacity-85">{o.detail}</span>}
                </>
              ) : (
                <span className="flex min-w-0 flex-col gap-0.5">
                  <span className="text-body-lg font-bold">{o.title}</span>
                  {o.detail && (
                    <span
                      className={cx(
                        'text-body-sm leading-snug font-semibold',
                        o.disabled ? 'text-ink-3' : 'text-ink-2',
                      )}
                    >
                      {o.detail}
                    </span>
                  )}
                </span>
              )}
            </label>
          )
        })}
      </div>
      {hint}
    </fieldset>
  )
}

/**
 * Where something will be stored (PathPreview): mono text in the success
 * color, or dimmed while the data is incomplete.
 */
export function PathPreview({
  label,
  complete = true,
  children,
}: {
  label: string
  complete?: boolean
  children: ReactNode
}) {
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <span className="text-caption font-bold text-ink-2">{label}</span>
      <span
        className={cx(
          'rounded-md bg-surface-input px-3.5 py-3 font-mono text-caption [overflow-wrap:anywhere]',
          complete ? 'text-success' : 'text-ink-3',
        )}
      >
        {children}
      </span>
    </div>
  )
}

interface MenuItem {
  label: string
  onSelect: () => void
  tone?: 'default' | 'danger'
}

/**
 * "More" actions (MoreMenu): a button with a menu of buttons. Arrows move
 * through the items, Escape or B close it and focus returns to the button.
 */
export function MoreMenu({
  label,
  items,
  children,
  compact = false,
  className,
}: {
  /** The button's accessible name. */
  label: string
  items: readonly MenuItem[]
  /** The button's content (an icon, or an icon and text). */
  children: ReactNode
  /** An icon-only round button (rows of a table). */
  compact?: boolean
  className?: string
}) {
  const id = useId()
  const [open, setOpen] = useState(false)
  const box = useRef<HTMLDivElement>(null)
  const button = useRef<HTMLButtonElement>(null)

  const close = (refocus = true) => {
    setOpen(false)
    if (refocus) button.current?.focus()
  }

  useAction(
    'back',
    () => {
      close()
      return true
    },
    open,
  )

  useEffect(() => {
    if (!open) return
    box.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus()
    const onDown = (e: PointerEvent) => {
      if (!box.current?.contains(e.target as Node)) setOpen(false)
    }
    window.addEventListener('pointerdown', onDown)
    return () => {
      window.removeEventListener('pointerdown', onDown)
    }
  }, [open])

  const onKeyDown = (e: KeyboardEvent) => {
    if (!open) return
    const list = Array.from(box.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])
    const at = list.indexOf(document.activeElement as HTMLElement)
    let next: HTMLElement | undefined
    if (e.key === 'ArrowDown') next = list[(at + 1) % list.length]
    else if (e.key === 'ArrowUp') next = list[(at - 1 + list.length) % list.length]
    else if (e.key === 'Home') next = list[0]
    else if (e.key === 'End') next = list[list.length - 1]
    else if (e.key === 'Escape') {
      e.preventDefault()
      close()
      return
    } else if (e.key === 'Tab') {
      close(false)
      return
    }
    if (next) {
      e.preventDefault()
      next.focus()
    }
  }

  return (
    <div ref={box} className={cx('relative inline-flex', className)} onKeyDown={onKeyDown}>
      <button
        ref={button}
        type="button"
        aria-label={label}
        title={label}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? id : undefined}
        onClick={() => {
          setOpen((o) => !o)
        }}
        className={
          compact
            ? 'inline-flex size-control-sm cursor-pointer items-center justify-center rounded-full border-0 bg-transparent text-ink-1 hover:bg-hover'
            : buttonClass('secondary', 'md')
        }
      >
        {children}
      </button>
      {open && (
        <ul
          id={id}
          role="menu"
          aria-label={label}
          className="absolute top-full right-0 z-30 m-0 mt-2 flex min-w-60 list-none flex-col gap-0.5 rounded-lg border border-control bg-surface-card p-1.5 shadow-lg"
        >
          {items.map((item) => (
            <li key={item.label} role="none">
              <button
                type="button"
                role="menuitem"
                onClick={() => {
                  close()
                  item.onSelect()
                }}
                className={cx(
                  'flex min-h-control-sm w-full cursor-pointer items-center rounded-md border-0 bg-transparent px-3.5 text-left text-body font-semibold whitespace-nowrap hover:bg-hover focus-visible:bg-hover',
                  item.tone === 'danger' ? 'text-danger' : 'text-ink-1',
                )}
              >
                {item.label}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
