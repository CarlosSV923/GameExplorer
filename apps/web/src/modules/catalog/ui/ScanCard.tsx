import { useTranslation } from 'react-i18next'

import { describeError } from '@/shared/i18n/errors'
import { useFormat } from '@/shared/i18n/hooks'
import { useNow } from '@/shared/kernel/hooks'
import { Button, RefreshIcon } from '@/shared/ui'

import { useLastScan, useScanLibrary } from '../application/queries'
import type { ScanReport } from '../domain/types'

/** "2 archivos movidos a No asignados · 1 archivo ya no estaba". */
function useScanSummary() {
  const { t } = useTranslation()
  return (r: ScanReport) => {
    const parts = [
      r.unassigned > 0 && t('scan.unassigned', { count: r.unassigned }),
      r.removed > 0 && t('scan.removed', { count: r.removed }),
      r.pending > 0 && t('scan.pending', { count: r.pending }),
    ].filter(Boolean)
    return parts.length > 0 ? parts.join(' · ') : t('scan.nothing')
  }
}

/**
 * The library scan (RF-26): when it last ran, what it changed and "Revisar
 * ahora". Compact, it is one line for the unassigned screen's header.
 */
export function ScanCard({ compact = false }: { compact?: boolean }) {
  const { t } = useTranslation()
  const format = useFormat()
  const now = useNow(30_000)
  const last = useLastScan()
  const scan = useScanLibrary()
  const summary = useScanSummary()
  const report = last.data

  const when = report ? t('scan.last', { when: format.relative(report.scannedAt, now) }) : null
  const button = (
    <Button
      size="sm"
      icon={<RefreshIcon size={16} />}
      loading={scan.isPending}
      onClick={() => {
        scan.mutate()
      }}
    >
      {t('scan.now')}
    </Button>
  )

  if (compact) {
    return (
      <span role="status" className="flex flex-wrap items-center gap-3 text-body-sm text-ink-2">
        {scan.isError ? describeError(t, scan.error) : (when ?? t('scan.never'))}
        {button}
      </span>
    )
  }

  return (
    <section
      aria-labelledby="scan-title"
      className="flex flex-col gap-3 rounded-lg border border-line px-5 py-4.5"
    >
      <h2 id="scan-title" className="m-0 text-body-lg font-bold">
        {t('scan.title')}
      </h2>
      <p className="m-0 text-body-sm leading-normal text-ink-2">{t('scan.body')}</p>
      <div role="status" className="flex flex-col gap-1 text-body-sm">
        {scan.isError ? (
          <span className="text-danger">{describeError(t, scan.error)}</span>
        ) : report ? (
          <>
            <span className="font-bold text-ink-1">{when}</span>
            <span className="text-ink-2">{summary(report)}</span>
          </>
        ) : (
          <span className="text-ink-2">{t('scan.never')}</span>
        )}
      </div>
      <div>{button}</div>
    </section>
  )
}
