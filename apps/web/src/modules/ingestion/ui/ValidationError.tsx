import { useNavigate } from '@tanstack/react-router'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { consoleExtensions, fileExtension } from '@/modules/catalog/domain/items'
import { gameFolder, itemFileName } from '@/modules/catalog/domain/naming'
import type { Console } from '@/modules/catalog/domain/types'
import { describeError } from '@/shared/i18n/errors'
import { useFormat } from '@/shared/i18n/hooks'
import { baseName } from '@/shared/kernel/text'
import { BackLink } from '@/shared/routing/links'
import {
  ArrowRightIcon,
  BackLabel,
  Banner,
  Button,
  CloseIcon,
  ConfirmDialog,
  cx,
  ErrorIcon,
  FolderIcon,
  OptionCards,
  PageHeader,
  TrashIcon,
} from '@/shared/ui'

import { useChangeConsole, useResolveJob } from '../application/queries'
import type { ResolveAction, StagedFile, UploadJob } from '../domain/types'

/** Why a console can or cannot take the upload's files. */
type Fit =
  | { kind: 'fits'; file: string; path: string }
  | { kind: 'fitsMany'; count: number }
  | { kind: 'onlyOne' }
  | { kind: 'rejects'; exts: string }

/** Where the files of the upload could go instead, and why not elsewhere. */
function targetsFor(
  job: UploadJob,
  files: readonly StagedFile[],
  consoles: readonly Console[],
): { console: Console; fit: Fit }[] {
  const exts = [...new Set(files.map((f) => fileExtension(f.path, consoles)).filter(Boolean))]
  return consoles
    .filter((c) => c.slug !== job.console)
    .map((c) => {
      const fit = files.filter((f) => f.consoles.includes(c.slug))
      const [only] = fit
      if (!only) return { console: c, fit: { kind: 'rejects', exts: exts.join(' ') } }
      if (fit.length > 1) {
        return {
          console: c,
          fit: c.multipleFiles ? { kind: 'fitsMany', count: fit.length } : { kind: 'onlyOne' },
        }
      }
      const folder = gameFolder(job.title)
      const name = itemFileName(
        { title: job.title, kind: c.kinds.includes('game') ? 'game' : 'base' },
        fileExtension(only.path, consoles),
      )
      const path = folder.ok && name.ok ? `${c.slug}/${folder.name}/${name.name}` : `${c.slug}/…`
      return { console: c, fit: { kind: 'fits', file: baseName(only.path), path } }
    })
}

const usable = (fit: Fit) => fit.kind === 'fits' || fit.kind === 'fitsMany'

function ActionCard({
  icon,
  title,
  body,
  danger = false,
  onClick,
  busy,
}: {
  icon: ReactNode
  title: string
  body: string
  danger?: boolean
  onClick: () => void
  busy: boolean
}) {
  return (
    <button
      type="button"
      disabled={busy}
      onClick={onClick}
      className={cx(
        'flex min-h-24 cursor-pointer flex-col items-start gap-1.5 rounded-xl border bg-transparent px-4.5 py-4 text-left disabled:cursor-default disabled:opacity-45',
        danger ? 'border-danger text-danger' : 'border-control text-ink-1 hover:bg-hover',
      )}
    >
      <span className="inline-flex items-center gap-2 text-body-lg font-bold">
        {icon}
        {title}
      </span>
      <span
        className={cx(
          'text-caption leading-snug font-semibold',
          danger ? 'text-danger-ink' : 'text-ink-2',
        )}
      >
        {body}
      </span>
    </button>
  )
}

/**
 * An upload whose files do not fit its console (RF-07): change console
 * without extracting again (first, it keeps the work), or set the files
 * aside: unassigned, trash or delete for good.
 */
