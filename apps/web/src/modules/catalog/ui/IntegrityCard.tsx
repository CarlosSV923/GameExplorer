import { useTranslation } from 'react-i18next'

import { describeError } from '@/shared/i18n/errors'
import { useFormat } from '@/shared/i18n/hooks'
import { useNow } from '@/shared/kernel/hooks'
import { Button, RematchIcon } from '@/shared/ui'

import { useCheckLibrary, useLastCheck } from '../application/queries'

/** Settings › General: the hourly integrity check, and "check now" (RF-26). */
export function IntegrityCard() {
  const { t } = useTranslation()
  const format = useFormat()
  const now = useNow(30_000)
  const last = useLastCheck()
  const check = useCheckLibrary()
  const report = last.data

  let status: string
  if (check.isError) status = describeError(t, check.error)
  else if (!report) status = t('integrity.never')
  else {
    const when = format.relative(report.checkedAt, now)
    status =
      report.missingTotal > 0
        ? t('integrity.missing', { when, count: report.missingTotal })
        : t('integrity.ok', { when })
  }

  return (
    <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-4 rounded-lg border border-line px-5 py-4.5">
      <span className="flex flex-col gap-1">
        <span className="text-body-lg font-bold">{t('integrity.title')}</span>
        <span role="status" className="text-body-sm text-ink-2">
          {status}
        </span>
      </span>
      <Button
        size="sm"
        icon={<RematchIcon size={16} />}
        loading={check.isPending}
        onClick={() => {
          check.mutate()
        }}
      >
        {t('integrity.checkNow')}
      </Button>
    </div>
  )
}
