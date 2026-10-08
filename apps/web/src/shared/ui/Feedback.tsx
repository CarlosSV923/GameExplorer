import type { ReactNode } from 'react'

import { cx } from './cx'
import { ErrorIcon, WarningIcon } from './icons'

/** Busy indicator; label is announced, the drawing is decorative. */
export function Spinner({ label, size = 18 }: { label: string; size?: number }) {
  return (
    <span role="status" className="inline-flex">
      <svg
        width={size}
        height={size}
        viewBox="0 0 24 24"
        aria-hidden="true"
        className="animate-spin"
      >
        <circle
          cx="12"
          cy="12"
          r="9"
          fill="none"
          stroke="currentColor"
          strokeOpacity="0.3"
          strokeWidth="3"
        />
        <path
          d="M21 12a9 9 0 0 0-9-9"
          fill="none"
          stroke="currentColor"
          strokeWidth="3"
          strokeLinecap="round"
        />
      </svg>
      <span className="sr-only">{label}</span>
    </span>
  )
}

type BannerTone = 'warning' | 'danger' | 'info'

const bannerTones: Record<BannerTone, string> = {
  warning: 'bg-warning-surface border-accent text-warning-ink',
  danger: 'bg-danger-surface border-danger text-danger-ink',
  info: 'bg-surface border-line text-ink-2',
}

/**
 * A notice. Warnings and info are polite status messages; danger is an
 * alert, announced at once (WCAG 4.1.3).
 */
export function Banner({
  tone = 'warning',
  action,
  children,
}: {
  tone?: BannerTone
  action?: ReactNode
  children: ReactNode
}) {
  const Icon = tone === 'danger' ? ErrorIcon : WarningIcon
  return (
    <div
      role={tone === 'danger' ? 'alert' : 'status'}
      className={cx(
        'flex flex-wrap items-start justify-between gap-3 rounded-md border px-4 py-3.5 text-body-sm leading-[1.45]',
        bannerTones[tone],
      )}
    >
      <span className="flex min-w-0 items-start gap-3">
        <Icon
          size={20}
          className={cx('mt-px shrink-0', tone === 'danger' ? 'text-danger' : 'text-accent')}
        />
        <span>{children}</span>
      </span>
      {action}
    </div>
  )
}

/** Determinate progress with its accessible name (WCAG 4.1.2). */
export function ProgressBar({
  value,
  label,
  tone = 'upload',
}: {
  value: number
  label: string
  tone?: 'upload' | 'extract'
}) {
  const pct = Math.min(100, Math.max(0, Math.round(value)))
  return (
    <div
      role="progressbar"
      aria-label={label}
      aria-valuenow={pct}
      aria-valuemin={0}
      aria-valuemax={100}
      className="h-2 overflow-hidden rounded-full bg-line"
    >
      <div
        className={cx(
          'h-full transition-[width] duration-300 ease-out',
          tone === 'extract' ? 'bg-progress-extract' : 'bg-progress-upload',
        )}
        style={{ width: `${String(pct)}%` }}
      />
    </div>
  )
}

/** Centered empty state (States.dc.html). */
export function EmptyState({
  icon,
  title,
  body,
  action,
}: {
  icon?: ReactNode
  title: string
  body: string
  action?: ReactNode
}) {
  return (
    <section className="flex min-h-80 flex-col items-center justify-center gap-3.5 rounded-xl border border-line p-8 text-center">
      {icon && <span className="text-ink-2">{icon}</span>}
      <h2 className="m-0 text-heading font-bold">{title}</h2>
      <p className="m-0 max-w-[36ch] text-body text-ink-2">{body}</p>
      {action}
    </section>
  )
}
