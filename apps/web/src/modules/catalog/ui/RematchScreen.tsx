import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import type { MetadataGame } from '@/modules/metadata/domain/types'
import { IgdbGamePicker } from '@/modules/metadata/ui/IgdbGamePicker'
import { describeError } from '@/shared/i18n/errors'
import { useAction } from '@/shared/input'
import { useDelayedFlag } from '@/shared/kernel/hooks'
import { BackLink, ButtonLink } from '@/shared/routing/links'
import {
  BackLabel,
  Banner,
  Button,
  EmptyState,
  HelpBar,
  PageHeader,
  Spinner,
  TableBox,
} from '@/shared/ui'

import {
  useConsoleGames,
  useConsoles,
  useGame,
  useRematch,
  useRematchPlan,
} from '../application/queries'
import type { DuplicateAction, RematchPlan } from '../domain/types'
import { ItemChip } from './ItemChip'

function DecisionRadios({
  name,
  value,
  target,
  onChange,
}: {
  name: string
  value: DuplicateAction
  target: string
  onChange: (v: DuplicateAction) => void
}) {
  const { t } = useTranslation()
  return (
    <fieldset className="m-0 flex flex-col gap-0.5 border-0 p-0">
      <legend className="sr-only">{t('rematch.duplicate')}</legend>
      {(['replace', 'skip'] as const).map((v) => (
        <label
          key={v}
          className="inline-flex min-h-control-sm cursor-pointer items-center gap-2 text-body-sm"
        >
          <input
            type="radio"
            name={name}
            checked={value === v}
            onChange={() => {
              onChange(v)
            }}
            className="size-4.5 accent-accent"
          />
          {v === 'replace' ? t('rematch.replace', { title: target }) : t('rematch.skip')}
        </label>
      ))}
    </fieldset>
  )
}

