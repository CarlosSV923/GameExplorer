import type { TFunction } from 'i18next'

import type { Language } from '.'

const units = ['bytes', 'kb', 'mb', 'gb', 'tb'] as const

/**
 * Formats a size in decimal units, as disks and the NAS report them:
 * "22,7 GB" in Spanish, "22.7 GB" in English.
 */
export function formatSize(bytes: number, language: Language, t: TFunction): string {
  let value = Math.max(0, bytes)
  let unit = 0
  while (value >= 1000 && unit < units.length - 1) {
    value /= 1000
    unit++
  }
  const digits = unit === 0 || value >= 100 ? 0 : 1
  const number = new Intl.NumberFormat(language, {
    maximumFractionDigits: digits,
    minimumFractionDigits: digits,
  }).format(value)
  return t(`size.${units[unit] ?? 'tb'}`, { value: number })
}

/** Short date: "6 oct 2026" / "Oct 6, 2026". */
export function formatDate(date: Date, language: Language): string {
  return new Intl.DateTimeFormat(language, { day: 'numeric', month: 'short', year: 'numeric' })
    .format(date)
    .replace(/\./g, '')
}

const relativeSteps: [Intl.RelativeTimeFormatUnit, number][] = [
  ['second', 60],
  ['minute', 60],
  ['hour', 24],
  ['day', 30],
  ['month', 12],
  ['year', Infinity],
]

/** "hace 12 min" / "12 min. ago", "en 6 días" / "in 6 days". */
export function formatRelative(date: Date, now: number, language: Language): string {
  let value = (date.getTime() - now) / 1000
  let unit: Intl.RelativeTimeFormatUnit = 'second'
  for (const [step, size] of relativeSteps) {
    unit = step
    if (Math.abs(value) < size) break
    value /= size
  }
  if (unit === 'second') {
    value = 0
    unit = 'minute'
  }
  return new Intl.RelativeTimeFormat(language, { numeric: 'auto', style: 'short' }).format(
    Math.round(value),
    unit,
  )
}

/** Days from now until date, to the nearest day; the last day counts as 1. */
export function daysUntil(date: Date, now: number): number {
  return Math.max(1, Math.round((date.getTime() - now) / 86_400_000))
}