export function ValidationError({
  job,
  files,
  console,
  consoles,
}: {
  job: UploadJob
  files: readonly StagedFile[]
  console: Console | undefined
  consoles: readonly Console[]
}) {
  const { t } = useTranslation()
  const format = useFormat()
  const navigate = useNavigate()
  const change = useChangeConsole(job.id)
  const resolve = useResolveJob(job.id)
  const [deleting, setDeleting] = useState(false)
  const targets = targetsFor(job, files, consoles)
  const [target, setTarget] = useState(() => targets.find((x) => usable(x.fit))?.console.slug)
  const names = new Map(consoles.map((c) => [c.slug, c.displayName]))
  const consoleName = console?.displayName ?? job.console
  const found = files.filter((f) => f.consoles.length > 0)
  const discarded = files.filter((f) => f.consoles.length === 0)
  const total = files.reduce((s, f) => s + f.size, 0)
  const many = job.invalidReason === 'many'
  const chosen = targets.find((x) => x.console.slug === target && usable(x.fit))

  const setAside = (action: ResolveAction) => {
    resolve.mutate(action, {
      onSuccess: () => {
        void navigate({ to: action === 'unassigned' ? '/no-asignados' : '/subidas' })
      },
      onSettled: () => {
        setDeleting(false)
      },
    })
  }

  const note = (fit: Fit) => {
    switch (fit.kind) {
      case 'fits':
        return t('invalid.fits', { file: fit.file, path: fit.path })
      case 'fitsMany':
        return t('invalid.fitsMany', { count: fit.count })
      case 'onlyOne':
        return t('invalid.onlyOne')
      case 'rejects':
        return fit.exts ? t('invalid.rejects', { exts: fit.exts }) : t('invalid.rejectsAll')
    }
  }

  let reason: string
  if (many) {
    reason = t('invalid.many', { console: consoleName })
  } else {
    const exts = console ? format.orList(consoleExtensions(console)) : ''
    reason = found.length > 0 ? t('invalid.noneOther', { exts }) : t('invalid.none', { exts })
  }

  return (
    <>
      <PageHeader
        back={
          <BackLink to="/subidas">
            <BackLabel>{t('uploads.title')}</BackLabel>
          </BackLink>
        }
        title={many ? t('invalid.manyTitle') : t('invalid.noneTitle', { console: consoleName })}
        meta={
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-body-sm font-semibold text-ink-2">
            <span className="font-bold text-ink-1">
              {t('details.gameLine', { title: job.title, console: consoleName })}
            </span>
            <span className="font-mono text-caption text-ink-1">{job.fileName}</span>
            <span>{t('invalid.nothingSaved')}</span>
          </div>
        }
      />
      <main className="box-border flex flex-1 flex-wrap items-start gap-8 px-4 pt-7 pb-10 sm:px-6 lg:px-10">
        <section className="flex min-w-0 flex-[1_1_380px] flex-col gap-4 lg:max-w-[520px]">
          <div
            role="alert"
            className="flex items-start gap-3 rounded-lg border border-danger bg-danger-surface px-4.5 py-4 text-body leading-normal font-semibold text-danger-ink"
          >
            <ErrorIcon size={20} className="mt-0.5 shrink-0 text-danger" />
            <span>{reason}</span>
          </div>
          <h2 className="m-0 mt-2 text-body-sm font-bold tracking-label text-ink-2 uppercase">
            {t('invalid.found')}
          </h2>
          {found.length === 0 ? (
            <p className="m-0 text-body-sm text-ink-3">{t('invalid.foundNone')}</p>
          ) : (
            <ul className="m-0 flex list-none flex-col gap-2 p-0">
              {found.map((f) => (
                <li
                  key={f.path}
                  className="flex flex-col gap-1 rounded-lg border border-line bg-surface px-4 py-3"
                >
                  <span className="flex justify-between gap-3">
                    <span className="font-mono text-caption [overflow-wrap:anywhere] text-ink-1">
                      {f.path}
                    </span>
                    <span className="shrink-0 text-caption font-semibold text-ink-2">
                      {format.size(f.size)}
                    </span>
                  </span>
                  <span className="text-caption font-semibold text-ink-3">
                    {t('invalid.fitsFor', {
                      names: format.orList(f.consoles.map((c) => names.get(c) ?? c)),
                    })}
                  </span>
                </li>
              ))}
            </ul>
          )}
          {discarded.length > 0 && (
            <span className="text-body-sm font-semibold text-ink-3">
              {t('invalid.discarded')}{' '}
              <span className="font-mono text-caption">
                {discarded.map((d) => baseName(d.path)).join(', ')}
              </span>
            </span>
          )}
        </section>

        <section
          aria-labelledby="what-now"
          className="flex min-w-0 flex-[999_1_520px] flex-col gap-4"
        >
          <h2
            id="what-now"
            className="m-0 text-body-sm font-bold tracking-label text-ink-2 uppercase"
          >
            {t('invalid.whatNow')}
          </h2>
          <article
            className={cx(
              'flex flex-col gap-3.5 rounded-xl border bg-surface p-5',
              chosen ? 'border-accent' : 'border-line',
            )}
          >
            <div className="flex flex-col gap-1">
              <h3 className="m-0 text-heading font-bold">{t('invalid.change')}</h3>
              <span className="text-body-sm leading-snug font-semibold text-ink-2">
                {t('invalid.changeBody')}
              </span>
            </div>
            <OptionCards
              legend={t('invalid.target')}
              hideLegend
              name="target-console"
              appearance="row"
              value={target}
              onChange={setTarget}
              options={targets.map((x) => ({
                value: x.console.slug,
                title: x.console.displayName,
                disabled: !usable(x.fit),
                detail: note(x.fit),
              }))}
            />
            {chosen && (
              <Button
                variant="primary"
                className="self-start"
                icon={<ArrowRightIcon />}
                loading={change.isPending}
                onClick={() => {
                  change.mutate(chosen.console.slug)
                }}
              >
                {t('invalid.moveTo', { name: chosen.console.displayName })}
              </Button>
            )}
            {change.isError && <Banner tone="danger">{describeError(t, change.error)}</Banner>}
          </article>
          {job.fromUnassigned ? (
            // The entry never left the section: setting it aside is ending the assignment (RF-27a).
            <div className="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-3">
              <ActionCard
                icon={<FolderIcon />}
                title={t('invalid.release')}
                body={t('invalid.releaseBody')}
                busy={resolve.isPending}
                onClick={() => {
                  setAside('unassigned')
                }}
              />
            </div>
          ) : (
            <div className="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-3">
              <ActionCard
                icon={<FolderIcon />}
                title={t('invalid.unassigned')}
                body={t('invalid.unassignedBody')}
                busy={resolve.isPending}
                onClick={() => {
                  setAside('unassigned')
                }}
              />
              <ActionCard
                icon={<TrashIcon />}
                title={t('invalid.trash')}
                body={t('invalid.trashBody')}
                busy={resolve.isPending}
                onClick={() => {
                  setAside('trash')
                }}
              />
              <ActionCard
                danger
                icon={<CloseIcon />}
                title={t('invalid.delete')}
                body={t('invalid.deleteBody')}
                busy={resolve.isPending}
                onClick={() => {
                  setDeleting(true)
                }}
              />
            </div>
          )}
          {resolve.isError && <Banner tone="danger">{describeError(t, resolve.error)}</Banner>}
        </section>
      </main>

      {deleting && (
        <ConfirmDialog
          tone="danger"
          title={t('invalid.deleteTitle')}
          confirmLabel={t('invalid.delete')}
          busy={resolve.isPending}
          onCancel={() => {
            setDeleting(false)
          }}
          onConfirm={() => {
            setAside('delete')
          }}
        >
          <p className="m-0">
            {t('invalid.deleteConfirm', { name: job.fileName, size: format.size(total) })}
          </p>
        </ConfirmDialog>
      )}
    </>
  )
}
