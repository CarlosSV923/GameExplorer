import {
  useId,
  type ComponentPropsWithRef,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
} from 'react'

import { cx } from './cx'
import { ChevronDownIcon, ErrorIcon, SearchIcon } from './icons'

interface TextFieldProps extends Omit<ComponentPropsWithRef<'input'>, 'size'> {
  label: string
  /** Keep the label for screen readers only. */
  hideLabel?: boolean
  hint?: string
  error?: string
  size?: 'lg' | 'md'
}

/** A labelled input; hint and error are tied to it with aria-describedby. */
export function TextField({
  label,
  hideLabel = false,
  hint,
  error,
  size = 'md',
  className,
  ...rest
}: TextFieldProps) {
  const id = useId()
  const hintId = `${id}-hint`
  const errorId = `${id}-error`
  const describedBy = [hint && hintId, error && errorId].filter(Boolean).join(' ') || undefined
  return (
    <div className="flex flex-col gap-2">
      <label
        htmlFor={id}
        className={cx('text-caption font-bold text-ink-2', hideLabel && 'sr-only')}
      >
        {label}
      </label>
      <input
        id={id}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        className={cx(
          'box-border w-full rounded-md border bg-surface-field px-4 text-ink-1 placeholder:text-ink-3',
          size === 'lg'
            ? 'h-control-lg rounded-lg border-2 text-body-lg'
            : 'h-control-md text-body',
          error ? 'border-danger' : size === 'lg' ? 'border-accent' : 'border-control',
          className,
        )}
        {...rest}
      />
      {hint && !error && (
        <span id={hintId} className="text-caption text-ink-3">
          {hint}
        </span>
      )}
      {error && (
        <span id={errorId} className="flex items-start gap-2 text-body-sm text-danger">
          <ErrorIcon className="mt-px shrink-0" />
          {error}
        </span>
      )}
    </div>
  )
}

interface SelectFieldProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'size'> {
  label: string
  hideLabel?: boolean
  hint?: ReactNode
  error?: string
  children: ReactNode
}

/** A native select (gamepad and touch friendly), styled like the fields. */
export function SelectField({
  label,
  hideLabel = false,
  hint,
  error,
  className,
  children,
  ...rest
}: SelectFieldProps) {
  const id = useId()
  const errorId = `${id}-error`
  return (
    <div className="flex flex-col gap-2">
      <label
        htmlFor={id}
        className={cx('text-caption font-bold text-ink-2', hideLabel && 'sr-only')}
      >
        {label}
      </label>
      <span className="relative flex">
        <select
          id={id}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? errorId : undefined}
          className={cx(
            'box-border h-13 w-full cursor-pointer appearance-none rounded-md border bg-surface-field pr-12 pl-4 text-body-lg font-bold text-ink-1',
            error ? 'border-danger' : 'border-control',
            className,
          )}
          {...rest}
        >
          {children}
        </select>
        <ChevronDownIcon className="pointer-events-none absolute top-1/2 right-4 -translate-y-1/2 text-ink-2" />
      </span>
      {hint}
      {error && (
        <span id={errorId} className="flex items-start gap-2 text-body-sm text-danger">
          <ErrorIcon className="mt-px shrink-0" />
          {error}
        </span>
      )}
    </div>
  )
}

interface SearchFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label: string
  /** Show the "/" shortcut hint; the field is focused with "/" or Y. */
  shortcut?: boolean
}

/** Search box with an icon; the label is for screen readers. */
export function SearchField({ label, shortcut = false, className, ...rest }: SearchFieldProps) {
  return (
    <label
      className={cx(
        'flex h-control-sm min-w-0 items-center gap-2.5 rounded-full bg-surface-field px-4 text-ink-2 focus-within:outline-3 focus-within:outline-offset-2 focus-within:outline-accent',
        className,
      )}
    >
      <SearchIcon />
      <span className="sr-only">{label}</span>
      <input
        type="search"
        data-shortcut={shortcut ? 'search' : undefined}
        className="min-w-0 flex-1 border-0 bg-transparent text-body-sm text-ink-1 outline-0 placeholder:text-ink-3"
        {...rest}
      />
      {shortcut && (
        <kbd
          aria-hidden="true"
          className="rounded-sm border border-control px-1.5 font-mono text-caption text-ink-3"
        >
          /
        </kbd>
      )}
    </label>
  )
}
