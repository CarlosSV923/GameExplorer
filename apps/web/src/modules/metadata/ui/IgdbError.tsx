import { useTranslation } from 'react-i18next'

import { isAppError } from '@/shared/kernel/errors'
import { Button, ClockIcon, ErrorIcon } from '@/shared/ui'

/**
 * IGDB could not answer (States.dc.html): not configured (credentials
 * missing; the upload waits in review) or not responding (retry).
 */
export function IgdbError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const { t } = useTranslation()
  if (isAppError(error, 'igdbUnconfigured')) {
    return (
      <section className="flex flex-col gap-3 rounded-xl border border-line p-6">
        <h3 className="m-0 flex items-center gap-2 text-heading font-bold">
          <ErrorIcon className="text-danger" />
          {t('igdb.unconfiguredTitle')}
        </h3>
        <p className="m-0 text-body text-ink-2">{t('igdb.unconfiguredBody')}</p>
      </section>
    )
  }
  return (
    <section
      role="status"
      className="flex flex-wrap items-center justify-between gap-3 rounded-md bg-surface px-4 py-3 text-body-sm text-ink-2"
    >
      <span className="flex items-center gap-2.5">
        <ClockIcon />
        {t('igdb.unavailable')}
      </span>
      <Button size="sm" onClick={onRetry}>
        {t('common.retry')}
      </Button>
    </section>
  )
}
