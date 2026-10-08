import { useNavigate } from '@tanstack/react-router'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { useConsoleGames, useConsoles } from '@/modules/catalog/application/queries'
import type { Console } from '@/modules/catalog/domain/types'
import { itemLabel } from '@/modules/catalog/ui/itemLabel'
import type { MetadataGame } from '@/modules/metadata/domain/types'
import { IgdbGamePicker } from '@/modules/metadata/ui/IgdbGamePicker'
import { describeError } from '@/shared/i18n/errors'
import { useFormat } from '@/shared/i18n/hooks'
import { useAction } from '@/shared/input'
import { isAppError } from '@/shared/kernel/errors'
import { useDelayedFlag } from '@/shared/kernel/hooks'
import { baseName } from '@/shared/kernel/text'
import { BackLink, ButtonLink } from '@/shared/routing/links'
import {
  BackLabel,
  Banner,
  Button,
  CheckIcon,
  ConfirmDialog,
  EmptyState,
  FolderIcon,
  HelpBar,
  PageHeader,
  SelectField,
  Spinner,
} from '@/shared/ui'

import { useCancelJob, useCommit, useCommitPlan, useJob, useJobItems } from '../application/queries'
import {
  detect,
  draftError,
  guessTitle,
  initialConsole,
  initialDraft,
  storedCount,
  toCommitItem,
  type ItemDraft,
} from '../domain/review'
import type { CommitRequest, StagedItem, UploadJob } from '../domain/types'
import { ItemDraftCard } from './ItemDraftCard'

function Step({ n, title, children }: { n: number; title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3">
      <h2 className="m-0 flex items-center gap-2.5 text-body-sm font-bold tracking-label text-ink-2 uppercase">
        <span
          aria-hidden="true"
          className="inline-flex size-6 items-center justify-center rounded-full bg-ink-1 text-caption text-on-accent"
        >
          {n}
        </span>
        {title}
      </h2>
      {children}
    </section>
  )
}

