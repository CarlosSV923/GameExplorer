import { useNavigate } from '@tanstack/react-router'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { useConsoles } from '@/modules/catalog/application/queries'
import { useAction } from '@/shared/input'
import { isAppError } from '@/shared/kernel/errors'
import { useDelayedFlag } from '@/shared/kernel/hooks'
import { BackLink, ButtonLink } from '@/shared/routing/links'
import { BackLabel, EmptyState, HelpBar, PageHeader, Spinner } from '@/shared/ui'

import { useJob, useJobFiles } from '../application/queries'
import type { JobStatus } from '../domain/types'
import { FileDetails } from './FileDetails'
import { ValidationError } from './ValidationError'

const stateKeys: Partial<Record<JobStatus, 'done' | 'failed' | 'unassigned' | 'trashed'>> = {
  done: 'done',
  failed: 'failed',
  cancelled: 'failed',
  unassigned: 'unassigned',
  trashed: 'trashed',
}

/**
 * One upload that needs the user (`/subidas/:id`): the file data and
 * confirmation (RF-08), or the validation error (RF-07). Other states point
 * back to the uploads.
 */
export function JobScreen({ jobId }: { jobId: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const job = useJob(jobId)
  const consoles = useConsoles()
  const status = job.data?.status
  const ready = status === 'confirm' || status === 'committing' || status === 'invalid'
  const files = useJobFiles(jobId, ready)
  const slow = useDelayedFlag(job.isPending || consoles.isPending || files.isPending)

  useAction('back', () => {
    void navigate({ to: '/subidas' })
    return true
  })

  const all = consoles.data ?? []
  const console = all.find((c) => c.slug === job.data?.console)

  let content: ReactNode
  if (job.isError) {
    content = (
      <EmptyState
        title={isAppError(job.error, 'notFound') ? t('job.goneTitle') : t('errors.loadTitle')}
        body={t('job.goneBody')}
        action={
          <ButtonLink to="/subidas" variant="primary">
            {t('uploads.title')}
          </ButtonLink>
        }
      />
    )
  } else if (!job.data || !consoles.data) {
    content = slow ? <Spinner label={t('common.loading')} /> : null
  } else if (!ready) {
    const key = (status && stateKeys[status]) ?? 'busy'
    content = (
      <EmptyState
        title={t(`job.state.${key}`)}
        body={key === 'failed' ? (job.data.error ?? t('errors.failed')) : t('job.state.body')}
        action={
          <ButtonLink to="/subidas" variant="primary">
            {t('uploads.title')}
          </ButtonLink>
        }
      />
    )
  } else if (!files.data) {
    content = slow ? <Spinner label={t('common.loading')} /> : null
  } else {
    // A console change re-validates the same files: start the form again.
    const view =
      status === 'invalid' || !console ? (
        <ValidationError job={job.data} files={files.data} console={console} consoles={all} />
      ) : (
        <FileDetails
          key={job.data.console}
          job={job.data}
          files={files.data}
          console={console}
          consoles={all}
        />
      )
    return (
      <div className="flex min-h-app flex-col">
        {view}
        <HelpBar
          actions={[
            { glyph: 'A', label: t('help.select') },
            { glyph: 'B', label: t('help.back') },
            ...(status === 'invalid' ? [] : [{ glyph: 'X' as const, label: t('help.save') }]),
            { glyph: 'DPAD', label: t('help.choose') },
          ]}
        />
      </div>
    )
  }

  return (
    <div className="flex min-h-app flex-col">
      <PageHeader
        back={
          <BackLink to="/subidas">
            <BackLabel>{t('uploads.title')}</BackLabel>
          </BackLink>
        }
        title={t('job.title')}
      />
      <main className="box-border flex-1 px-4 py-8 sm:px-6 lg:px-10">{content}</main>
    </div>
  )
}
