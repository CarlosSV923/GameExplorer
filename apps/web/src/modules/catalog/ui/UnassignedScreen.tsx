import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useOpenUploadForm } from '@/modules/ingestion/application/queries'
import { UploadsIndicator } from '@/modules/ingestion/ui/UploadButton'
import { describeError } from '@/shared/i18n/errors'
import { useFormat } from '@/shared/i18n/hooks'
import { useAction } from '@/shared/input'
import { useDelayedFlag } from '@/shared/kernel/hooks'
import { BackLink } from '@/shared/routing/links'
import {
  BackLabel,
  Banner,
  Button,
  CloseIcon,
  ConfirmDialog,
  cx,
  DownloadIcon,
  EmptyState,
  FileIcon,
  FolderIcon,
  HelpBar,
  IconButton,
  IconLink,
  PageHeader,
  Skeleton,
  TableBox,
  TrashIcon,
} from '@/shared/ui'

import { useCatalogPorts } from '../application/ports'
import {
  useConsoles,
  useDeleteUnassigned,
  useDeleteUnassignedEntry,
  useTrashUnassigned,
  useTrashUnassignedEntry,
  useUnassigned,
} from '../application/queries'
import type { UnassignedEntry, UnassignedFile } from '../domain/types'
import { ScanCard } from './ScanCard'

/** Whether an entry can be assigned: some file is an archive, or some console takes it. */
function assignable(e: UnassignedEntry): boolean {
  return e.files.some((f) => f.archive || f.consoles.length > 0)
}

/** An entry admits actions once every file settled and no assignment uses it. */
function actionable(e: UnassignedEntry): boolean {
  return e.id !== 0 && !e.copying && !e.busy
}

/** What the section is asked to delete for good: a whole entry or one of its files. */
type Deleting = { kind: 'entry'; entry: UnassignedEntry } | { kind: 'file'; file: UnassignedFile }

function extensionOfName(name: string): string {
  const dot = name.lastIndexOf('.')
  return dot > 0 ? name.slice(dot).toLowerCase() : name
}

/**
 * The unassigned section (RF-27): files that arrived over Samba, uploads
 * without console or that did not fit, and what the user moved here. A
 * first-level folder is one entry, assigned at once (RF-27a); files still
 * being copied show up as such, without actions (RF-26a).
 */