function ReviewForm({
  job,
  items,
  consoles,
}: {
  job: UploadJob
  items: StagedItem[]
  consoles: Console[]
}) {
  const { t } = useTranslation()
  const format = useFormat()
  const navigate = useNavigate()
  const kept = items.filter((i) => !i.ignored)
  const ignored = items.filter((i) => i.ignored)
  const detection = detect(items)
  const names = new Map(consoles.map((c) => [c.slug, c.displayName]))

  const [slug, setSlug] = useState(() =>
    initialConsole(
      job,
      detection,
      consoles.map((c) => c.slug),
    ),
  )
  const [game, setGame] = useState<MetadataGame | undefined>()
  const [drafts, setDrafts] = useState<Record<string, ItemDraft>>(() =>
    Object.fromEntries(kept.map((i) => [i.path, initialDraft(i, kept.length === 1)])),
  )
  const [tried, setTried] = useState(false)
  const [cancelling, setCancelling] = useState(false)

  const chosen = consoles.find((c) => c.slug === slug)
  const library = useConsoleGames(slug || undefined)
  const owned = new Set((library.data ?? []).map((g) => g.igdbId))
  const valid = kept.every((i) => {
    const d = drafts[i.path]
    return d !== undefined && draftError(d) === null
  })
  const request: CommitRequest | undefined =
    slug && game && valid
      ? {
          console: slug,
          igdbGameId: game.id,
          items: kept.map((i) => toCommitItem(i.path, drafts[i.path] ?? initialDraft(i, false))),
        }
      : undefined
  const plan = useCommitPlan(job.id, request)
  const current = request && !plan.isPlaceholderData ? plan.data : undefined
  const count = storedCount(drafts, current)
  const commit = useCommit(job.id, (r) => {
    void navigate({
      to: '/consolas/$slug/$gameId',
      params: { slug: r.path.split('/')[0] ?? slug, gameId: String(r.gameId) },
    })
  })
  const cancel = useCancelJob()

  let blocked: string | undefined
  if (!slug) blocked = t('review.needConsole')
  else if (!game) blocked = t('review.needGame')
  else if (count === 0) blocked = t('review.nothing')

  const save = () => {
    setTried(true)
    if (!request || blocked) return
    commit.mutate(request)
  }

  useAction('action1', () => {
    save()
    return true
  })

  const existing = current?.existing ?? []
  const totalSize = kept.reduce((s, i) => s + i.size, 0)

  return (
    <>
      <PageHeader
        back={
          <BackLink to="/subidas">
            <BackLabel>{t('uploads.title')}</BackLabel>
          </BackLink>
        }
        title={t('review.title')}
        meta={
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-body-sm text-ink-2">
            <span className="font-mono text-caption text-ink-1">{job.fileName}</span>
            <span>{t('review.summary', { count: kept.length, size: format.size(totalSize) })}</span>
          </div>
        }
      />
      <main className="box-border flex flex-1 flex-wrap items-start gap-8 px-4 pt-8 pb-10 sm:px-6 lg:px-10">
        <div className="flex min-w-0 flex-[1_1_380px] flex-col gap-7 lg:max-w-[520px]">
          <Step n={1} title={t('review.console')}>
            <SelectField
              label={t('review.console')}
              hideLabel
              value={slug}
              onChange={(e) => {
                setSlug(e.target.value)
                setGame(undefined)
              }}
              {...(tried && !slug ? { error: t('review.needConsole') } : {})}
            >
              <option value="" disabled>
                {t('review.chooseConsole')}
              </option>
              {consoles.map((c) => (
                <option key={c.slug} value={c.slug}>
                  {c.displayName}
                </option>
              ))}
            </SelectField>
            {job.originConsole && (
              <span className="flex items-center gap-2 text-body-sm text-ink-2">
                <FolderIcon size={16} />
                {t('review.fromConsole', {
                  name: names.get(job.originConsole) ?? job.originConsole,
                })}
              </span>
            )}
            {slug && detection.slug === slug && (
              <span className="flex items-center gap-2 text-body-sm text-success">
                <CheckIcon size={16} />
                {t(
                  detection.confidence === 'header'
                    ? 'review.matchesHeader'
                    : 'review.matchesExtension',
                )}
              </span>
            )}
            {detection.slug && slug && detection.slug !== slug && (
              <Banner
                action={
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={() => {
                      setSlug(detection.slug ?? '')
                      setGame(undefined)
                    }}
                  >
                    {t('review.use', { name: names.get(detection.slug) ?? detection.slug })}
                  </Button>
                }
              >
                {t('review.looksLike', { name: names.get(detection.slug) ?? detection.slug })}
              </Banner>
            )}
            {!detection.slug && detection.candidates.length > 1 && (
              <span className="text-body-sm text-ink-2">
                {t('review.maybe', {
                  names: detection.candidates.map((c) => names.get(c) ?? c).join(' · '),
                })}
              </span>
            )}
            {detection.candidates.length === 0 && (
              <span className="text-body-sm text-ink-2">{t('review.unknownConsole')}</span>
            )}
          </Step>

          <Step n={2} title={t('review.game')}>
            {chosen ? (
              <IgdbGamePicker
                key={chosen.slug}
                platformId={chosen.igdbPlatformId ?? undefined}
                platformName={chosen.displayName}
                initialQuery={guessTitle(job.fileName)}
                value={game}
                onChange={setGame}
                owned={owned}
              />
            ) : (
              <p className="m-0 text-body text-ink-3">{t('review.consoleFirst')}</p>
            )}
            {tried && slug && !game && (
              <span className="text-body-sm text-danger">{t('review.needGame')}</span>
            )}
          </Step>
        </div>

        <div className="flex min-w-0 flex-[999_1_560px] flex-col gap-4">
          <Step n={3} title={t('review.items')}>
            {existing.length > 0 && current && (
              <Banner>
                {t('review.existing', {
                  title: current.title,
                  console: names.get(current.console) ?? current.console,
                  items: existing.map((e) => itemLabel(t, e)).join(', '),
                })}
              </Banner>
            )}
            {plan.isError && request && (
              <Banner tone="danger">{describeError(t, plan.error)}</Banner>
            )}
            {kept.map((item) => {
              const draft = drafts[item.path]
              return draft ? (
                <ItemDraftCard
                  key={item.path}
                  item={item}
                  draft={draft}
                  plan={current}
                  showErrors={tried}
                  onChange={(next) => {
                    setDrafts((d) => ({ ...d, [item.path]: next }))
                  }}
                />
              ) : null
            })}
            {ignored.length > 0 && (
              <p className="m-0 text-body-sm text-ink-3">
                {t('review.ignored')}{' '}
                <span className="font-mono text-caption">
                  {ignored.map((i) => baseName(i.path)).join(', ')}
                </span>
              </p>
            )}
          </Step>

          {commit.isError && <Banner tone="danger">{describeError(t, commit.error)}</Banner>}

          <div className="flex flex-wrap items-start justify-end gap-3 pt-2">
            <Button
              size="lg"
              onClick={() => {
                setCancelling(true)
              }}
            >
              {t('review.cancelUpload')}
            </Button>
            <Button
              variant="primary"
              size="lg"
              icon={<CheckIcon />}
              loading={commit.isPending}
              disabled={Boolean(blocked) && tried}
              {...(blocked && tried ? { disabledReason: blocked } : {})}
              onClick={save}
            >
              {t('review.save', { count })}
            </Button>
          </div>
        </div>
      </main>

      {cancelling && (
        <ConfirmDialog
          tone="danger"
          title={t('review.cancelTitle')}
          confirmLabel={t('review.cancelUpload')}
          busy={cancel.isPending}
          onCancel={() => {
            setCancelling(false)
          }}
          onConfirm={() => {
            cancel.mutate(job.id, {
              onSuccess: () => {
                void navigate({ to: '/subidas' })
              },
              onSettled: () => {
                setCancelling(false)
              },
            })
          }}
        >
          <p className="m-0">{t('review.cancelBody', { name: job.fileName })}</p>
        </ConfirmDialog>
      )}
    </>
  )
}

