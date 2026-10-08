import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Cover } from '@/modules/metadata/ui/Cover'
import { describeError } from '@/shared/i18n/errors'
import { useFormat } from '@/shared/i18n/hooks'
import { useAction } from '@/shared/input'
import { isAppError } from '@/shared/kernel/errors'
import { useDelayedFlag } from '@/shared/kernel/hooks'
import { ButtonLink } from '@/shared/routing/links'
import {
  Banner,
  Button,
  Chip,
  ConfirmDialog,
  DownloadIcon,
  EmptyState,
  FolderIcon,
  IconButton,
  IconLink,
  LinkButton,
  RematchIcon,
  Skeleton,
  TableBox,
  TrashIcon,
  UploadIcon,
} from '@/shared/ui'

import { useCatalogPorts } from '../application/ports'
import { useForgetItem, useGame, useTrashGame, useTrashItem } from '../application/queries'
import type { GameDetail as Game, LibraryItem } from '../domain/types'
import { ItemChip } from './ItemChip'

function DetailSkeleton() {
  return (
    <div aria-hidden="true" className="flex flex-wrap gap-8">
      <Skeleton className="h-[293px] w-[220px]" />
      <div className="flex min-w-80 flex-1 flex-col gap-4">
        <Skeleton className="h-10 w-3/4" />
        <Skeleton className="h-5 w-1/3" />
        <Skeleton className="h-24 w-full" />
      </div>
    </div>
  )
}

type Pending =
  { type: 'game' } | { type: 'item'; item: LibraryItem } | { type: 'forget'; item: LibraryItem }

