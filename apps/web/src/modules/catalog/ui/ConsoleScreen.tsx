import { Link, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { UploadButton, UploadsIndicator } from '@/modules/ingestion/ui/UploadButton'
import { useAction } from '@/shared/input'
import { isAppError } from '@/shared/kernel/errors'
import { matchesWords } from '@/shared/kernel/text'
import { BackLink } from '@/shared/routing/links'
import {
  BackLabel,
  Button,
  EmptyState,
  GamepadIcon,
  HelpBar,
  PageHeader,
  PlusIcon,
  SearchField,
  SearchIcon,
} from '@/shared/ui'

import { useConsoleGames, useConsoles } from '../application/queries'
import { GameBrowser, GameList } from './GameBrowser'
import { GameDetail } from './GameDetail'

/** A console: its games by title and the selected game's detail (RF-21). */
export function ConsoleScreen({ slug, gameId }: { slug: string; gameId: number | undefined }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const consoles = useConsoles()
  const games = useConsoleGames(slug)
  const [filter, setFilter] = useState('')

  const info = consoles.data?.find((c) => c.slug === slug)
  const name = info?.displayName ?? slug
  const all = games.data ?? []
  const shown = filter.trim() ? all.filter((g) => matchesWords(g.title, filter)) : all
  // Wide screens show a detail at once: the first game when none is chosen.
  const selectedId = gameId ?? shown[0]?.id

  useAction('back', () => {
    if (gameId !== undefined && window.matchMedia('(max-width: 1023px)').matches) {
      void navigate({ to: '/consolas/$slug', params: { slug } })
    } else {
      void navigate({ to: '/', search: { consola: slug } })
    }
    return true
  })

  let list
  if (games.isError) {
    list = (
      <div className="px-4 sm:px-6 lg:px-10">
        <EmptyState
          title={isAppError(games.error, 'notFound') ? t('console.unknown') : t('errors.loadTitle')}
          body={t('errors.loadBody')}
        />
      </div>
    )
  } else if (all.length === 0) {
    list = (
      <div className="px-4 sm:px-6 lg:px-10">
        <EmptyState
          icon={<GamepadIcon size={40} strokeWidth={1.6} />}
          title={t('console.emptyTitle', { name })}
          body={t('console.emptyBody', { name })}
          action={
            <UploadButton consoleSlug={slug} icon={<PlusIcon />}>
              {t('help.add')}
            </UploadButton>
          }
        />
      </div>
    )
  } else if (shown.length === 0) {
    list = (
      <div className="px-4 sm:px-6 lg:px-10">
        <EmptyState
          icon={<SearchIcon size={40} />}
          title={t('search.noMatchTitle', { query: filter.trim() })}
          body={t('search.noMatchBody')}
          action={
            <Button
              onClick={() => {
                setFilter('')
              }}
            >
              {t('search.clear')}
            </Button>
          }
        />
      </div>
    )
  } else {
    list = (
      <GameList
        groups={[{ games: shown }]}
        selectedId={selectedId}
        chosen={gameId !== undefined}
        renderLink={(g, props) => (
          <Link to="/consolas/$slug/$gameId" params={{ slug, gameId: String(g.id) }} {...props} />
        )}
      />
    )
  }

  return (
    <div className="flex min-h-dvh flex-col">
      <PageHeader
        back={
          <BackLink to="/" search={{ consola: slug }}>
            <BackLabel>{t('nav.consoles')}</BackLabel>
          </BackLink>
        }
        title={name}
        aside={
          <>
            <SearchField
              shortcut
              label={t('console.filterLabel')}
              placeholder={t('console.filterPlaceholder', { name })}
              value={filter}
              onChange={(e) => {
                setFilter(e.target.value)
              }}
              className="min-w-[min(300px,100%)]"
            />
            <UploadButton consoleSlug={slug} size="sm" icon={<PlusIcon />}>
              {t('help.add')}
            </UploadButton>
            <UploadsIndicator />
            <span className="text-count font-medium">{t('home.games', { count: all.length })}</span>
          </>
        }
      />
      <GameBrowser
        label={t('console.gamesLabel')}
        loading={games.isPending}
        showDetail={gameId !== undefined}
        list={list}
        detail={
          selectedId !== undefined && (
            <GameDetail
              key={selectedId}
              gameId={selectedId}
              back={
                <BackLink to="/consolas/$slug" params={{ slug }} className="lg:hidden">
                  <BackLabel>{t('console.allGames', { name })}</BackLabel>
                </BackLink>
              }
              onGone={() => {
                void navigate({ to: '/consolas/$slug', params: { slug } })
              }}
              onEdited={(r) => {
                void navigate({
                  to: '/consolas/$slug/$gameId',
                  params: { slug: r.path.split('/')[0] ?? slug, gameId: String(r.gameId) },
                })
              }}
            />
          )
        }
      />
      <HelpBar
        actions={[
          { glyph: 'A', label: t('help.select') },
          { glyph: 'B', label: t('help.back') },
          { glyph: 'X', label: t('help.download') },
          { glyph: 'Y', label: t('help.search') },
          { glyph: 'RT', label: t('help.add') },
          { glyph: 'DPAD', label: t('help.choose') },
        ]}
      />
    </div>
  )
}