/** Review an upload: console, IGDB game and items, then store it (RF-08 to RF-11). */
export function ReviewScreen({ jobId }: { jobId: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const job = useJob(jobId)
  const consoles = useConsoles()
  const status = job.data?.status
  const items = useJobItems(jobId, status === 'review' || status === 'committing')
  const slow = useDelayedFlag(job.isPending || consoles.isPending || items.isPending)

  useAction('back', () => {
    void navigate({ to: '/subidas' })
    return true
  })

  let content: ReactNode
  if (job.isError) {
    content = (
      <EmptyState
        title={isAppError(job.error, 'notFound') ? t('review.goneTitle') : t('errors.loadTitle')}
        body={t('review.goneBody')}
        action={
          <ButtonLink to="/subidas" variant="primary">
            {t('uploads.title')}
          </ButtonLink>
        }
      />
    )
  } else if (!job.data || !consoles.data) {
    content = slow ? <Spinner label={t('common.loading')} /> : null
  } else if (status !== 'review' && status !== 'committing') {
    const key =
      status === 'done' ? 'done' : status === 'failed' || status === 'cancelled' ? 'failed' : 'busy'
    content = (
      <EmptyState
        title={t(`review.state.${key}`)}
        body={key === 'failed' ? (job.data.error ?? t('errors.failed')) : t('review.state.body')}
        action={
          <ButtonLink to="/subidas" variant="primary">
            {t('uploads.title')}
          </ButtonLink>
        }
      />
    )
  } else if (!items.data) {
    content = slow ? <Spinner label={t('common.loading')} /> : null
  } else {
    return (
      <div className="flex min-h-dvh flex-col">
        <ReviewForm key={jobId} job={job.data} items={items.data} consoles={consoles.data} />
        <HelpBar
          actions={[
            { glyph: 'A', label: t('help.select') },
            { glyph: 'B', label: t('help.back') },
            { glyph: 'X', label: t('help.save') },
            { glyph: 'DPAD', label: t('help.choose') },
          ]}
        />
      </div>
    )
  }

  return (
    <div className="flex min-h-dvh flex-col">
      <PageHeader
        back={
          <BackLink to="/subidas">
            <BackLabel>{t('uploads.title')}</BackLabel>
          </BackLink>
        }
        title={t('review.title')}
      />
      <main className="box-border flex-1 px-4 py-8 sm:px-6 lg:px-10">{content}</main>
    </div>
  )
}
