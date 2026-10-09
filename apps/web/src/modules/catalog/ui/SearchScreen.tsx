import { Link, useNavigate } from '@tanstack/react-router'
import { useState, type SubmitEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { UploadsIndicator } from '@/modules/ingestion/ui/UploadButton'
import { useAction } from '@/shared/input'
import { BackLink } from '@/shared/routing/links'
import { BackLabel, EmptyState, HelpBar, PageHeader, SearchField, SearchIcon } from '@/shared/ui'

import { useConsoles, useLibrarySearch } from '../application/queries'
import { groupByConsole } from '../domain/items'
import { GameBrowser, GameList } from './GameBrowser'
import { GameDetail } from './GameDetail'

/** Search across consoles (RF-22): results grouped by console, with the detail. */
export function SearchScreen({ query, gameId }: { query: string; gameId: number | undefined }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const consoles = useConsoles()
  const results = useLibrarySearch(query)
  const [draft, setDraft] = useState(query)

  const names = new Map((consoles.data ?? []).map((c) => [c.slug, c.displayName]))
  const groups = groupByConsole(
    results.data ?? [],
    (consoles.data ?? []).map((c) => c.slug),
  ).map((g) => ({ heading: names.get(g.console) ?? g.console, games: g.games }))
  const first = groups[0]?.games[0]
  const selected = results.data?.find((g) => g.id === gameId) ?? first

  useAction('back', () => {
    if (gameId !== undefined && window.matchMedia('(max-width: 1023px)').matches) {
      void navigate({ to: '/buscar', search: { q: query } })
    } else {
      void navigate({ to: '/' })
    }
    return true
  })

  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    void navigate({ to: '/buscar', search: { q: draft.trim() }, replace: true })
  }

  const empty = !query.trim() || (results.isSuccess && results.data.length === 0)

  return (
    <div className="flex min-h-app flex-col">
      <PageHeader
        back={
          <BackLink to="/">
            <BackLabel>{t('nav.consoles')}</BackLabel>
          </BackLink>
        }
        title={t('search.title')}
        aside={
          <>
            <form role="search" onSubmit={submit} className="min-w-[min(280px,100%)]">
              <SearchField
                shortcut
                label={t('home.searchLabel')}
                placeholder={t('home.searchPlaceholder')}
                value={draft}
                onChange={(e) => {
                  setDraft(e.target.value)
                }}
              />
            </form>
            <UploadsIndicator />
            {results.data && (
              <span className="text-count font-medium">
                {t('home.games', { count: results.data.length })}
              </span>
            )}
          </>
        }
      />
      <GameBrowser
        label={t('search.resultsLabel')}
        loading={results.isPending && Boolean(query.trim())}
        showDetail={gameId !== undefined}
        list={
          empty ? (
            <div className="px-4 sm:px-6 lg:px-10">
              <EmptyState
                icon={<SearchIcon size={40} />}
                title={
                  query.trim()
                    ? t('search.noMatchTitle', { query: query.trim() })
                    : t('search.title')
                }
                body={query.trim() ? t('search.noMatchBody') : t('search.prompt')}
              />
            </div>
          ) : (
            <GameList
              groups={groups}
              selectedId={selected?.id}
              chosen={gameId !== undefined}
              renderLink={(g, props) => (
                <Link to="/buscar" search={{ q: query, juego: g.id }} {...props} />
              )}
            />
          )
        }
        detail={
          selected && (
            <GameDetail
              key={selected.id}
              gameId={selected.id}
              back={
                <BackLink to="/buscar" search={{ q: query }} className="lg:hidden">
                  <BackLabel>{t('search.results')}</BackLabel>
                </BackLink>
              }
              onGone={() => {
                void navigate({ to: '/buscar', search: { q: query } })
              }}
              onEdited={(r) => {
                void navigate({ to: '/buscar', search: { q: query, juego: r.gameId } })
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
          { glyph: 'DPAD', label: t('help.choose') },
        ]}
      />
    </div>
  )
}
