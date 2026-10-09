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
  useTrashUnassigned,
  useUnassigned,
} from '../application/queries'
import type { UnassignedFile } from '../domain/types'
import { ScanCard } from './ScanCard'

/** Whether the file can be assigned: an archive, or some console takes its extension. */
function assignable(f: UnassignedFile): boolean {
  return f.archive || f.consoles.length > 0
}

function extensionOfName(name: string): string {
  const dot = name.lastIndexOf('.')
  return dot > 0 ? name.slice(dot).toLowerCase() : name
}

/**
 * The unassigned section (RF-27): files that arrived over Samba outside a
 * game folder, uploads that did not fit, and what the user moved here.
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
  const remove = useDeleteUnassigned()
  const [deleting, setDeleting] = useState<UnassignedFile | null>(null)
  const [notice, setNotice] = useState<{ tone: 'info' | 'danger'; text: string } | null>(null)

  const list = files.data ?? []
  const names = new Map((consoles.data ?? []).map((c) => [c.slug, c.displayName]))
  const total = list.reduce((s, f) => s + f.size, 0)

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
              {list.map((f) => {
                const fits = fit(f)
                return (
                  <tr key={f.id} className="border-t border-line">
                    <td className="px-5 py-3.5">
                      <div className="flex flex-col gap-1">
                        <span className="font-mono text-caption font-medium [overflow-wrap:anywhere] text-ink-1">
                          {f.name}
                        </span>
                        <span className="text-caption text-ink-3">
                          {t('unassigned.origin')}{' '}
                          <span className="font-mono [overflow-wrap:anywhere]">{f.origin}</span>
                        </span>
                        <span
                          className={cx(
                            'text-caption font-bold',
                            fits.tone === 'ok' && 'text-success',
                            fits.tone === 'muted' && 'text-ink-2',
                            fits.tone === 'bad' && 'text-danger',
                          )}
                        >
                          {fits.text}
                        </span>
                      </div>
                    </td>
                    <td className="px-5 py-3.5 whitespace-nowrap">
                      <div className="flex flex-col gap-1">
                        <span className="text-ink-1">{format.date(f.arrivedAt)}</span>
                        <span className="text-caption text-ink-2">
                          {t(`unassigned.reason.${f.reason}`)}
                        </span>
                      </div>
                    </td>
                    <td className="px-5 py-3.5 text-right whitespace-nowrap text-ink-2">
                      {format.size(f.size)}
                    </td>
                    <td className="px-4 py-2 text-right whitespace-nowrap">
                      <span className="inline-flex items-center gap-1">
                        <Button
                          size="sm"
                          variant="primary"
                          disabled={!assignable(f)}
                          aria-label={
                            assignable(f)
                              ? t('unassigned.assignFile', { name: f.name })
                              : t('unassigned.assignBlocked', { name: f.name })
                          }
                          onClick={() => {
                            openForm({ kind: 'unassigned', file: f })
                          }}
                        >
                          {t('unassigned.assign')}
                        </Button>
                        <IconLink
                          href={ports.downloadUrl(f.id)}
                          download=""
                          label={t('game.downloadItem', { name: f.name })}
                        >
                          <DownloadIcon />
                        </IconLink>
                        <IconButton
                          label={t('unassigned.trash', { name: f.name })}
                          onClick={() => {
                            setNotice(null)
                            trash.mutate(f.id, {
                              onSuccess: () => {
                                setNotice({
                                  tone: 'info',
                                  text: t('unassigned.trashed', { name: f.name }),
                                })
                              },
                              onError: (e) => {
                                setNotice({ tone: 'danger', text: describeError(t, e) })
                              },
                            })
                          }}
                        >
                          <TrashIcon />
                        </IconButton>
                        <IconButton
                          tone="danger"
                          label={t('unassigned.delete', { name: f.name })}
                          onClick={() => {
                            setDeleting(f)
                          }}
                        >
                          <CloseIcon />
                        </IconButton>
                      </span>
                    </td>
                  </tr>
                )
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
          busy={remove.isPending}
          onCancel={() => {
            setDeleting(null)
          }}
          onConfirm={() => {
            remove.mutate(deleting.id, {
              onSettled: () => {
                setDeleting(null)
              },
              onError: (e) => {
                setNotice({ tone: 'danger', text: describeError(t, e) })
              },
            })
          }}
        >
          <p className="m-0">
            {t('unassigned.deleteBody', { name: deleting.name, size: format.size(deleting.size) })}
          </p>
        </ConfirmDialog>
      )}
    </div>
  )
}
