import type { TFunction } from 'i18next'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { describeError } from '@/shared/i18n/errors'
import { useFormat } from '@/shared/i18n/hooks'
import { isAppError } from '@/shared/kernel/errors'
import { useDelayedFlag, useNow } from '@/shared/kernel/hooks'
import {
  Banner,
  Button,
  Chip,
  ConfirmDialog,
  cx,
  Dialog,
  EmptyState,
  IconButton,
  RestoreIcon,
  Skeleton,
  TableBox,
  TrashIcon,
} from '@/shared/ui'

import {
  useConsoles,
  useDeleteTrashEntry,
  useEmptyTrash,
  useRestore,
  useTrash,
} from '../application/queries'
import type { TrashEntry } from '../domain/types'
import { ItemChip } from './ItemChip'

/** Retention, read from an entry (TRASH_RETENTION_DAYS is the server's). */
function retentionDays(entries: readonly TrashEntry[]): number | undefined {
  const e = entries[0]
  if (!e) return undefined
  return Math.round((Date.parse(e.expiresAt) - Date.parse(e.trashedAt)) / 86_400_000)
}

function entryFiles(e: TrashEntry, t: TFunction) {
  if (e.wholeGame)
    return `${e.console}/${e.folder}/ · ${t('trash.files', { count: e.items.length })}`
  const item = e.items[0]
  if (!item) return ''
  return item.files.length > 1
    ? `${item.files[0] ?? ''} + ${t('trash.files', { count: item.files.length - 1 })}`
    : (item.files[0] ?? '')
}

type Ask =
  | { type: 'delete'; entry: TrashEntry }
  | { type: 'empty' }
  | { type: 'conflict'; entry: TrashEntry; detail: string }

