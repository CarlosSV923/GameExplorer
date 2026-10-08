import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { useDelayedFlag } from '@/shared/kernel/hooks'
import { cx, Skeleton } from '@/shared/ui'

import type { GameSummary } from '../domain/types'

export interface GameGroup {
  heading?: string
  games: GameSummary[]
}

export type RenderLink = (
  game: GameSummary,
  props: { className: string; 'aria-current'?: 'true'; children: ReactNode },
) => ReactNode

/** The list of games, by title; the selected one is highlighted (RF-21). */
export function GameList({
  groups,
  selectedId,
  chosen,
  renderLink,
}: {
  groups: readonly GameGroup[]
  selectedId: number | undefined
  /**
   * The user chose it (it is in the URL). Otherwise wide screens show the
   * first game's detail and only they highlight it: narrow ones show the list.
   */
  chosen: boolean
  renderLink: RenderLink
}) {
  const { t } = useTranslation()
  return (
    <div className="flex flex-col gap-4">
      {groups.map((group, i) => (
        <section key={group.heading ?? i} className="flex flex-col">
          {group.heading && (
            <h2 className="m-0 px-4 pt-2 pb-1 text-body-sm font-bold tracking-label text-ink-2 uppercase sm:px-6 lg:px-10">
              {group.heading}
            </h2>
          )}
          <ul className="m-0 flex list-none flex-col p-0">
            {group.games.map((g) => {
              const selected = g.id === selectedId
              return (
                <li key={g.id}>
                  {renderLink(g, {
                    className: cx(
                      'flex min-h-control-md items-center justify-between gap-3 border-l-4 px-4 text-body-lg no-underline sm:px-6 lg:pr-10 lg:pl-9',
                      'text-ink-1 hover:text-ink-1',
                      !selected && 'border-transparent hover:bg-hover',
                      selected && chosen && 'border-accent bg-surface-card font-bold',
                      selected &&
                        !chosen &&
                        'border-transparent hover:bg-hover lg:border-accent lg:bg-surface-card lg:font-bold',
                    ),
                    ...(selected && chosen ? { 'aria-current': 'true' as const } : {}),
                    children: (
                      <>
                        <span className="truncate" title={g.title}>
                          {g.title}
                        </span>
                        <span className="flex shrink-0 items-center gap-2 text-caption font-bold text-ink-2">
                          {g.missingCount > 0 && (
                            <span className="text-accent">
                              {t('game.missingShort', { count: g.missingCount })}
                            </span>
                          )}
                          {selected && (
                            <span className={chosen ? undefined : 'hidden lg:inline'}>
                              {t('game.itemCount', { count: g.itemCount })}
                            </span>
                          )}
                        </span>
                      </>
                    ),
                  })}
                </li>
              )
            })}
          </ul>
        </section>
      ))}
    </div>
  )
}

function ListSkeleton() {
  return (
    <div aria-hidden="true" className="flex flex-col gap-3 px-4 py-2 sm:px-6 lg:px-10">
      {Array.from({ length: 8 }, (_, i) => (
        <Skeleton key={i} className="h-7 w-full" />
      ))}
    </div>
  )
}

/**
 * List and detail side by side; below 1024 px they stack into two pages:
 * the list, or (with a game in the URL) the detail (docs/design-handoff.md §5).
 */
export function GameBrowser({
  label,
  loading,
  list,
  detail,
  showDetail,
}: {
  label: string
  loading: boolean
  list: ReactNode
  detail: ReactNode
  /** A game is selected in the URL: on narrow screens show the detail. */
  showDetail: boolean
}) {
  const slow = useDelayedFlag(loading)
  return (
    <main className="flex min-h-0 flex-1 items-stretch">
      <nav
        aria-label={label}
        className={cx(
          'box-border w-full py-5 lg:block lg:max-w-[460px] lg:flex-[1_1_340px] lg:border-r lg:border-line',
          showDetail && 'hidden',
        )}
      >
        {loading ? slow && <ListSkeleton /> : list}
      </nav>
      <section
        className={cx(
          'box-border min-w-0 flex-[999_1_560px] flex-col gap-7 px-4 pt-8 pb-10 sm:px-6 lg:flex lg:px-10',
          showDetail ? 'flex' : 'hidden',
        )}
      >
        {detail}
      </section>
    </main>
  )
}
