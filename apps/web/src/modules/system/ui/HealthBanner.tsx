import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button, Dialog, ErrorIcon } from '@/shared/ui'

import { useHealth } from '../application/ports'

/**
 * A fixed alert when a startup check fails (States.dc.html): most often the
 * library is not writable because of the dataset ACL.
 */
export function HealthBanner() {
  const { t } = useTranslation()
  const health = useHealth()
  const [open, setOpen] = useState(false)
  const failed = health.data?.checks.filter((c) => !c.ok) ?? []
  if (failed.length === 0) return null

  const library = failed.some((c) => c.name === 'library-writable')
  return (
    <div
      role="alert"
      className="flex flex-wrap items-center justify-between gap-x-6 gap-y-3 border-b border-danger bg-danger-surface px-4 py-3.5 sm:px-6 lg:px-10"
    >
      <span className="flex items-start gap-2.5 text-body leading-[1.45] text-danger-ink">
        <ErrorIcon size={20} className="mt-0.5 shrink-0 text-danger" />
        <span>
          <strong className="text-ink-1">
            {library ? t('health.libraryTitle') : t('health.genericTitle')}
          </strong>{' '}
          {library ? t('health.libraryBody') : t('health.genericBody')}
        </span>
      </span>
      <Button
        variant="danger"
        size="sm"
        className="text-ink-1"
        onClick={() => {
          setOpen(true)
        }}
      >
        {t('health.details')}
      </Button>
      {open && (
        <Dialog
          title={t('health.detailsTitle')}
          onClose={() => {
            setOpen(false)
          }}
          actions={
            <Button
              variant="primary"
              onClick={() => {
                setOpen(false)
              }}
            >
              {t('common.close')}
            </Button>
          }
        >
          <ul className="m-0 flex list-none flex-col gap-3 p-0">
            {failed.map((c) => (
              <li key={c.name} className="flex flex-col gap-1">
                <span className="font-mono text-caption text-ink-3">{c.name}</span>
                <span className="text-body">{c.message ?? t('health.noMessage')}</span>
              </li>
            ))}
          </ul>
        </Dialog>
      )}
    </div>
  )
}