function ItemsTable({ game, onAsk }: { game: Game; onAsk: (p: Pending) => void }) {
  const { t } = useTranslation()
  const format = useFormat()
  const { library } = useCatalogPorts()
  return (
    <TableBox>
      <table className="w-full min-w-[640px] border-collapse">
        <thead>
          <tr className="text-left text-caption font-bold tracking-[0.06em] text-ink-3 uppercase">
            <th className="px-4 py-3">{t('game.kind')}</th>
            <th className="px-4 py-3">{t('game.file')}</th>
            <th className="px-4 py-3 text-right">{t('game.size')}</th>
            <th className="px-4 py-3">
              <span className="sr-only">{t('game.actions')}</span>
            </th>
          </tr>
        </thead>
        <tbody className="text-body-sm">
          {game.items.map((item) => {
            const name = item.files[0] ?? ''
            const missing = Boolean(item.missingSince)
            return (
              <tr key={item.id} className="border-t border-line">
                <td className="px-4 py-2.5">
                  <ItemChip item={item} />
                </td>
                <td className="px-4 py-2.5">
                  <span className="flex flex-wrap items-center gap-2 font-mono text-caption [overflow-wrap:anywhere] text-ink-1">
                    {name}
                    {item.files.length > 1 && (
                      <span className="text-ink-3">
                        {t('game.moreFiles', { count: item.files.length - 1 })}
                      </span>
                    )}
                    {missing && <Chip kind="missing">{t('kind.missing')}</Chip>}
                  </span>
                </td>
                <td className="px-4 py-2.5 text-right whitespace-nowrap text-ink-2">
                  {format.size(item.size)}
                </td>
                <td className="px-3 py-1.5 text-right whitespace-nowrap">
                  {missing ? (
                    <Button
                      size="sm"
                      onClick={() => {
                        onAsk({ type: 'forget', item })
                      }}
                    >
                      {t('game.forget')}
                    </Button>
                  ) : (
                    <>
                      <IconLink
                        href={library.itemDownloadUrl(item.id)}
                        download=""
                        label={t('game.downloadItem', { name })}
                      >
                        <DownloadIcon />
                      </IconLink>
                      <IconButton
                        tone="danger"
                        label={t('game.trashItem', { name })}
                        onClick={() => {
                          onAsk({ type: 'item', item })
                        }}
                      >
                        <TrashIcon />
                      </IconButton>
                    </>
                  )}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </TableBox>
  )
}

/** The detail panel of a game (RF-21): cover, data, files and actions. */
export function GameDetail({
  gameId,
  consoleName,
  back,
  onTrashed,
}: {
  gameId: number
  consoleName: string
  /** On narrow screens the detail is its own page: a link back to the list. */
  back?: ReactNode
  onTrashed: () => void
}) {
  const { t } = useTranslation()
  const format = useFormat()
  const { library } = useCatalogPorts()
  const game = useGame(gameId)
  const slow = useDelayedFlag(game.isPending)
  const trashGame = useTrashGame()
  const trashItem = useTrashItem()
  const forget = useForgetItem()
  const [pending, setPending] = useState<Pending | null>(null)
  const [error, setError] = useState<string | null>(null)

  // X downloads the whole game (docs/design-handoff.md §4).
  useAction(
    'action1',
    () => {
      window.location.assign(library.gameDownloadUrl(gameId))
      return true
    },
    game.isSuccess,
  )

  if (game.isPending) return slow ? <DetailSkeleton /> : null
  if (game.isError) {
    return (
      <EmptyState
        title={isAppError(game.error, 'notFound') ? t('game.goneTitle') : t('errors.loadTitle')}
        body={isAppError(game.error, 'notFound') ? t('game.goneBody') : t('errors.loadBody')}
      />
    )
  }
  const g = game.data

  const confirm = () => {
    if (!pending) return
    const done = {
      onSuccess: () => {
        setPending(null)
        setError(null)
        if (pending.type === 'game') onTrashed()
      },
      onError: (e: unknown) => {
        setPending(null)
        setError(describeError(t, e))
      },
    }
    if (pending.type === 'game') trashGame.mutate(g.id, done)
    else if (pending.type === 'item') trashItem.mutate(pending.item.id, done)
    else forget.mutate(pending.item.id, done)
  }

  const askName = pending && pending.type !== 'game' ? (pending.item.files[0] ?? '') : g.title
  const meta = [g.releaseYear, g.genres.join(' · ')].filter(Boolean)

  return (
    <div className="flex flex-col gap-7">
      {back}
      <div className="flex flex-wrap items-start gap-8">
        <Cover imageId={g.coverImageId} size="big" />
        <div className="flex min-w-0 flex-[1_1_320px] flex-col gap-3.5">
          <h2 className="m-0 text-display leading-[1.1] font-bold">{g.title}</h2>
          {meta.length > 0 && (
            <div className="flex flex-wrap gap-x-5 gap-y-2 text-body text-ink-2">
              {meta.map((m) => (
                <span key={String(m)}>{m}</span>
              ))}
            </div>
          )}
          {g.summary && (
            <p className="m-0 line-clamp-6 max-w-[62ch] text-body-lg text-ink-1">{g.summary}</p>
          )}
          <span className="flex items-center gap-2 font-mono text-caption [overflow-wrap:anywhere] text-ink-3">
            <FolderIcon size={16} className="shrink-0" />
            {g.path}/
          </span>
          <div className="mt-1.5 flex flex-wrap gap-3">
            <LinkButton
              variant="primary"
              href={library.gameDownloadUrl(g.id)}
              download=""
              icon={<DownloadIcon />}
            >
              {t('game.downloadAll', { size: format.size(g.size) })}
            </LinkButton>
            <ButtonLink
              to="/consolas/$slug/$gameId/reemparejar"
              params={{ slug: g.console, gameId: String(g.id) }}
              icon={<RematchIcon />}
            >
              {t('game.rematch')}
            </ButtonLink>
            <Button
              variant="danger"
              icon={<TrashIcon />}
              onClick={() => {
                setPending({ type: 'game' })
              }}
            >
              {t('game.trash')}
            </Button>
          </div>
        </div>
      </div>

      {error && <Banner tone="danger">{error}</Banner>}

      <section className="flex flex-col gap-3">
        <h3 className="m-0 text-body-sm font-bold tracking-label text-ink-2 uppercase">
          {t('game.files')}
        </h3>
        {g.missingCount > 0 && (
          <Banner>{t('game.missingBanner', { count: g.missingCount })}</Banner>
        )}
        <ItemsTable game={g} onAsk={setPending} />
        <span className="flex items-center gap-2 text-body-sm text-ink-3">
          <UploadIcon size={16} className="shrink-0" />
          {t('game.dropHint', { console: consoleName })}
        </span>
      </section>

      {pending && (
        <ConfirmDialog
          title={
            pending.type === 'forget'
              ? t('game.forgetTitle')
              : t('game.trashTitle', { name: askName })
          }
          confirmLabel={pending.type === 'forget' ? t('game.forget') : t('game.trashConfirm')}
          tone={pending.type === 'forget' ? 'danger' : 'primary'}
          busy={trashGame.isPending || trashItem.isPending || forget.isPending}
          onCancel={() => {
            setPending(null)
          }}
          onConfirm={confirm}
        >
          <p className="m-0">
            {pending.type === 'forget'
              ? t('game.forgetBody', { name: askName })
              : t('game.trashBody')}
          </p>
        </ConfirmDialog>
      )}
    </div>
  )
}