function PlanView({
  plan,
  oldFolder,
  decisions,
  onDecide,
}: {
  plan: RematchPlan
  oldFolder: string
  decisions: Readonly<Record<number, DuplicateAction>>
  onDecide: (itemId: number, v: DuplicateAction) => void
}) {
  const { t } = useTranslation()
  const merge = plan.mergeInto != null
  return (
    <>
      {merge ? (
        <Banner>
          {t('rematch.mergeBanner', {
            title: plan.title,
            path: `${plan.console}/${plan.folder}/`,
          })}
        </Banner>
      ) : (
        <div className="flex flex-col gap-1.5 rounded-lg border border-line bg-surface px-4.5 py-4">
          <span className="text-caption font-bold tracking-[0.06em] text-ink-3 uppercase">
            {t('rematch.folder')}
          </span>
          {oldFolder !== plan.folder && (
            <span className="font-mono text-body-sm text-ink-2 line-through">
              {plan.console}/{oldFolder}/
            </span>
          )}
          <span className="font-mono text-body-sm text-ink-1">
            {plan.console}/{plan.folder}/
          </span>
          <span className="text-caption text-ink-2">{t('rematch.smbFollow')}</span>
        </div>
      )}
      <TableBox>
        <table className="w-full min-w-[640px] border-collapse">
          <thead>
            <tr className="text-left text-caption font-bold tracking-[0.06em] text-ink-3 uppercase">
              <th className="px-4 py-3">{t('game.kind')}</th>
              <th className="px-4 py-3">{t('rematch.newName')}</th>
              <th className="px-4 py-3">{merge ? t('rematch.decision') : t('rematch.what')}</th>
            </tr>
          </thead>
          <tbody className="text-body-sm">
            {plan.items.map((row) => (
              <tr key={row.item.id} className="border-t border-line align-top">
                <td className="px-4 py-3">
                  <ItemChip item={row.item} />
                </td>
                <td className="px-4 py-3">
                  <span className="flex flex-col gap-1 font-mono [overflow-wrap:anywhere]">
                    <span className="text-caption text-ink-1">{row.files[0]}</span>
                    <span className="text-chip text-ink-3">
                      {t('rematch.before', { name: row.item.files[0] ?? '' })}
                    </span>
                  </span>
                </td>
                <td className="px-4 py-1.5">
                  {row.duplicate ? (
                    <DecisionRadios
                      name={`dup-${String(row.item.id)}`}
                      value={decisions[row.item.id] ?? 'skip'}
                      target={plan.title}
                      onChange={(v) => {
                        onDecide(row.item.id, v)
                      }}
                    />
                  ) : (
                    <span className="inline-flex min-h-control-sm items-center text-success">
                      {merge ? t('rematch.moves') : t('rematch.renames')}
                    </span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </TableBox>
    </>
  )
}

/** Match a game to another IGDB entry: rename or merge (RF-24). */
export function RematchScreen({ slug, gameId }: { slug: string; gameId: number }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const consoles = useConsoles()
  const game = useGame(gameId)
  const games = useConsoleGames(slug)
  const slow = useDelayedFlag(game.isPending)
  const [pick, setPick] = useState<MetadataGame | undefined>()
  const [decisions, setDecisions] = useState<Record<number, DuplicateAction>>({})
  const plan = useRematchPlan(gameId, pick?.id, decisions)
  const rematch = useRematch(gameId)

  const info = consoles.data?.find((c) => c.slug === slug)
  const g = game.data
  const same = pick !== undefined && g !== undefined && pick.id === g.igdbId
  const owned = new Set((games.data ?? []).filter((x) => x.id !== gameId).map((x) => x.igdbId))
  const canConfirm = Boolean(pick && plan.data && !plan.isPlaceholderData && !same)

  const back = () => {
    void navigate({ to: '/consolas/$slug/$gameId', params: { slug, gameId: String(gameId) } })
  }
  const confirm = () => {
    if (!pick || !canConfirm) return
    // Every duplicate needs a decision: the shown default counts.
    const all = { ...decisions }
    for (const row of plan.data?.items ?? []) {
      if (row.duplicate && !(row.item.id in all)) all[row.item.id] = 'skip'
    }
    rematch.mutate(
      { igdbGameId: pick.id, decisions: all },
      {
        onSuccess: (r) => {
          void navigate({
            to: '/consolas/$slug/$gameId',
            params: { slug, gameId: String(r.gameId) },
            replace: true,
          })
        },
      },
    )
  }

  useAction('back', () => {
    back()
    return true
  })
  useAction('action1', () => {
    confirm()
    return true
  })

  if (!g) {
    return (
      <main className="p-10">
        {game.isError ? (
          <EmptyState title={t('game.goneTitle')} body={t('game.goneBody')} />
        ) : (
          slow && <Spinner label={t('common.loading')} />
        )}
      </main>
    )
  }

  const merge = plan.data?.mergeInto != null

  return (
    <div className="flex min-h-dvh flex-col">
      <PageHeader
        back={
          <BackLink to="/consolas/$slug/$gameId" params={{ slug, gameId: String(gameId) }}>
            <BackLabel>{g.title}</BackLabel>
          </BackLink>
        }
        title={t('rematch.title')}
        aside={
          <span className="font-mono text-body-sm text-ink-2">
            {t('rematch.where', { path: `${g.path}/`, count: g.items.length })}
          </span>
        }
      />
      <main className="box-border flex flex-1 flex-wrap items-start gap-8 px-4 pt-7 pb-10 sm:px-6 lg:px-10">
        <section className="flex min-w-0 flex-[1_1_380px] flex-col gap-3.5 lg:max-w-[520px]">
          <h2 className="m-0 text-body-sm font-bold tracking-label text-ink-2 uppercase">
            {t('rematch.correct')}
          </h2>
          <IgdbGamePicker
            platformId={info?.igdbPlatformId ?? undefined}
            platformName={info?.displayName ?? slug}
            initialQuery={g.title}
            value={pick}
            onChange={(next) => {
              setPick(next)
              setDecisions({})
            }}
            owned={owned}
          />
        </section>

        <section className="flex min-w-0 flex-[999_1_560px] flex-col gap-4">
          <h2 className="m-0 text-body-sm font-bold tracking-label text-ink-2 uppercase">
            {t('rematch.result')}
          </h2>
          {!pick && <p className="m-0 text-body text-ink-3">{t('rematch.pickFirst')}</p>}
          {same && <Banner tone="info">{t('rematch.same')}</Banner>}
          {pick && plan.isError && <Banner tone="danger">{describeError(t, plan.error)}</Banner>}
          {pick && !same && plan.data && (
            <PlanView
              plan={plan.data}
              oldFolder={g.folder}
              decisions={decisions}
              onDecide={(itemId, v) => {
                setDecisions((d) => ({ ...d, [itemId]: v }))
              }}
            />
          )}
          {rematch.isError && <Banner tone="danger">{describeError(t, rematch.error)}</Banner>}
          <div className="mt-2 flex flex-wrap justify-end gap-3">
            <ButtonLink
              to="/consolas/$slug/$gameId"
              params={{ slug, gameId: String(gameId) }}
              size="lg"
            >
              {t('common.cancel')}
            </ButtonLink>
            <Button
              variant="primary"
              size="lg"
              disabled={!canConfirm}
              loading={rematch.isPending}
              onClick={confirm}
            >
              {merge ? t('rematch.confirmMerge') : t('rematch.confirm')}
            </Button>
          </div>
        </section>
      </main>
      <HelpBar
        actions={[
          { glyph: 'A', label: t('help.select') },
          { glyph: 'B', label: t('common.cancel') },
          { glyph: 'X', label: t('common.confirm') },
        ]}
      />
    </div>
  )
}
