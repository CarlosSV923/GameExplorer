import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { fileExtension, singleFile } from '@/modules/catalog/domain/items'
import { gameFolder, kindDraftError, previewFileName } from '@/modules/catalog/domain/naming'
import type { Console, DuplicateAction } from '@/modules/catalog/domain/types'
import { FileTypeFields } from '@/modules/catalog/ui/FileTypeFields'
import { itemLabel } from '@/modules/catalog/ui/itemLabel'
import { describeError } from '@/shared/i18n/errors'
import { useFormat } from '@/shared/i18n/hooks'
import { useAction } from '@/shared/input'
import { baseName } from '@/shared/kernel/text'
import { BackLink } from '@/shared/routing/links'
import {
  BackLabel,
  Banner,
  Button,
  CheckIcon,
  ConfirmDialog,
  cx,
  PageHeader,
  PathPreview,
  TrashIcon,
} from '@/shared/ui'

import { useCancelJob, useCommit, useCommitPlan } from '../application/queries'
import { initialDraft, storedCount, toCommitFile, type FileDraft } from '../domain/details'
import type { CommitRequest, PlannedFile, StagedFile, UploadJob } from '../domain/types'

/** The decision for a file whose final name is taken (RF-09). */
function DuplicateChoice({
  path,
  planned,
  value,
  onChange,
}: {
  path: string
  planned: PlannedFile
  value: DuplicateAction
  onChange: (v: DuplicateAction) => void
}) {
  const { t } = useTranslation()
  const format = useFormat()
  const dup = planned.duplicate
  if (!dup) return null
  return (
    <fieldset className="m-0 flex flex-wrap gap-x-5 gap-y-1 rounded-md border border-accent bg-warning-surface px-3.5 py-3">
      <legend className="px-1.5 text-caption font-bold text-warning-ink">
        {t('details.exists', { name: dup.file, size: format.size(dup.size) })}
      </legend>
      {(['replace', 'skip'] as const).map((v) => (
        <label
          key={v}
          className="inline-flex min-h-control-sm cursor-pointer items-center gap-2 text-body font-semibold"
        >
          <input
            type="radio"
            name={`dup-${path}`}
            checked={value === v}
            onChange={() => {
              onChange(v)
            }}
            className="size-4.5 accent-accent"
          />
          {v === 'replace' ? t('details.replace') : t('details.skip')}
        </label>
      ))}
    </fieldset>
  )
}

/**
 * The confirmation step (RF-08): on Switch each valid file needs its kind
 * (and the update's version or the DLC's name); on Wii and PSP the file and
 * its final name are confirmed. Duplicates ask Replace or Skip (RF-09).
 */
