import { useState } from 'react'
import type { i18n } from 'i18next'
import { I18nextProvider } from 'react-i18next'

import { createI18n } from '@/shared/i18n'
import { InputProvider } from '@/shared/input'

import { UiKit } from './UiKit'

/** Composition root: providers, then the screens (phase 9). */
export function App({ i18n: injected }: { i18n?: i18n }) {
  const [instance] = useState(() => injected ?? createI18n())
  return (
    <I18nextProvider i18n={instance}>
      <InputProvider>
        <UiKit />
      </InputProvider>
    </I18nextProvider>
  )
}