export function UnassignedScreen() {
  const { t } = useTranslation()
  const format = useFormat()
  const navigate = useNavigate()
  const { unassigned: ports } = useCatalogPorts()
  const files = useUnassigned()
  const consoles = useConsoles()
  const slow = useDelayedFlag(files.isPending)
  const openForm = useOpenUploadForm()
  const trash = useTrashUnassigned()
  const trashEntry = useTrashUnassignedEntry()
  const remove = useDeleteUnassigned()
  const removeEntry = useDeleteUnassignedEntry()
  const [deleting, setDeleting] = useState<Deleting | null>(null)
  const [notice, setNotice] = useState<{ tone: 'info' | 'danger'; text: string } | null>(null)

  const list = files.data ?? []
  const names = new Map((consoles.data ?? []).map((c) => [c.slug, c.displayName]))
  const total = list.reduce((s, e) => s + e.size, 0)

  const sendToTrash = (kind: 'entry' | 'file', id: number, name: string) => {
    setNotice(null)
    const mutation = kind === 'entry' ? trashEntry : trash
    mutation.mutate(id, {
      onSuccess: () => {
        setNotice({ tone: 'info', text: t('unassigned.trashed', { name }) })
      },
      onError: (e) => {
        setNotice({ tone: 'danger', text: describeError(t, e) })
      },
    })
  }

  useAction('back', () => {
    void navigate({ to: '/' })
    return true
  })

  const fit = (f: UnassignedFile): { text: string; tone: 'ok' | 'muted' | 'bad' } => {
    if (f.archive) return { text: t('unassigned.archive'), tone: 'muted' }
    if (f.consoles.length > 0) {
      return {
        text: t('unassigned.fits', {
          names: format.orList(f.consoles.map((c) => names.get(c) ?? c)),
        }),
        tone: 'ok',
      }
    }
    return { text: t('unassigned.fitsNone', { ext: extensionOfName(f.name) }), tone: 'bad' }
  }

  let content
  if (files.isPending) {
    content = slow ? (
      <div aria-hidden="true" className="flex flex-col gap-3">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-16 w-full" />
        ))}
      </div>
    ) : null
  } else if (files.isError) {
    content = <EmptyState title={t('errors.loadTitle')} body={t('errors.loadBody')} />
  } else if (list.length === 0) {
    content = (
      <EmptyState
        icon={<FolderIcon size={40} strokeWidth={1.6} />}
        title={t('unassigned.emptyTitle')}
        body={t('unassigned.emptyBody')}
      />
    )
  } else {
    content = (
      <>
        <TableBox>
          <table className="w-full min-w-[900px] border-collapse">
            <thead>
              <tr className="text-left text-caption font-bold tracking-[0.06em] text-ink-3 uppercase">
                <th className="px-5 py-3.5">{t('unassigned.file')}</th>
                <th className="px-5 py-3.5">{t('unassigned.arrived')}</th>
                <th className="px-5 py-3.5 text-right">{t('game.size')}</th>
                <th className="px-5 py-3.5">
                  <span className="sr-only">{t('game.actions')}</span>
                </th>
              </tr>
            </thead>
            <tbody className="text-body">
              {list.map((e) => {
                const ready = actionable(e)
                const loose = !e.folder ? e.files[0] : undefined
                const fits = loose && !loose.copying ? fit(loose) : undefined
                let state: string
                if (e.copying) state = t('unassigned.copying')
                else if (e.busy) state = t('unassigned.busy')
                else if (e.folder) state = t('unassigned.files', { count: e.files.length })
                else state = t(`unassigned.reason.${e.files[0]?.reason ?? 'samba'}`)
                return [
                  <tr key={`e${e.name}`} className="border-t border-line">
                    <td className="px-5 py-3.5">
                      <div className="flex flex-col gap-1">
                        <span className="flex items-center gap-2 font-mono text-caption font-medium [overflow-wrap:anywhere] text-ink-1">
                          {e.folder ? (
                            <FolderIcon size={16} className="shrink-0 text-ink-2" />
                          ) : (
                            <FileIcon size={16} className="shrink-0 text-ink-2" />
                          )}
                          {e.name}
                        </span>
                        {e.console && (
                          <span className="text-caption text-ink-3">
                            {t('unassigned.cameFrom', { name: names.get(e.console) ?? e.console })}
                          </span>
                        )}
                        {loose && !loose.copying && (
                          <span className="text-caption text-ink-3">
                            {t('unassigned.origin')}{' '}
                            <span className="font-mono [overflow-wrap:anywhere]">
                              {loose.origin}
                            </span>
                          </span>
                        )}
                        {fits && <FitNote text={fits.text} tone={fits.tone} />}
                      </div>
                    </td>
                    <td className="px-5 py-3.5 whitespace-nowrap">
                      <div className="flex flex-col gap-1">
                        <span className="text-ink-1">{format.date(e.arrivedAt)}</span>
                        <span
                          className={cx(
                            'text-caption',
                            e.copying || e.busy ? 'font-bold text-warning-ink' : 'text-ink-2',
                          )}
                        >
                          {state}
                        </span>
                      </div>
                    </td>
                    <td className="px-5 py-3.5 text-right whitespace-nowrap text-ink-2">
                      {format.size(e.size)}
                    </td>
                    <td className="px-4 py-2 text-right whitespace-nowrap">
                      <span className="inline-flex items-center gap-1">
                        <Button
                          size="sm"
                          variant="primary"
                          disabled={!ready || !assignable(e)}
                          aria-label={
                            assignable(e)
                              ? t('unassigned.assignFile', { name: e.name })
                              : t('unassigned.assignBlocked', { name: e.name })
                          }
                          onClick={() => {
                            openForm({ kind: 'unassigned', entry: e })
                          }}
                        >
                          {t('unassigned.assign')}
                        </Button>
                        {ready ? (
                          <IconLink
                            href={ports.entryDownloadUrl(e.id)}
                            download=""
                            label={t('unassigned.download', { name: e.name })}
                          >
                            <DownloadIcon />
                          </IconLink>
                        ) : (
                          <IconButton disabled label={t('unassigned.download', { name: e.name })}>
                            <DownloadIcon />
                          </IconButton>
                        )}
                        <IconButton
                          disabled={!ready}
                          label={t('unassigned.trash', { name: e.name })}
                          onClick={() => {
                            sendToTrash('entry', e.id, e.name)
                          }}
                        >
                          <TrashIcon />
                        </IconButton>
                        <IconButton
                          tone="danger"
                          disabled={!ready}
                          label={t('unassigned.delete', { name: e.name })}
                          onClick={() => {
                            setDeleting({ kind: 'entry', entry: e })
                          }}
                        >
                          <CloseIcon />
                        </IconButton>
                      </span>
                    </td>
                  </tr>,
                  ...(e.folder
                    ? e.files.map((f) => {
                        const fileReady = !f.copying && !e.busy && f.id !== 0
                        const note = f.copying ? undefined : fit(f)
                        const rel = f.path.slice(e.name.length + 1)
                        return (
                          <tr key={`f${f.path}`} className="bg-surface-card/40">
                            <td className="py-2 pr-5 pl-12">
                              <div className="flex flex-col gap-0.5">
                                <span className="font-mono text-caption [overflow-wrap:anywhere] text-ink-1">
                                  {rel}
                                </span>
                                {f.copying ? (
                                  <span className="text-caption font-bold text-warning-ink">
                                    {t('unassigned.copying')}
                                  </span>
                                ) : (
                                  note && <FitNote text={note.text} tone={note.tone} />
                                )}
                              </div>
                            </td>
                            <td className="px-5 py-2" />
                            <td className="px-5 py-2 text-right text-caption whitespace-nowrap text-ink-2">
                              {format.size(f.size)}
                            </td>
                            <td className="px-4 py-1.5 text-right whitespace-nowrap">
                              <span className="inline-flex items-center gap-1">
                                {fileReady ? (
                                  <IconLink
                                    href={ports.downloadUrl(f.id)}
                                    download=""
                                    label={t('game.downloadItem', { name: f.name })}
                                  >
                                    <DownloadIcon />
                                  </IconLink>
                                ) : (
                                  <IconButton
                                    disabled
                                    label={t('game.downloadItem', { name: f.name })}
                                  >
                                    <DownloadIcon />
                                  </IconButton>
                                )}
                                <IconButton
                                  disabled={!fileReady}
                                  label={t('unassigned.trash', { name: f.name })}
                                  onClick={() => {
                                    sendToTrash('file', f.id, f.name)
                                  }}
                                >
                                  <TrashIcon />
                                </IconButton>
                                <IconButton
                                  tone="danger"
                                  disabled={!fileReady}
                                  label={t('unassigned.delete', { name: f.name })}
                                  onClick={() => {
                                    setDeleting({ kind: 'file', file: f })
                                  }}
                                >
                                  <CloseIcon />
                                </IconButton>
                              </span>
                            </td>
                          </tr>
                        )
                      })
                    : []),
                ]
              })}
            </tbody>
          </table>
        </TableBox>
        <p className="m-0 text-body-sm leading-normal text-ink-3">{t('unassigned.footnote')}</p>
      </>
    )
  }

  return (
    <div className="flex min-h-dvh flex-col">
      <PageHeader
        back={
          <BackLink to="/">
            <BackLabel>{t('nav.consoles')}</BackLabel>
          </BackLink>
        }
        title={t('unassigned.title')}
        meta={
          <span className="max-w-[72ch] text-body-sm leading-normal text-ink-2">
            {t('unassigned.intro')}
          </span>
        }
        aside={
          <div className="flex flex-col items-end gap-2">
            <span className="flex items-center gap-4">
              <UploadsIndicator />
              {files.data && (
                <span className="text-count font-medium">
                  {t('unassigned.summary', { count: list.length, size: format.size(total) })}
                </span>
              )}
            </span>
            <ScanCard compact />
          </div>
        }
      />
      <main className="box-border flex flex-1 flex-col gap-5 px-4 pt-7 pb-10 sm:px-6 lg:px-10">
        {notice && <Banner tone={notice.tone}>{notice.text}</Banner>}
        {content}
      </main>
      <HelpBar
        actions={[
          { glyph: 'A', label: t('help.select') },
          { glyph: 'B', label: t('help.back') },
          { glyph: 'DPAD', label: t('help.choose') },
        ]}
      />

      {deleting && (
        <ConfirmDialog
          tone="danger"
          title={t('unassigned.deleteTitle')}
          confirmLabel={t('unassigned.deleteConfirm')}
          busy={remove.isPending || removeEntry.isPending}
          onCancel={() => {
            setDeleting(null)
          }}
          onConfirm={() => {
            const options = {
              onSettled: () => {
                setDeleting(null)
              },
              onError: (e: unknown) => {
                setNotice({ tone: 'danger', text: describeError(t, e) })
              },
            }
            if (deleting.kind === 'entry') removeEntry.mutate(deleting.entry.id, options)
            else remove.mutate(deleting.file.id, options)
          }}
        >
          <p className="m-0">
            {deleting.kind === 'entry'
              ? t('unassigned.deleteBody', {
                  name: deleting.entry.name,
                  size: format.size(deleting.entry.size),
                })
              : t('unassigned.deleteBody', {
                  name: deleting.file.name,
                  size: format.size(deleting.file.size),
                })}
          </p>
        </ConfirmDialog>
      )}
    </div>
  )
}

/** Which consoles take a file, under its name. */
function FitNote({ text, tone }: { text: string; tone: 'ok' | 'muted' | 'bad' }) {
  return (
    <span
      className={cx(
        'text-caption font-bold',
        tone === 'ok' && 'text-success',
        tone === 'muted' && 'text-ink-2',
        tone === 'bad' && 'text-danger',
      )}
    >
      {text}
    </span>
  )
}
