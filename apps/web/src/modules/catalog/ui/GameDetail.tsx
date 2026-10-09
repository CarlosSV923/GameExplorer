import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Cover } from '@/modules/metadata/ui/Cover'
import { describeError } from '@/shared/i18n/errors'
import { useFormat } from '@/shared/i18n/hooks'
import { useAction } from '@/shared/input'
import { isAppError } from '@/shared/kernel/errors'
import { useDelayedFlag } from '@/shared/kernel/hooks'
import {
  ArrowRightIcon,
  Banner,
  Button,
  ConfirmDialog,
  DownloadIcon,
  EditIcon,
  EmptyState,
  FolderIcon,
  IconButton,
  IconLink,
  LinkButton,
  MoreIcon,
  MoreMenu,
  Skeleton,
  TableBox,
  TrashIcon,
  UploadIcon,
} from '@/shared/ui'

import { useCatalogPorts } from '../application/ports'
import {
  useConsoles,
  useGame,
  useTrashGame,
  useTrashItem,
  useUnassignGame,
  useUnassignItem,
} from '../application/queries'
import { fileExtension, moveTargets, singleFile } from '../domain/items'
import type { Console, GameDetail as Game, GameEditResult, LibraryItem } from '../domain/types'
import { EditGameDialog, EditItemDialog } from './EditDialogs'
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

/** What the detail is asking about or editing. */
type Pending =
  | { type: 'trashGame' }
  | { type: 'unassignGame' }
  | { type: 'trashItem'; item: LibraryItem }
  | { type: 'unassignItem'; item: LibraryItem }
  | { type: 'rename' }
  | { type: 'move' }
  | { type: 'editItem'; item: LibraryItem }

