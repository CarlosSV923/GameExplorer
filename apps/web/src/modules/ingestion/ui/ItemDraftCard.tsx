import { useTranslation } from 'react-i18next'

import { itemLabel } from '@/modules/catalog/ui/itemLabel'
import type { DuplicateAction, ItemKind } from '@/modules/catalog/domain/types'
import { useFormat } from '@/shared/i18n/hooks'
import { ChoiceChips, cx, TextField } from '@/shared/ui'

import { draftError, type ItemDraft } from '../domain/review'
import type { CommitPlan, StagedItem } from '../domain/types'

const kinds: readonly ItemKind[] = ['base', 'update', 'dlc', 'disc']

/** One staged item in the review (RF-09): kind, label, final name, duplicate. */
export function ItemDraftCard({
  item,
  draft,
  onChange,
  plan,
  showErrors,
}: {
  item: StagedItem
  draft: ItemDraft
  onChange: (next: ItemDraft) => void
  plan: CommitPlan | undefined
  showErrors: boolean
}) {
  const { t } = useTranslation()
  const format = useFormat()
  const planned = plan?.items.find((p) => p.path === item.path)
  const error = showErrors ? draftError(draft) : null
  const duplicate = !draft.skip ? planned?.duplicate : undefined
  const set = (patch: Partial<ItemDraft>) => {
    onChange({ ...draft, ...patch })
  }

  let hint: string | undefined
  if (item.suggestedKind && draft.kind === item.suggestedKind) {
    hint = item.titleId
      ? t('review.suggestedTitleId', { suffix: `…${item.titleId.slice(-3)}` })
      : t('review.suggested')
  } else if (item.discNumber && draft.kind === 'disc') {
    hint = t('review.suggested')
  }

  const target = planned
    ? `${plan?.console ?? ''}/${plan?.folder ?? ''}/${planned.files[0] ?? ''}`
    : undefined
  const extra = planned && planned.files.length > 1 ? planned.files.length - 1 : 0

  return (
    <article
      className={cx(
        'flex flex-col gap-3.5 rounded-xl border bg-surface p-4.5',
        duplicate ? 'border-accent' : 'border-line',
      )}
    >
      <div className="flex flex-wrap items-start justify-between gap-2">
        <span className="min-w-0 font-mono text-caption [overflow-wrap:anywhere] text-ink-1">
          {item.path}
          {item.shape === 'folder' && (
            <span className="ml-2 text-ink-3">
              {t('review.folderGame', { count: item.parts.length })}
            </span>
          )}
          {item.shape === 'disc' && (
            <span className="ml-2 text-ink-3">
              {t('review.tracks', { count: item.parts.length - 1 })}
            </span>
          )}
        </span>
        <span className="flex items-center gap-4">
          <span className="text-body-sm text-ink-2">{format.size(item.size)}</span>
          <label className="inline-flex min-h-control-sm cursor-pointer items-center gap-2 text-body-sm">
            <input
              type="checkbox"
              checked={draft.skip}
              onChange={(e) => {
                set({ skip: e.target.checked })
              }}
              className="size-4.5 accent-accent"
            />
            {t('review.skipItem')}
          </label>
        </span>
      </div>

      {!draft.skip && (
        <>
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-caption font-bold text-ink-2" aria-hidden="true">
              {t('game.kind')}
            </span>
            <ChoiceChips<ItemKind>
              legend={t('review.kindOf', { name: item.path })}
              name={`kind-${item.path}`}
              value={draft.kind}
              onChange={(kind) => {
                set({ kind })
              }}
              options={kinds.map((k) => ({ value: k, label: t(`kind.${k}`) }))}
              {...(hint ? { hint } : {})}
            />
          </div>
          {error === 'kind' && (
            <span className="text-body-sm text-danger">{t('review.pickKind')}</span>
          )}

          <div className="flex flex-wrap items-end gap-4">
            {draft.kind === 'update' && (
              <div className="flex-[0_1_200px]">
                <TextField
                  label={t('review.version')}
                  value={draft.version}
                  onChange={(e) => {
                    set({ version: e.target.value })
                  }}
                  {...(error === 'version' ? { error: t('common.required') } : {})}
                />
              </div>
            )}
            {draft.kind === 'dlc' && (
              <div className="flex-[0_1_260px]">
                <TextField
                  label={t('review.dlcName')}
                  value={draft.dlcName}
                  onChange={(e) => {
                    set({ dlcName: e.target.value })
                  }}
                  {...(error === 'dlcName' ? { error: t('common.required') } : {})}
                />
              </div>
            )}
            {draft.kind === 'disc' && (
              <div className="flex-[0_1_160px]">
                <TextField
                  label={t('review.discNumber')}
                  inputMode="numeric"
                  value={draft.disc}
                  onChange={(e) => {
                    set({ disc: e.target.value.replace(/\D/g, '') })
                  }}
                  {...(error === 'disc' ? { error: t('review.badDisc') } : {})}
                />
              </div>
            )}
            <div className="flex min-w-0 flex-[1_1_280px] flex-col gap-1.5">
              <span className="text-caption font-bold text-ink-2">{t('review.storedAs')}</span>
              <span
                className={cx(
                  'rounded-md bg-surface-input px-3.5 py-3 font-mono text-caption [overflow-wrap:anywhere]',
                  target ? 'text-success' : 'text-ink-3',
                )}
              >
                {target ?? t('review.storedAsPending')}
                {extra > 0 && ` ${t('review.plusFiles', { count: extra })}`}
              </span>
            </div>
          </div>

          {duplicate && (
            <fieldset className="m-0 flex flex-wrap gap-x-5 gap-y-2 rounded-md border border-accent bg-warning-surface px-3.5 py-3">
              <legend className="px-1.5 text-caption font-bold text-warning-ink">
                {t('review.duplicate', { item: itemLabel(t, duplicate) })}
              </legend>
              {(['replace', 'skip'] as const satisfies readonly DuplicateAction[]).map((v) => (
                <label
                  key={v}
                  className="inline-flex min-h-control-sm cursor-pointer items-center gap-2 text-body-sm"
                >
                  <input
                    type="radio"
                    name={`dup-${item.path}`}
                    checked={draft.onDuplicate === v}
                    onChange={() => {
                      set({ onDuplicate: v })
                    }}
                    className="size-4.5 accent-accent"
                  />
                  {v === 'replace' ? t('review.replace') : t('review.keep')}
                </label>
              ))}
            </fieldset>
          )}
        </>
      )}
    </article>
  )
}
