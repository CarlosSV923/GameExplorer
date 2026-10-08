import { useTranslation } from 'react-i18next'

import { SegmentedToggle } from '@/shared/ui'

import { changeLanguage, type Language } from '.'
import { useLanguage } from './hooks'

/** ES / EN switch (RF-51); `long` shows "Español" / "English". */
export function LanguageToggle({ long = false }: { long?: boolean }) {
  const { t, i18n } = useTranslation()
  const language = useLanguage()
  return (
    <SegmentedToggle<Language>
      label={t('language.label')}
      value={language}
      onChange={(next) => void changeLanguage(i18n, next)}
      options={[
        { value: 'es', label: long ? t('language.es') : t('language.esShort') },
        { value: 'en', label: long ? t('language.en') : t('language.enShort') },
      ]}
    />
  )
}
