import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { defaultLanguage, isLanguage, type Language } from '.'
import { daysUntil, formatDate, formatRelative, formatSize } from './format'

/** The interface language. */
export function useLanguage(): Language {
  const { i18n } = useTranslation()
  return isLanguage(i18n.language) ? i18n.language : defaultLanguage
}

/** Formatters bound to the current language. */
export function useFormat() {
  const { t } = useTranslation()
  const language = useLanguage()
  return useMemo(
    () => ({
      size: (bytes: number) => formatSize(bytes, language, t),
      date: (iso: string) => formatDate(new Date(iso), language),
      relative: (iso: string, now: number) => formatRelative(new Date(iso), now, language),
      daysUntil: (iso: string, now: number) => daysUntil(new Date(iso), now),
    }),
    [language, t],
  )
}
