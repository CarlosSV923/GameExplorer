import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ChoiceList, Spinner, TextField } from '@/shared/ui'

import { minQuery, useGameSearch } from '../application/queries'
import type { MetadataGame } from '../domain/types'
import { Cover } from './Cover'
import { IgdbError } from './IgdbError'

interface IgdbGamePickerProps {
  /** IGDB platform of the chosen console; results are limited to it. */
  platformId: number | undefined
  platformName: string
  initialQuery: string
  value: MetadataGame | undefined
  onChange: (game: MetadataGame) => void
  /** IGDB ids already in the library on this console: "Ya en tu biblioteca". */
  owned?: ReadonlySet<number>
}

/** Search box and results as radios (Review, Rematch): RF-09, RF-24. */
export function IgdbGamePicker({
  platformId,
  platformName,
  initialQuery,
  value,
  onChange,
  owned,
}: IgdbGamePickerProps) {
  const { t } = useTranslation()
  const [query, setQuery] = useState(initialQuery)
  const search = useGameSearch(query, platformId)

  const results = search.data ?? []
  // Keep the chosen game visible while the user searches for something else.
  const games = value && !results.some((g) => g.id === value.id) ? [value, ...results] : results

  let body
  if (query.trim().length < minQuery && !value) {
    body = <p className="m-0 text-body-sm text-ink-3">{t('igdb.typeMore')}</p>
  } else if (search.isError) {
    body = (
      <IgdbError
        error={search.error}
        onRetry={() => {
          void search.refetch()
        }}
      />
    )
  } else if (search.isPending && games.length === 0) {
    body = <Spinner label={t('igdb.searching')} />
  } else if (games.length === 0) {
    body = (
      <p className="m-0 text-body-sm text-ink-3">{t('igdb.noResults', { query: query.trim() })}</p>
    )
  } else {
    body = (
      <ChoiceList
        legend={t('igdb.results', { platform: platformName })}
        name="igdb-game"
        value={value ? String(value.id) : undefined}
        onChange={(id) => {
          const game = games.find((g) => String(g.id) === id)
          if (game) onChange(game)
        }}
        options={games.map((g) => ({
          value: String(g.id),
          title: g.name,
          meta: [g.releaseYear, platformName].filter(Boolean).join(' · '),
          art: <Cover imageId={g.coverImageId} size="small" />,
          ...(owned?.has(g.id)
            ? {
                badge: (
                  <span className="rounded-full bg-on-accent px-2.5 py-1 text-chip font-bold whitespace-nowrap text-accent">
                    {t('igdb.owned')}
                  </span>
                ),
              }
            : {}),
        }))}
      />
    )
  }

  return (
    <div className="flex flex-col gap-3">
      <TextField
        type="search"
        size="lg"
        hideLabel
        label={t('igdb.searchLabel')}
        placeholder={t('igdb.searchPlaceholder')}
        value={query}
        onChange={(e) => {
          setQuery(e.target.value)
        }}
      />
      <div aria-live="polite" className="flex flex-col gap-2">
        {body}
        {search.isFetching && games.length > 0 && (
          <span className="text-caption text-ink-3">{t('igdb.searching')}</span>
        )}
      </div>
    </div>
  )
}