function ItemsTable({
  game,
  editable,
  onAsk,
}: {
  game: Game
  editable: boolean
  onAsk: (p: Pending) => void
}) {
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
          {game.items.map((item) => (
            <tr key={item.id} className="border-t border-line">
              <td className="px-4 py-2.5">
                <ItemChip item={item} />
              </td>
              <td className="px-4 py-2.5 font-mono text-caption [overflow-wrap:anywhere] text-ink-1">
                {item.file}
              </td>
              <td className="px-4 py-2.5 text-right whitespace-nowrap text-ink-2">
                {format.size(item.size)}
              </td>
              <td className="px-3 py-1.5 text-right whitespace-nowrap">
                <span className="inline-flex items-center">
                  <IconLink
                    href={library.itemDownloadUrl(item.id)}
                    download=""
                    label={t('game.downloadItem', { name: item.file })}
                  >
                    <DownloadIcon />
                  </IconLink>
                  {editable && (
                    <IconButton
                      label={t('game.editItem', { name: item.file })}
                      onClick={() => {
                        onAsk({ type: 'editItem', item })
                      }}
                    >
                      <EditIcon />
                    </IconButton>
                  )}
                  <IconButton
                    tone="danger"
                    label={t('game.trashItem', { name: item.file })}
                    onClick={() => {
                      onAsk({ type: 'trashItem', item })
                    }}
                  >
                    <TrashIcon />
                  </IconButton>
                  <MoreMenu
                    compact
                    label={t('game.moreFor', { name: item.file })}
                    items={[
                      {
                        label: t('game.unassign'),
                        onSelect: () => {
                          onAsk({ type: 'unassignItem', item })
                        },
                      },
                    ]}
                  >
                    <MoreIcon />
                  </MoreMenu>
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </TableBox>
  )
}

/** The data under the title: IGDB's, or the note of a name of its own. */
function Meta({ game }: { game: Game }) {
  const { t } = useTranslation()
  if (!game.igdbId) {
    return <span className="text-body text-ink-2">{t('game.ownName')}</span>
  }
  const meta = [game.releaseYear, game.genres.join(' · ')].filter(Boolean)
  return (
    <>
      <div className="flex flex-wrap gap-x-5 gap-y-2 text-body text-ink-2">
        {meta.map((m) => (
          <span key={String(m)}>{m}</span>
        ))}
        <span className="font-bold text-success">{t('game.linked')}</span>
      </div>
      {game.summary && (
        <p className="m-0 line-clamp-6 max-w-[62ch] text-body-lg text-ink-1">{game.summary}</p>
      )}
    </>
  )
}

/** The detail panel of a game (RF-21, RF-24): cover, data, files and actions. */
export function GameDetail({
  gameId,
  back,
  onGone,
  onEdited,
}: {
  gameId: number
  /** On narrow screens the detail is its own page: a link back to the list. */
  back?: ReactNode
  /** The game left (trash, unassigned): show the list again. */
  onGone: () => void
  /** Renamed or moved: the game may have another id and console now. */
  onEdited: (result: GameEditResult) => void
}) {
  const { t } = useTranslation()
  const format = useFormat()
  const { library } = useCatalogPorts()
  const game = useGame(gameId)
  const consoles = useConsoles()
  const slow = useDelayedFlag(game.isPending)
  const trashGame = useTrashGame()
  const trashItem = useTrashItem()
  const unassignGame = useUnassignGame()
  const unassignItem = useUnassignItem()
  const [pending, setPending] = useState<Pending | null>(null)
  const [error, setError] = useState<string | null>(null)

  const only = game.data?.items.length === 1 ? game.data.items[0] : undefined
  const downloadUrl = only ? library.itemDownloadUrl(only.id) : library.gameDownloadUrl(gameId)

  // X downloads the whole game (docs/design-handoff.md §4).
  useAction(
    'action1',
    () => {
      window.location.assign(downloadUrl)
      return true
    },
    game.isSuccess && pending === null,
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
  const all = consoles.data ?? []
  const console: Console | undefined = all.find((c) => c.slug === g.console)
  const canMove = moveTargets(g.items, g.console, all).some((x) => !x.blockedBy)
  const editable = console !== undefined && !singleFile(console)
  const exts = [...new Set(g.items.map((it) => fileExtension(it.file, all)).filter(Boolean))]

  const confirm = () => {
    if (!pending) return
    const done = {
      onSuccess: () => {
        setPending(null)
        setError(null)
        if (pending.type === 'trashGame' || pending.type === 'unassignGame') onGone()
      },
      onError: (e: unknown) => {
        setPending(null)
        setError(describeError(t, e))
      },
    }
    if (pending.type === 'trashGame') trashGame.mutate(g.id, done)
    else if (pending.type === 'unassignGame') unassignGame.mutate(g.id, done)
    else if (pending.type === 'trashItem') trashItem.mutate(pending.item.id, done)
    else if (pending.type === 'unassignItem') unassignItem.mutate(pending.item.id, done)
  }

  const busy =
    trashGame.isPending || trashItem.isPending || unassignGame.isPending || unassignItem.isPending

  let ask: { title: string; body: string; confirm: string } | null = null
  if (pending?.type === 'trashGame' || pending?.type === 'trashItem') {
    const name = pending.type === 'trashItem' ? pending.item.file : g.title
    ask = {
      title: t('game.trashTitle', { name }),
      body: t('game.trashBody'),
      confirm: t('game.trashConfirm'),
    }
  } else if (pending?.type === 'unassignGame' || pending?.type === 'unassignItem') {
    const name = pending.type === 'unassignItem' ? pending.item.file : g.title
    ask = {
      title: t('game.unassignTitle', { name }),
      body: t('game.unassignBody'),
      confirm: t('game.unassignConfirm'),
    }
  }

  return (
    <div className="flex flex-col gap-7">
      {back}
      <div className="flex flex-wrap items-start gap-8">
        <Cover
          imageId={g.coverImageId}
          size="big"
          {...(g.igdbId ? {} : { generic: { title: g.title, caption: `/${g.console}` } })}
        />
        <div className="flex min-w-0 flex-[1_1_320px] flex-col gap-3.5">
          <h2 className="m-0 text-display leading-[1.1] font-bold">{g.title}</h2>
          <Meta game={g} />
          <span className="flex items-center gap-2 font-mono text-caption [overflow-wrap:anywhere] text-ink-3">
            <FolderIcon size={16} className="shrink-0" />
            {g.path}/
          </span>
          <div className="mt-1.5 flex flex-wrap gap-3">
            <LinkButton variant="primary" href={downloadUrl} download="" icon={<DownloadIcon />}>
              {only
                ? t('game.downloadOne', {
                    ext: fileExtension(only.file, all) || '',
                    size: format.size(g.size),
                  })
                : t('game.downloadAll', { size: format.size(g.size) })}
            </LinkButton>
            <Button
              icon={<EditIcon />}
              onClick={() => {
                setPending({ type: 'rename' })
              }}
            >
              {t('game.rename')}
            </Button>
            <Button
              icon={<ArrowRightIcon />}
              disabled={!canMove}
              {...(canMove
                ? {}
                : { disabledReason: t('game.moveBlocked', { exts: exts.join(' ') }) })}
              onClick={() => {
                setPending({ type: 'move' })
              }}
            >
              {t('game.move')}
            </Button>
            <Button
              variant="danger"
              icon={<TrashIcon />}
              onClick={() => {
                setPending({ type: 'trashGame' })
              }}
            >
              {t('game.trash')}
            </Button>
            <MoreMenu
              label={t('game.more')}
              items={[
                {
                  label: t('game.unassignGame'),
                  onSelect: () => {
                    setPending({ type: 'unassignGame' })
                  },
                },
              ]}
            >
              <MoreIcon />
              {t('game.more')}
            </MoreMenu>
          </div>
        </div>
      </div>

      {error && <Banner tone="danger">{error}</Banner>}

      <section className="flex flex-col gap-3">
        <h3 className="m-0 text-body-sm font-bold tracking-label text-ink-2 uppercase">
          {t('game.files')}
        </h3>
        <ItemsTable game={g} editable={editable} onAsk={setPending} />
        <span className="flex items-center gap-2 text-body-sm text-ink-3">
          <UploadIcon size={16} className="shrink-0" />
          {console && singleFile(console)
            ? t('game.oneFile', { console: console.displayName })
            : t('game.dropHint', { console: console?.displayName ?? g.console })}
        </span>
      </section>

      {ask && (
        <ConfirmDialog
          title={ask.title}
          confirmLabel={ask.confirm}
          busy={busy}
          onCancel={() => {
            setPending(null)
          }}
          onConfirm={confirm}
        >
          <p className="m-0">{ask.body}</p>
        </ConfirmDialog>
      )}
      {(pending?.type === 'rename' || pending?.type === 'move') && (
        <EditGameDialog
          game={g}
          mode={pending.type}
          consoles={all}
          onClose={() => {
            setPending(null)
          }}
          onDone={(result) => {
            setPending(null)
            onEdited(result)
          }}
        />
      )}
      {pending?.type === 'editItem' && console && (
        <EditItemDialog
          game={g}
          item={pending.item}
          console={console}
          consoles={all}
          onClose={() => {
            setPending(null)
          }}
        />
      )}
    </div>
  )
}
