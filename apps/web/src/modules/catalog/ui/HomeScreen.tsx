import { Link, useNavigate } from '@tanstack/react-router'
import {
  useEffect,
  useRef,
  useState,
  type SubmitEvent,
  type PointerEvent,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'

import { UploadButton, UploadsIndicator } from '@/modules/ingestion/ui/UploadButton'
import { ConsoleLogo } from '@/modules/metadata/ui/Cover'
import { useFormat } from '@/shared/i18n/hooks'
import { useAction } from '@/shared/input'
import { ownsArrows } from '@/shared/input/context'
import { useDelayedFlag } from '@/shared/kernel/hooks'
import { ButtonLink } from '@/shared/routing/links'
import {
  Button,
  ChevronLeftIcon,
  ChevronRightIcon,
  cx,
  EmptyState,
  FolderIcon,
  GamepadIcon,
  HelpBar,
  IconButton,
  MenuIcon,
  SearchField,
  Spinner,
  UploadIcon,
} from '@/shared/ui'

import { useCarousel, type CarouselEntry } from '../application/carousel'

/** Swipe distance that changes console (docs/design-handoff.md §4). */
const swipeThreshold = 48

/** The visible name of an entry: the console's, or "No asignados". */
function useEntryName() {
  const { t } = useTranslation()
  return (e: CarouselEntry) => (e.unassigned ? t('unassigned.title') : e.name)
}

function SideConsole({
  entry: c,
  distance,
  onPick,
}: {
  entry: CarouselEntry
  distance: number
  onPick: () => void
}) {
  const { t } = useTranslation()
  const name = useEntryName()(c)
  return (
    <button
      type="button"
      onClick={onPick}
      aria-label={t('home.show', { name })}
      className={cx(
        'min-h-24 min-w-0 flex-1 cursor-pointer flex-col items-center justify-center gap-1.5 rounded-md border-0 bg-transparent px-1 py-3 text-ink-1 md:p-3',
        distance === 3 ? 'hidden xl:flex' : distance === 2 ? 'hidden md:flex' : 'flex',
      )}
    >
      <span className="line-clamp-2 text-center text-chip leading-tight font-bold break-words uppercase opacity-78 md:text-body md:tracking-[0.06em]">
        {name}
      </span>
      {c.unassigned ? (
        <span className="text-chip text-ink-2">{t('unassigned.files', { count: c.count })}</span>
      ) : (
        c.releaseYear && <span className="text-chip text-ink-2">{c.releaseYear}</span>
      )}
    </button>
  )
}

function Rail({ side, children }: { side: 'left' | 'right'; children: ReactNode }) {
  const line = cx(
    'h-0.5 from-ink-1/75 from-82% to-transparent',
    side === 'left' ? 'bg-linear-to-r' : 'bg-linear-to-l',
  )
  return (
    <div className="flex min-w-0 flex-1 flex-col">
      <span aria-hidden="true" className={line} />
      <div className="flex items-center justify-evenly gap-3 overflow-hidden px-1 py-11 md:px-3">
        {children}
      </div>
      <span aria-hidden="true" className={line} />
    </div>
  )
}

/** Home: the carousel of consoles with games and the unassigned section (RF-20). */
export function HomeScreen({
  selected,
  onSelect,
}: {
  selected: string | undefined
  onSelect: (slug: string) => void
}) {
  const { t } = useTranslation()
  const format = useFormat()
  const navigate = useNavigate()
  const carousel = useCarousel()
  const entryName = useEntryName()
  const slow = useDelayedFlag(carousel.isPending)
  const [dir, setDir] = useState<'next' | 'prev'>('next')
  const [query, setQuery] = useState('')
  const [badLogo, setBadLogo] = useState<string | null>(null)
  const center = useRef<HTMLAnchorElement>(null)
  const swipe = useRef<number | null>(null)

  const list = carousel.entries
  const n = list.length
  const index = Math.max(
    0,
    list.findIndex((c) => c.slug === selected),
  )
  const current = list[index]
  const at = (offset: number) => list[(((index + offset) % n) + n) % n]
  // The others split between both sides without repeating (Main.dc.html).
  const right = Math.min(3, Math.ceil((n - 1) / 2))
  const left = Math.min(3, n - 1 - right)

  const go = (offset: number) => {
    const target = at(offset)
    if (!target || n < 2) return
    setDir(offset > 0 ? 'next' : 'prev')
    onSelect(target.slug)
  }
  const goTo = (i: number) => {
    const target = list[i]
    if (!target) return
    setDir(i > index ? 'next' : 'prev')
    onSelect(target.slug)
  }

  useAction('navigate', (direction) => {
    if (direction === 'left' || direction === 'right') {
      go(direction === 'left' ? -1 : 1)
      return true
    }
    return false
  })
  useAction('prev', () => {
    go(-1)
    return true
  })
  useAction('next', () => {
    go(1)
    return true
  })

  // Home and End go to the ends of the carousel.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.key !== 'Home' && e.key !== 'End') || ownsArrows(document.activeElement)) return
      e.preventDefault()
      goTo(e.key === 'Home' ? 0 : n - 1)
    }
    window.addEventListener('keydown', onKey)
    return () => {
      window.removeEventListener('keydown', onKey)
    }
  })

  // The current console has focus, so A / Enter opens it.
  const ready = current !== undefined
  useEffect(() => {
    if (ready && (document.activeElement === document.body || !document.activeElement)) {
      center.current?.focus()
    }
  }, [ready])

  const search = (e: SubmitEvent) => {
    e.preventDefault()
    const q = query.trim()
    if (q) void navigate({ to: '/buscar', search: { q } })
  }

  const onPointerDown = (e: PointerEvent) => {
    swipe.current = e.pointerType === 'mouse' ? null : e.clientX
  }
  const onPointerUp = (e: PointerEvent) => {
    if (swipe.current === null) return
    const dx = e.clientX - swipe.current
    swipe.current = null
    if (Math.abs(dx) >= swipeThreshold) go(dx > 0 ? -1 : 1)
  }

  const showLogo = current?.logoImageId && badLogo !== current.slug
  const currentName = current ? entryName(current) : ''
  const long = currentName.length > 14
  const empty = !carousel.isPending && !carousel.isError && list.length === 0
  // Uploading from here preselects the console in the middle (RF-03).
  const uploadConsole = current && !current.unassigned ? current.slug : undefined

  let main
  if (carousel.isError) {
    main = (
      <EmptyState
        title={t('errors.loadTitle')}
        body={t('errors.loadBody')}
        action={
          <Button
            onClick={() => {
              void carousel.refetch()
            }}
          >
            {t('common.retry')}
          </Button>
        }
      />
    )
  } else if (empty) {
    main = (
      <div className="mx-auto flex max-w-[620px] flex-col items-center gap-5 px-4 text-center">
        <GamepadIcon size={56} strokeWidth={1.6} className="text-ink-3" />
        <h1 className="m-0 text-display leading-[1.1] font-bold">{t('home.emptyTitle')}</h1>
        <p className="m-0 text-body-lg text-ink-2">
          {t('home.emptyBody', { names: format.orList(carousel.consoleNames) })}
        </p>
        <UploadButton size="lg">{t('help.upload')}</UploadButton>
        <span className="text-body text-ink-3">{t('home.emptyDrop')}</span>
      </div>
    )
  } else if (!current) {
    main = slow ? <Spinner label={t('common.loading')} size={32} /> : null
  } else {
    main = (
      <>
        <div className="flex w-full items-center">
          <Rail side="left">
            {Array.from({ length: left }, (_, i) => left - i).map((d) => {
              const c = at(-d)
              return c ? (
                <SideConsole
                  key={`l${String(d)}`}
                  entry={c}
                  distance={d}
                  onPick={() => {
                    go(-d)
                  }}
                />
              ) : null
            })}
          </Rail>
          <div className="box-border flex w-[46vw] flex-none flex-col items-center gap-4.5 px-3 md:w-[min(560px,62vw)]">
            <Link
              ref={center}
              {...(current.unassigned
                ? { to: '/no-asignados' as const }
                : { to: '/consolas/$slug' as const, params: { slug: current.slug } })}
              className="flex flex-col items-center gap-2 rounded-xl px-4 py-2 text-ink-1 no-underline hover:text-ink-1"
            >
              <span
                key={current.slug}
                className={cx(
                  'flex flex-col items-center gap-4',
                  dir === 'next' ? 'animate-carousel-next' : 'animate-carousel-prev',
                )}
              >
                {current.unassigned && (
                  <FolderIcon size={72} strokeWidth={1.6} className="text-ink-2" />
                )}
                {showLogo && (
                  <ConsoleLogo
                    imageId={current.logoImageId}
                    className="h-[clamp(96px,16vw,180px)] w-[min(300px,100%)]"
                    onMissing={() => {
                      setBadLogo(current.slug)
                    }}
                  />
                )}
                <h1
                  aria-live="polite"
                  className={cx(
                    'm-0 text-center font-bold tracking-display uppercase',
                    showLogo ? 'text-title' : long ? 'text-display-lg' : 'text-display-xl',
                  )}
                >
                  {currentName}
                </h1>
              </span>
              {current.unassigned ? (
                <span className="text-heading font-bold">
                  {t('unassigned.files', { count: current.count })}
                </span>
              ) : (
                current.releaseYear && (
                  <span className="text-heading font-bold">{current.releaseYear}</span>
                )
              )}
            </Link>
            <span className="text-center font-mono text-caption text-ink-3">
              {current.unassigned ? t('home.unassignedHint') : `/${current.slug}`}
            </span>
          </div>
          <Rail side="right">
            {Array.from({ length: right }, (_, i) => i + 1).map((d) => {
              const c = at(d)
              return c ? (
                <SideConsole
                  key={`r${String(d)}`}
                  entry={c}
                  distance={d}
                  onPick={() => {
                    go(d)
                  }}
                />
              ) : null
            })}
          </Rail>
        </div>
        <div className={cx('mt-10 flex justify-center gap-3', n < 2 && 'invisible')}>
          <IconButton
            label={t('home.previous')}
            className="border border-control"
            onClick={() => {
              go(-1)
            }}
          >
            <ChevronLeftIcon />
          </IconButton>
          <IconButton
            label={t('home.next')}
            className="border border-control"
            onClick={() => {
              go(1)
            }}
          >
            <ChevronRightIcon />
          </IconButton>
        </div>
      </>
    )
  }

  return (
    <div className="flex min-h-dvh flex-col overflow-hidden">
      <header className="flex flex-wrap items-center justify-between gap-4 px-4 pt-7 sm:px-6 lg:px-10">
        <div className="flex min-w-0 flex-wrap items-center gap-5">
          <span className="flex items-center gap-2.5 text-body-lg font-bold">
            <GamepadIcon size={22} className="text-accent" strokeWidth={2} />
            {t('app.name')}
          </span>
          <form role="search" onSubmit={search} className="min-w-[min(320px,100%)]">
            <SearchField
              shortcut
              label={t('home.searchLabel')}
              placeholder={t('home.searchPlaceholder')}
              value={query}
              onChange={(e) => {
                setQuery(e.target.value)
              }}
            />
          </form>
        </div>
        <div className="flex flex-wrap items-center gap-4">
          <UploadsIndicator />
          {current && (
            <span className="text-count font-medium">
              {current.unassigned
                ? t('unassigned.files', { count: current.count })
                : t('home.games', { count: current.count })}
            </span>
          )}
        </div>
      </header>

      <main
        aria-roledescription={t('home.carousel')}
        aria-label={t('home.consoles')}
        onPointerDown={onPointerDown}
        onPointerUp={onPointerUp}
        className="flex flex-1 touch-pan-y flex-col justify-center pt-6 pb-12"
      >
        {main}
      </main>

      <HelpBar
        actions={[
          { glyph: 'MENU', label: t('help.menu') },
          { glyph: 'A', label: t('help.select') },
          { glyph: 'Y', label: t('help.search') },
          { glyph: 'RT', label: t('help.upload') },
          { glyph: 'DPAD', label: t('help.choose') },
        ]}
      >
        <ButtonLink to="/ajustes/$tab" params={{ tab: 'consolas' }} icon={<MenuIcon />}>
          {t('help.menu')}
        </ButtonLink>
        <span className="hidden items-center gap-2.5 text-body text-ink-3 md:inline-flex">
          <UploadIcon />
          {current?.unassigned
            ? t('home.unassignedUpload')
            : current
              ? t('home.dropHintConsole', { name: currentName })
              : t('help.dropHint')}
        </span>
        <UploadButton {...(uploadConsole ? { consoleSlug: uploadConsole } : {})}>
          {t('help.upload')}
        </UploadButton>
      </HelpBar>
    </div>
  )
}
