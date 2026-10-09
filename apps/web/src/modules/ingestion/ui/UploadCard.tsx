import { useState, type SubmitEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { useConsoles } from '@/modules/catalog/application/queries'
import { describeError } from '@/shared/i18n/errors'
import { useFormat } from '@/shared/i18n/hooks'
import { ButtonLink } from '@/shared/routing/links'
import {
  Button,
  CheckIcon,
  ClockIcon,
  CloseIcon,
  cx,
  ErrorIcon,
  FolderIcon,
  FileButton,
  IconButton,
  LockIcon,
  ProgressBar,
  Spinner,
  TextField,
  TrashIcon,
} from '@/shared/ui'

import { useCancelJob, useSubmitPassword, useUploadQueue } from '../application/queries'
import type { UploadRow } from '../domain/uploads'

function Status({
  icon,
  tone = 'muted',
  children,
}: {
  icon?: ReactNode
  tone?: 'muted' | 'success' | 'accent' | 'danger'
  children: ReactNode
}) {
  return (
    <span
      className={cx(
        'inline-flex items-start gap-1.5 text-caption leading-[1.4] font-bold',
        tone === 'success' && 'text-success',
        tone === 'accent' && 'text-accent',
        tone === 'danger' && 'text-danger',
        tone === 'muted' && 'text-ink-2',
      )}
    >
      {icon && <span className="mt-px shrink-0">{icon}</span>}
      {children}
    </span>
  )
}

function PasswordForm({ jobId, error }: { jobId: string; error: string | null | undefined }) {
  const { t } = useTranslation()
  const submit = useSubmitPassword()
  const [password, setPassword] = useState('')
  const onSubmit = (e: SubmitEvent) => {
    e.preventDefault()
    if (!password) return
    submit.mutate(
      { id: jobId, password },
      {
        onSuccess: () => {
          setPassword('')
        },
      },
    )
  }
  const message = submit.isError ? describeError(t, submit.error) : (error ?? undefined)
  return (
    <form onSubmit={onSubmit} className="flex flex-wrap items-end gap-2">
      <div className="min-w-40 flex-1">
        <TextField
          type="password"
          autoComplete="off"
          label={t('uploads.password')}
          placeholder={t('uploads.passwordHint')}
          value={password}
          onChange={(e) => {
            setPassword(e.target.value)
          }}
          className="bg-surface-input"
          {...(message ? { error: message } : {})}
        />
      </div>
      <Button
        type="submit"
        variant="primary"
        size="sm"
        loading={submit.isPending}
        className="mb-px"
      >
        {t('uploads.unlock')}
      </Button>
    </form>
  )
}

/** One upload in the panel, by phase (DropOverlay.dc.html). */
export function UploadCard({ row }: { row: UploadRow }) {
  const { t } = useTranslation()
  const format = useFormat()
  const queue = useUploadQueue()
  const cancelJob = useCancelJob()
  const consoles = useConsoles()
  const [wrongFile, setWrongFile] = useState(false)
  // Without console the upload goes to the unassigned section (RF-07b).
  const consoleName =
    row.console === ''
      ? t('unassigned.title')
      : (consoles.data?.find((c) => c.slug === row.console)?.displayName ?? row.console)

  const pct = row.size > 0 ? (row.sent / row.size) * 100 : 0
  const amounts = t('uploads.amounts', {
    sent: format.size(row.sent),
    total: format.size(row.size),
  })
  const name = row.fileName

  const cancel = () => {
    if (row.localKey) void queue.cancel(row.localKey)
    else if (row.jobId) cancelJob.mutate(row.jobId)
  }
  const dismiss = () => {
    if (row.jobId) queue.dismiss(row.jobId)
  }

  let corner: ReactNode = null
  let body: ReactNode
  let border = 'border-transparent'

  switch (row.phase) {
    case 'queued':
      corner = <CornerButton label={t('uploads.cancel', { name })} onClick={cancel} />
      body = <Status icon={<ClockIcon size={14} />}>{t('uploads.queued')}</Status>
      break
    case 'uploading':
    case 'uploadingElsewhere':
      corner =
        row.phase === 'uploading' ? (
          <CornerButton label={t('uploads.cancel', { name })} onClick={cancel} />
        ) : null
      body = (
        <>
          <ProgressBar value={pct} label={t('uploads.uploadOf', { name })} />
          <div className="flex justify-between gap-3 text-caption text-ink-2">
            <span>
              {row.phase === 'uploading'
                ? t('uploads.uploading', { amounts })
                : t('uploads.elsewhere', { amounts })}
            </span>
            <span>{t('uploads.resumable')}</span>
          </div>
        </>
      )
      break
    case 'interrupted':
    case 'uploadError':
      border = row.phase === 'uploadError' ? 'border-danger' : 'border-accent'
      corner = <CornerButton label={t('uploads.cancel', { name })} onClick={cancel} />
      body = (
        <>
          <ProgressBar value={pct} label={t('uploads.uploadOf', { name })} />
          <Status
            icon={<ErrorIcon size={14} />}
            tone={row.phase === 'uploadError' ? 'danger' : 'accent'}
          >
            {row.phase === 'uploadError'
              ? t('uploads.networkError', { amounts })
              : t('uploads.interrupted', { amounts })}
          </Status>
          {row.phase === 'uploadError' && row.localKey ? (
            <Button
              size="sm"
              className="self-start"
              onClick={() => {
                if (row.localKey) queue.retry(row.localKey)
              }}
            >
              {t('common.retry')}
            </Button>
          ) : (
            <FileButton
              multiple={false}
              variant="secondary"
              size="sm"
              className="self-start"
              onFiles={([file]) => {
                if (!file || !row.jobId) return
                setWrongFile(
                  !queue.resume(
                    {
                      id: row.jobId,
                      fileName: row.fileName,
                      size: row.size,
                      console: row.console,
                      title: row.title,
                    },
                    file,
                  ),
                )
              }}
            >
              {t('uploads.pickAgain')}
            </FileButton>
          )}
          {wrongFile && (
            <Status tone="danger">
              {t('uploads.wrongFile', { name, size: format.size(row.size) })}
            </Status>
          )}
        </>
      )
      break
    case 'uploaded':
      body = (
        <Status icon={<Spinner label={t('common.loading')} size={14} />}>
          {t('uploads.waitingExtract')}
        </Status>
      )
      break
    case 'waitingParts':
      body = (
        <Status icon={<ClockIcon size={14} />}>
          {t('uploads.waitingParts', { count: row.groupSize ?? 2 })}
        </Status>
      )
      break
    case 'extracting':
      body = (
        <>
          <ProgressBar
            tone="extract"
            value={row.progress ?? 0}
            label={t('uploads.extractOf', { name })}
          />
          <div className="flex justify-between gap-3 text-caption text-ink-2">
            <span>{t('uploads.extracting')}</span>
            <span className="font-bold text-progress-extract">{`${String(row.progress ?? 0)} %`}</span>
          </div>
        </>
      )
      break
    case 'needsPassword':
      border = 'border-accent'
      body = (
        <>
          <Status icon={<LockIcon size={14} />} tone="accent">
            {t('uploads.needsPassword')}
          </Status>
          {row.jobId && <PasswordForm jobId={row.jobId} error={row.error} />}
        </>
      )
      break
    case 'confirm':
      border = 'border-accent'
      corner = row.jobId ? (
        <ButtonLink to="/subidas/$jobId" params={{ jobId: row.jobId }} size="sm" variant="primary">
          {t('uploads.complete')}
        </ButtonLink>
      ) : null
      body = (
        <Status icon={<CheckIcon size={14} />} tone="success">
          {t('uploads.ready')}
        </Status>
      )
      break
    case 'invalid':
      border = 'border-danger'
      corner = row.jobId ? (
        <ButtonLink to="/subidas/$jobId" params={{ jobId: row.jobId }} size="sm" variant="primary">
          {t('uploads.resolve')}
        </ButtonLink>
      ) : null
      body = (
        <Status icon={<ErrorIcon size={14} />} tone="danger">
          {t('uploads.invalid', { console: consoleName })}
        </Status>
      )
      break
    case 'committing':
      body = (
        <Status icon={<Spinner label={t('common.loading')} size={14} />}>
          {t('uploads.committing')}
        </Status>
      )
      break
    case 'done':
      corner = <CornerButton label={t('uploads.dismiss', { name })} onClick={dismiss} />
      body = (
        <Status icon={<CheckIcon size={14} />} tone="success">
          {t('uploads.done')}
        </Status>
      )
      break
    case 'unassigned':
      corner = <CornerButton label={t('uploads.dismiss', { name })} onClick={dismiss} />
      body = <Status icon={<FolderIcon size={14} />}>{t('uploads.unassigned')}</Status>
      break
    case 'trashed':
      corner = <CornerButton label={t('uploads.dismiss', { name })} onClick={dismiss} />
      body = <Status icon={<TrashIcon size={14} />}>{t('uploads.trashed')}</Status>
      break
    case 'failed':
      border = 'border-danger'
      corner = <CornerButton label={t('uploads.dismiss', { name })} onClick={dismiss} />
      body = (
        <Status icon={<ErrorIcon size={14} />} tone="danger">
          {t('uploads.failed', { reason: row.error ?? t('errors.failed') })}
        </Status>
      )
      break
  }

  return (
    <article className={cx('flex flex-col gap-2.5 rounded-lg border bg-surface-card p-4', border)}>
      <div className="flex items-center justify-between gap-3">
        <span className="min-w-0 truncate font-mono text-caption text-ink-1" title={name}>
          {name}
        </span>
        <span className="flex shrink-0 items-center gap-2">
          {(row.phase === 'uploading' || row.phase === 'uploadingElsewhere') && (
            <span className="text-body-sm font-bold">{`${String(Math.floor(pct))} %`}</span>
          )}
          {corner}
        </span>
      </div>
      <span className="-mt-1 truncate text-caption font-bold text-ink-2">
        {t('uploads.target', { title: row.title, console: consoleName })}
      </span>
      {body}
      {row.warning && row.phase !== 'failed' && (
        <span className="text-caption text-ink-3">{row.warning}</span>
      )}
    </article>
  )
}

function CornerButton({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <IconButton label={label} className="-my-3 -mr-2.5 text-ink-2" onClick={onClick}>
      <CloseIcon size={16} />
    </IconButton>
  )
}
