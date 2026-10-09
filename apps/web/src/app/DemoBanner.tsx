import { useLayoutEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

/**
 * The fixed notice of the demo (RF-65): no server, files never leave the
 * browser, and reloading starts over. Screens leave room for it
 * (--chrome-top, the min-h-app utility).
 */
export function DemoBanner() {
  const { t } = useTranslation()
  const box = useRef<HTMLDivElement>(null)

  useLayoutEffect(() => {
    const el = box.current
    if (!el) return
    const root = document.documentElement
    const update = () => {
      root.style.setProperty('--chrome-top', `${String(el.offsetHeight)}px`)
    }
    update()
    const observer = new ResizeObserver(update)
    observer.observe(el)
    return () => {
      observer.disconnect()
      root.style.removeProperty('--chrome-top')
    }
  }, [])

  return (
    <div
      ref={box}
      role="note"
      className="sticky top-0 z-30 border-b border-accent bg-warning-surface px-4 py-2 text-center text-body-sm leading-snug font-semibold text-warning-ink sm:px-6 lg:px-10"
    >
      <strong className="text-ink-1">{t('demo.title')}</strong> {t('demo.body')}
    </div>
  )
}
