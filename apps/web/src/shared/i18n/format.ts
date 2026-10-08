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