export function FileDetails({
  job,
  files,
  console,
  consoles,
}: {
  job: UploadJob
  files: readonly StagedFile[]
  console: Console
  consoles: readonly Console[]
}) {
  const { t } = useTranslation()
  const format = useFormat()
  const navigate = useNavigate()
  const valid = files.filter((f) => f.valid)
  const discarded = files.filter((f) => !f.valid)
  const kinds = !singleFile(console)
  const folder = gameFolder(job.title)
  const [drafts, setDrafts] = useState<Record<string, FileDraft>>(() =>
    Object.fromEntries(valid.map((f) => [f.path, initialDraft(console, valid.length)])),
  )
  const [tried, setTried] = useState(false)
  const [cancelling, setCancelling] = useState(false)

  const draftOf = (path: string) => drafts[path] ?? initialDraft(console, valid.length)
  const missing = valid.filter((f) => kindDraftError(draftOf(f.path)) !== null).length
  const request: CommitRequest | undefined =
    missing === 0 && valid.length > 0
      ? { files: valid.map((f) => toCommitFile(f.path, draftOf(f.path))) }
      : undefined
  const plan = useCommitPlan(job.id, request)
  const current = request && !plan.isPlaceholderData ? plan.data : undefined
  const undecided = current?.files.some((p) => p.action === 'undecided') ?? false
  const count = storedCount(
    valid.map((f) => f.path),
    current,
  )
  const commit = useCommit(job.id, (r) => {
    void navigate({
      to: '/consolas/$slug/$gameId',
      params: { slug: r.path.split('/')[0] ?? console.slug, gameId: String(r.gameId) },
    })
  })
  const cancel = useCancelJob()

  const save = () => {
    setTried(true)
    if (!request || !current || undecided || count === 0) return
    commit.mutate(request)
  }

  // X saves (docs/design-handoff.md §9).
  useAction('action1', () => {
    save()
    return true
  })

  const set = (path: string, next: FileDraft) => {
    setDrafts((d) => ({ ...d, [path]: next }))
  }
  const existing = current?.existing ?? []

  return (
    <>
      <PageHeader
        back={
          <BackLink to="/subidas">
            <BackLabel>{t('uploads.title')}</BackLabel>
          </BackLink>
        }
        title={kinds ? t('details.title') : t('details.confirmTitle')}
        meta={
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-body-sm font-semibold text-ink-2">
            <span className="font-bold text-ink-1">
              {t('details.gameLine', { title: job.title, console: console.displayName })}
            </span>
            <span className="font-mono text-caption text-ink-1">{job.fileName}</span>
            <span>
              {[
                t('details.valid', { count: valid.length }),
                ...(discarded.length > 0
                  ? [t('details.discardedCount', { count: discarded.length })]
                  : []),
              ].join(' · ')}
            </span>
          </div>
        }
        aside={
          job.igdbId ? (
            <span className="text-body-sm font-bold text-success">{t('game.linked')}</span>
          ) : (
            <span className="text-body-sm text-ink-2">{t('details.ownName')}</span>
          )
        }
      />
      <main className="box-border flex w-full max-w-[1100px] flex-1 flex-col gap-4.5 px-4 pt-7 pb-10 sm:px-6 lg:px-10">
        <p className="m-0 text-body-lg leading-normal text-ink-1">
          {kinds ? t('details.intro') : t('details.introOne', { console: console.displayName })}
        </p>
        {existing.length > 0 && current && (
          <Banner tone="info">
            {t('details.existing', {
              title: current.title,
              items: existing.map((e) => itemLabel(t, e)).join(', '),
            })}
          </Banner>
        )}
        {plan.isError && request && <Banner tone="danger">{describeError(t, plan.error)}</Banner>}

        {valid.map((f) => {
          const draft = draftOf(f.path)
          const planned = current?.files.find((p) => p.path === f.path)
          const error = tried ? kindDraftError(draft) : null
          const preview = previewFileName(job.title, draft, fileExtension(f.path, consoles))
          const path = (
            <PathPreview label={t('details.storedAs')} complete={preview.complete && folder.ok}>
              {`${console.slug}/${folder.ok ? folder.name : '…'}/${preview.name}`}
            </PathPreview>
          )
          return (
            <article
              key={f.path}
              className={cx(
                'flex flex-col gap-3.5 rounded-xl border bg-surface p-4.5',
                error ? 'border-danger' : planned?.duplicate ? 'border-accent' : 'border-line',
              )}
            >
              <div className="flex flex-wrap justify-between gap-2">
                <span className="min-w-0 font-mono text-caption [overflow-wrap:anywhere] text-ink-1">
                  {f.path}
                </span>
                <span className="text-body-sm font-semibold text-ink-2">{format.size(f.size)}</span>
              </div>
              {kinds ? (
                <FileTypeFields
                  name={`kind-${f.path}`}
                  fileName={baseName(f.path)}
                  kinds={console.kinds}
                  value={draft}
                  onChange={(next) => {
                    set(f.path, { ...draft, ...next })
                  }}
                  error={error}
                  preview={path}
                />
              ) : (
                path
              )}
              {planned && (
                <DuplicateChoice
                  path={f.path}
                  planned={planned}
                  value={draft.onDuplicate}
                  onChange={(onDuplicate) => {
                    set(f.path, { ...draft, onDuplicate })
                  }}
                />
              )}
            </article>
          )
        })}

        {discarded.length > 0 && (
          <p className="m-0 flex items-start gap-2 text-body-sm leading-normal font-semibold text-ink-3">
            <TrashIcon size={16} className="mt-0.5 shrink-0" />
            <span>
              {t('details.discarded')}{' '}
              <span className="font-mono text-caption">
                {discarded.map((d) => baseName(d.path)).join(', ')}
              </span>
            </span>
          </p>
        )}

        {commit.isError && <Banner tone="danger">{describeError(t, commit.error)}</Banner>}

        <div className="flex flex-wrap items-center justify-end gap-3 pt-2">
          {tried && missing > 0 && (
            <span role="status" className="mr-auto text-body font-bold text-danger">
              {t('details.missing', { count: missing })}
            </span>
          )}
          <Button
            size="lg"
            onClick={() => {
              setCancelling(true)
            }}
          >
            {t('details.cancelUpload')}
          </Button>
          <Button
            variant="primary"
            size="lg"
            icon={<CheckIcon />}
            loading={commit.isPending}
            disabled={tried && missing === 0 && (!current || count === 0)}
            onClick={save}
          >
            {count === 1 || !kinds ? t('details.saveOne') : t('details.save', { count })}
          </Button>
        </div>
      </main>

      {cancelling && (
        <ConfirmDialog
          tone="danger"
          title={t('details.cancelTitle')}
          confirmLabel={t('details.cancelUpload')}
          busy={cancel.isPending}
          onCancel={() => {
            setCancelling(false)
          }}
          onConfirm={() => {
            cancel.mutate(job.id, {
              onSuccess: () => {
                void navigate({ to: job.fromUnassigned ? '/no-asignados' : '/subidas' })
              },
              onSettled: () => {
                setCancelling(false)
              },
            })
          }}
        >
          <p className="m-0">
            {job.fromUnassigned
              ? t('details.cancelBodyUnassigned', { name: job.fileName })
              : t('details.cancelBody', { name: job.fileName })}
          </p>
        </ConfirmDialog>
      )}
    </>
  )
}
