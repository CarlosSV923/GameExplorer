import { useId, useState, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { isAppError } from '@/shared/kernel/errors'
import { Button, CheckIcon, cx, EditIcon, ErrorIcon, SearchIcon } from '@/shared/ui'

import { minQuery, useGameSearch, useMetadataStatus } from '../application/queries'
import { linkedId, type NameValue } from '../domain/names'
import { Cover } from './Cover'

/**
 * The game name field (NameCombobox, docs/design-handoff.md §9): any text
 * is valid; IGDB suggestions of the console's platform are optional. No
 * suggestion is picked by itself: Enter without one highlighted keeps the
 * typed text. Without IGDB (or when it fails) the form goes on with the text.
 */
export function NameCombobox({
  label,
  platformId,
  value,
  onChange,
  error,
}: {
  label: string
  platformId: number | undefined
  value: NameValue
  onChange: (next: NameValue) => void
  error?: string
}) {
  const { t } = useTranslation()
  const id = useId()
  const listId = `${id}-list`
  const status = useMetadataStatus()
  const configured = status.data?.configured === true
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(-1)
  const search = useGameSearch(value.text, platformId, configured && open)
  const text = value.text.trim()

  const games = text.length >= minQuery ? (search.data ?? []) : []
  const showList = open && configured && text.length >= minQuery && !search.isError
  // The last option keeps the typed text as it is.
  const count = games.length + 1
  const linked = linkedId(value) !== undefined

  const pick = (index: number) => {
    const game = games[index]
    onChange(game ? { text: game.name, game } : { text: value.text })
    setOpen(false)
    setActive(-1)
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      if (!showList) {
        setOpen(true)
        return
      }
      e.preventDefault()
      const by = e.key === 'ArrowDown' ? 1 : -1
      // -1 (back in the text) is one more stop of the cycle.
      setActive((a) => ((a + 1 + by + count + 1) % (count + 1)) - 1)
    } else if (e.key === 'Enter' && showList) {
      e.preventDefault()
      pick(active)
    } else if (e.key === 'Escape' && showList) {
      e.preventDefault()
      setOpen(false)
      setActive(-1)
    }
  }

  const optionId = (i: number) => `${id}-option-${String(i)}`
  const errorId = `${id}-error`

  return (
    <div className="flex min-w-0 flex-col gap-2.5">
      <label htmlFor={id} className="text-body-sm font-bold tracking-label text-ink-2 uppercase">
        {label}
      </label>
      <div className="relative">
        <div
          className={cx(
            'flex h-13 items-center gap-2.5 rounded-lg border-2 bg-surface-input px-4 focus-within:outline-3 focus-within:outline-offset-2 focus-within:outline-accent',
            error ? 'border-danger' : 'border-accent',
          )}
        >
          <SearchIcon className="shrink-0 text-ink-2" />
          <input
            id={id}
            type="text"
            role="combobox"
            autoComplete="off"
            aria-expanded={showList}
            aria-controls={listId}
            aria-autocomplete="list"
            aria-activedescendant={showList && active >= 0 ? optionId(active) : undefined}
            aria-invalid={error ? true : undefined}
            aria-describedby={error ? errorId : undefined}
            value={value.text}
            onChange={(e) => {
              onChange({ text: e.target.value, ...(value.game ? { game: value.game } : {}) })
              setOpen(true)
              setActive(-1)
            }}
            onFocus={() => {
              if (!linked) setOpen(true)
            }}
            onBlur={() => {
              setOpen(false)
              setActive(-1)
            }}
            onKeyDown={onKeyDown}
            className="min-w-0 flex-1 border-0 bg-transparent text-body-lg font-semibold text-ink-1 outline-0"
          />
        </div>
        {showList && (
          <ul
            id={listId}
            role="listbox"
            aria-label={t('igdb.suggestions')}
            className="m-0 mt-2 flex list-none flex-col gap-0.5 rounded-lg border border-control bg-surface-card p-1.5"
          >
            {games.map((g, i) => (
              <li
                key={g.id}
                id={optionId(i)}
                role="option"
                aria-selected={i === active}
                onMouseDown={(e) => {
                  e.preventDefault()
                }}
                onClick={() => {
                  pick(i)
                }}
                className={cx(
                  'flex min-h-16 cursor-pointer items-center gap-3.5 rounded-md px-2.5 py-1.5',
                  i === active ? 'bg-accent text-on-accent' : 'text-ink-1 hover:bg-hover',
                )}
              >
                <Cover imageId={g.coverImageId} size="small" />
                <span className="flex min-w-0 flex-col gap-0.5">
                  <span className="truncate text-body-lg font-bold">{g.name}</span>
                  <span className="text-caption font-bold opacity-80">
                    {[g.releaseYear, t('igdb.source')].filter(Boolean).join(' · ')}
                  </span>
                </span>
              </li>
            ))}
            <li
              id={optionId(games.length)}
              role="option"
              aria-selected={active === games.length}
              onMouseDown={(e) => {
                e.preventDefault()
              }}
              onClick={() => {
                pick(games.length)
              }}
              className={cx(
                'flex min-h-13 cursor-pointer items-center gap-3.5 rounded-md border-t border-line px-2.5 py-1.5',
                active === games.length ? 'bg-accent text-on-accent' : 'text-ink-1 hover:bg-hover',
              )}
            >
              <span aria-hidden="true" className="inline-flex w-[42px] justify-center">
                <EditIcon size={20} />
              </span>
              <span className="flex min-w-0 flex-col gap-0.5">
                <span className="text-body font-bold [overflow-wrap:anywhere]">
                  {t('igdb.useText', { text })}
                </span>
                <span className="text-caption font-semibold opacity-80">
                  {t('igdb.useTextHint')}
                </span>
              </span>
            </li>
          </ul>
        )}
      </div>
      {error && (
        <span id={errorId} className="flex items-start gap-2 text-body-sm text-danger">
          <ErrorIcon className="mt-px shrink-0" />
          {error}
        </span>
      )}
      {status.isSuccess && !configured && (
        <span className="text-body-sm leading-normal font-semibold text-ink-2">
          {t('igdb.notConfigured')}
        </span>
      )}
      {search.isError && (
        <span
          role="status"
          className="flex flex-wrap items-center gap-x-3 gap-y-2 text-body-sm font-semibold text-ink-2"
        >
          {isAppError(search.error, 'igdbUnconfigured')
            ? t('igdb.notConfigured')
            : t('igdb.failed')}
          <Button
            size="sm"
            onClick={() => {
              void search.refetch()
            }}
          >
            {t('common.retry')}
          </Button>
        </span>
      )}
      {configured && text && !showList && (
        <span
          className={cx(
            'inline-flex items-center gap-2 text-body-sm font-bold',
            linked ? 'text-success' : 'text-ink-2',
          )}
        >
          {linked ? <CheckIcon size={16} /> : <EditIcon size={16} />}
          {linked && value.game
            ? t('igdb.linked', {
                name: [value.game.name, value.game.releaseYear].filter(Boolean).join(' · '),
              })
            : t('igdb.ownName')}
        </span>
      )}
    </div>
  )
}