/** Settings › Trash: restore, delete for good, empty (RF-30). */
export function TrashPanel() {
  const { t } = useTranslation()
  const format = useFormat()
  const now = useNow(60_000)
  const trash = useTrash()
  const consoles = useConsoles()
  const slow = useDelayedFlag(trash.isPending)
  const restore = useRestore()
  const remove = useDeleteTrashEntry()
  const empty = useEmptyTrash()
  const [ask, setAsk] = useState<Ask | null>(null)
  const [notice, setNotice] = useState<{ tone: 'info' | 'danger'; text: string } | null>(null)

  const entries = trash.data ?? []
  const names = new Map((consoles.data ?? []).map((c) => [c.slug, c.displayName]))
  const total = entries.reduce((s, e) => s + e.size, 0)
  const days = retentionDays(entries)

  const doRestore = (entry: TrashEntry, replace: boolean) => {
    restore.mutate(
      { id: entry.id, replace },
      {
        onSuccess: (r) => {
          setAsk(null)
          setNotice({
            tone: 'info',
            text: t('trash.restored', { title: entry.title, path: r.path }),
          })
        },
        onError: (e) => {
          if (!replace && isAppError(e, 'conflict')) {
            setAsk({ type: 'conflict', entry, detail: describeError(t, e) })
          } else {
            setAsk(null)
            setNotice({ tone: 'danger', text: describeError(t, e) })
          }
        },
      },
    )
  }

  if (trash.isPending) {
    return slow ? (
      <div aria-hidden="true" className="flex flex-col gap-3">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className="h-16 w-full" />
        ))}
      </div>
    ) : null
  }
  if (trash.isError) {
    return <EmptyState title={t('errors.loadTitle')} body={t('errors.loadBody')} />
  }

  return (
    <section className="flex flex-col gap-5">
      <div className="flex flex-wrap items-end justify-between gap-5">
        <div className="flex flex-col gap-1">
          <span className="text-count font-medium">
            {t('trash.summary', { count: entries.length, size: format.size(total) })}
          </span>
          {days !== undefined && (
            <span className="text-body-sm text-ink-2">{t('trash.retention', { count: days })}</span>
          )}
        </div>
        {entries.length > 0 && (
          <Button
            variant="danger"
            icon={<TrashIcon />}
            onClick={() => {
              setAsk({ type: 'empty' })
            }}
          >
            {t('trash.empty')}
          </Button>
        )}
      </div>

      {notice && <Banner tone={notice.tone}>{notice.text}</Banner>}

      {entries.length === 0 ? (
        <EmptyState
          icon={<TrashIcon size={40} strokeWidth={1.6} />}
          title={t('trash.emptyTitle')}
          body={t('trash.emptyBody')}
        />
      ) : (
        <TableBox>
          <table className="w-full min-w-[860px] border-collapse">
            <thead>
              <tr className="text-left text-caption font-bold tracking-[0.06em] text-ink-3 uppercase">
                <th className="px-5 py-3.5">{t('trash.item')}</th>
                <th className="px-5 py-3.5">{t('trash.console')}</th>
                <th className="px-5 py-3.5">{t('trash.trashedAt')}</th>
                <th className="px-5 py-3.5">{t('trash.purge')}</th>
                <th className="px-5 py-3.5 text-right">{t('game.size')}</th>
                <th className="px-5 py-3.5">
                  <span className="sr-only">{t('game.actions')}</span>
                </th>
              </tr>
            </thead>
            <tbody className="text-body">
              {entries.map((e) => {
                const left = format.daysUntil(e.expiresAt, now)
                const item = e.items[0]
                return (
                  <tr key={e.id} className="border-t border-line">
                    <td className="px-5 py-3.5">
                      <div className="flex flex-col gap-1">
                        <span className="flex flex-wrap items-center gap-2.5 font-bold">
                          {e.title}
                          {e.wholeGame ? (
                            <Chip kind="whole">{t('kind.whole')}</Chip>
                          ) : (
                            item && <ItemChip item={item} />
                          )}
                        </span>
                        <span className="font-mono text-chip [overflow-wrap:anywhere] text-ink-3">
                          {entryFiles(e, t)}
                        </span>
                        {e.reason === 'replaced' && (
                          <span className="text-caption text-ink-2">{t('trash.replaced')}</span>
                        )}
                      </div>
                    </td>
                    <td className="px-5 py-3.5 text-ink-1">{names.get(e.console) ?? e.console}</td>
                    <td className="px-5 py-3.5 whitespace-nowrap text-ink-2">
                      {format.date(e.trashedAt)}
                    </td>
                    <td
                      className={cx(
                        'px-5 py-3.5 whitespace-nowrap',
                        left <= 7 ? 'font-bold text-danger' : 'text-ink-2',
                      )}
                    >
                      {t('trash.inDays', { count: left })}
                    </td>
                    <td className="px-5 py-3.5 text-right whitespace-nowrap text-ink-2">
                      {format.size(e.size)}
                    </td>
                    <td className="px-4 py-2 text-right whitespace-nowrap">
                      <Button
                        size="sm"
                        icon={<RestoreIcon size={16} />}
                        loading={restore.isPending && restore.variables.id === e.id}
                        onClick={() => {
                          setNotice(null)
                          doRestore(e, false)
                        }}
                      >
                        {t('trash.restore')}
                      </Button>
                      <IconButton
                        tone="danger"
                        className="ml-2"
                        label={t('trash.deleteForever', { title: e.title })}
                        onClick={() => {
                          setAsk({ type: 'delete', entry: e })
                        }}
                      >
                        <TrashIcon />
                      </IconButton>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </TableBox>
      )}

      {ask?.type === 'conflict' && (
        <Dialog
          title={t('trash.conflictTitle')}
          onClose={() => {
            setAsk(null)
          }}
          actions={
            <>
              <Button
                onClick={() => {
                  setAsk(null)
                }}
              >
                {t('common.cancel')}
              </Button>
              <Button
                variant="primary"
                loading={restore.isPending}
                onClick={() => {
                  doRestore(ask.entry, true)
                }}
              >
                {t('trash.replace')}
              </Button>
            </>
          }
        >
          <p className="m-0">{t('trash.conflictBody', { title: ask.entry.title })}</p>
          <p className="m-0 rounded-md bg-surface-card px-4 py-3.5 text-body-sm text-ink-2">
            {ask.detail}
          </p>
        </Dialog>
      )}
      {(ask?.type === 'delete' || ask?.type === 'empty') && (
        <ConfirmDialog
          tone="danger"
          title={
            ask.type === 'empty'
              ? t('trash.emptyConfirmTitle')
              : t('trash.deleteTitle', { title: ask.entry.title })
          }
          confirmLabel={ask.type === 'empty' ? t('trash.empty') : t('trash.deleteConfirm')}
          busy={remove.isPending || empty.isPending}
          onCancel={() => {
            setAsk(null)
          }}
          onConfirm={() => {
            const done = {
              onSettled: () => {
                setAsk(null)
              },
              onError: (err: unknown) => {
                setNotice({ tone: 'danger', text: describeError(t, err) })
              },
            }
            if (ask.type === 'empty') empty.mutate(undefined, done)
            else remove.mutate(ask.entry.id, done)
          }}
        >
          <p className="m-0">
            {ask.type === 'empty'
              ? t('trash.emptyConfirmBody', { count: entries.length, size: format.size(total) })
              : t('trash.deleteBody', { size: format.size(ask.entry.size) })}
          </p>
        </ConfirmDialog>
      )}
    </section>
  )
}
