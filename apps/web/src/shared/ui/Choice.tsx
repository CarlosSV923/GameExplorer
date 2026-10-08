import type { ReactNode } from 'react'

import { cx } from './cx'

export type ChipKind = 'base' | 'update' | 'dlc' | 'disc' | 'whole' | 'missing' | 'neutral'

const chipKinds: Record<ChipKind, string> = {
  base: 'bg-kind-base text-on-accent',
  update: 'bg-kind-update text-on-kind-update',
  dlc: 'bg-kind-dlc text-on-accent',
  disc: 'bg-kind-disc text-on-accent',
  whole: 'border border-ink-1 text-ink-1',
  missing: 'border border-accent text-accent',
  neutral: 'bg-surface-raised text-ink-1',
}

/** Item kind or status pill (one color per kind). */
export function Chip({ kind = 'neutral', children }: { kind?: ChipKind; children: ReactNode }) {
  return (
    <span
      className={cx(
        'inline-block rounded-full px-2.5 py-1 text-chip font-bold tracking-display whitespace-nowrap uppercase',
        chipKinds[kind],
      )}
    >
      {children}
    </span>
  )
}

interface Option<T extends string> {
  value: T
  label: ReactNode
}

interface ChoiceChipsProps<T extends string> {
  legend: string
  name: string
  options: readonly Option<T>[]
  value: T | undefined
  onChange: (value: T) => void
  hint?: ReactNode
}

/** Radios drawn as chips (item kind in the review). Arrow keys stay native. */
export function ChoiceChips<T extends string>({
  legend,
  name,
  options,
  value,
  onChange,
  hint,
}: ChoiceChipsProps<T>) {
  return (
    <fieldset className="m-0 flex min-w-0 flex-wrap items-center gap-2 border-0 p-0">
      <legend className="sr-only">{legend}</legend>
      {options.map((o) => {
        const checked = o.value === value
        return (
          <label
            key={o.value}
            className={cx(
              'relative inline-flex h-control-sm cursor-pointer items-center rounded-full px-4 text-body-sm font-bold',
              checked
                ? 'bg-accent text-on-accent'
                : 'border border-control text-ink-1 hover:bg-hover',
            )}
          >
            <input
              type="radio"
              name={name}
              value={o.value}
              checked={checked}
              onChange={() => {
                onChange(o.value)
              }}
              className="absolute size-px opacity-0"
            />
            {o.label}
          </label>
        )
      })}
      {hint && <span className="text-caption text-ink-3">{hint}</span>}
    </fieldset>
  )
}

interface ListOption<T extends string> {
  value: T
  title: string
  meta?: string
  art?: ReactNode
  badge?: ReactNode
}

interface ChoiceListProps<T extends string> {
  legend: string
  name: string
  options: readonly ListOption<T>[]
  value: T | undefined
  onChange: (value: T) => void
}

/** Radios drawn as rows (IGDB results). Replaces role=option lists. */
export function ChoiceList<T extends string>({
  legend,
  name,
  options,
  value,
  onChange,
}: ChoiceListProps<T>) {
  return (
    <fieldset className="m-0 flex min-w-0 flex-col gap-1 rounded-lg border border-line bg-surface p-1.5">
      <legend className="sr-only">{legend}</legend>
      {options.map((o) => {
        const checked = o.value === value
        return (
          <label
            key={o.value}
            className={cx(
              'relative flex cursor-pointer items-center gap-3.5 rounded-md p-2',
              checked ? 'bg-accent text-on-accent' : 'hover:bg-hover',
            )}
          >
            <input
              type="radio"
              name={name}
              value={o.value}
              checked={checked}
              onChange={() => {
                onChange(o.value)
              }}
              className="absolute size-px opacity-0"
            />
            {o.art}
            <span className="flex min-w-0 flex-col gap-0.5">
              <span
                className={cx('truncate text-body-lg', checked ? 'font-bold' : 'font-semibold')}
              >
                {o.title}
              </span>
              {o.meta && (
                <span className={cx('text-caption', checked ? 'font-bold' : 'text-ink-2')}>
                  {o.meta}
                </span>
              )}
            </span>
            {o.badge && <span className="ml-auto">{o.badge}</span>}
          </label>
        )
      })}
    </fieldset>
  )
}

interface SegmentedToggleProps<T extends string> {
  label: string
  options: readonly Option<T>[]
  value: T
  onChange: (value: T) => void
}

/** A group of pressed/unpressed buttons (ES/EN). */
export function SegmentedToggle<T extends string>({
  label,
  options,
  value,
  onChange,
}: SegmentedToggleProps<T>) {
  return (
    <span
      role="group"
      aria-label={label}
      className="inline-flex gap-1 rounded-full bg-surface-input p-1"
    >
      {options.map((o) => {
        const pressed = o.value === value
        return (
          <button
            key={o.value}
            type="button"
            aria-pressed={pressed}
            onClick={() => {
              onChange(o.value)
            }}
            className={cx(
              'h-control-sm cursor-pointer rounded-full border-0 px-4 text-body-sm font-bold',
              pressed ? 'bg-ink-1 text-on-accent' : 'bg-transparent text-ink-2 hover:bg-hover',
            )}
          >
            {o.label}
          </button>
        )
      })}
    </span>
  )
}
