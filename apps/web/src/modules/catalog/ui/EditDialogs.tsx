import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { linkedId, type NameValue } from '@/modules/metadata/domain/names'
import { NameCombobox } from '@/modules/metadata/ui/NameCombobox'
import { describeError } from '@/shared/i18n/errors'
import { useFormat } from '@/shared/i18n/hooks'
import { isAppError } from '@/shared/kernel/errors'
import { Banner, Button, Dialog, OptionCards, PathPreview } from '@/shared/ui'

import { useEditGame, useEditItem, useEditPlan } from '../application/queries'
import { fileExtension, moveTargets } from '../domain/items'
import { kindDraftError, kindDraftLabel, previewFileName, type KindDraft } from '../domain/naming'
import type {
  Console,
  DuplicateAction,
  GameDetail,
  GameEdit,
  GameEditPlan,
  GameEditResult,
  LibraryItem,
} from '../domain/types'
import { FileTypeFields } from './FileTypeFields'

/** At most this many renamed files are listed in the preview. */
const previewRows = 3

/** "Cambiará": the folder and the first files, before → after. */
function ChangePreview({ game, plan }: { game: GameDetail; plan: GameEditPlan }) {
  const { t } = useTranslation()
  const rows = [
    { from: `${game.path}/`, to: `${plan.console}/${plan.folder}/` },
    ...plan.items
      .filter((p) => p.file !== p.item.file && p.action !== 'skip')
      .map((p) => ({ from: p.item.file, to: p.file })),
  ].filter((r) => r.from !== r.to)
  if (rows.length === 0) {
    return <p className="m-0 text-body-sm text-ink-3">{t('edit.nothing')}</p>
  }
  const extra = rows.length - 1 - previewRows
  return (
    <div className="flex flex-col gap-1.5 rounded-md bg-surface-input px-3.5 py-3">
      <span className="text-caption font-bold text-ink-2">{t('edit.changes')}</span>
      {rows.slice(0, previewRows + 1).map((r) => (
        <span
          key={r.from}
          className="flex flex-wrap items-baseline gap-x-2 font-mono text-caption [overflow-wrap:anywhere]"
        >
          <span className="text-ink-3">{r.from}</span>
          <span aria-hidden="true" className="text-ink-3">
            →
          </span>
          <span className="sr-only">{t('edit.becomes')}</span>
          <span className="text-success">{r.to}</span>
        </span>
      ))}
      {extra > 0 && (
        <span className="text-caption text-ink-3">{t('edit.moreFiles', { count: extra })}</span>
      )}
    </div>
  )
}

/** The collisions of a merge (RF-09): replace the file already there, or skip this one. */
function MergeDecisions({
  plan,
  decisions,
  onChange,
}: {
  plan: GameEditPlan
  decisions: Readonly<Record<number, DuplicateAction>>
  onChange: (itemId: number, action: DuplicateAction) => void
}) {
  const { t } = useTranslation()
  const format = useFormat()
  const clashes = plan.items.filter((p) => p.duplicate)
  return (
    <>
      <Banner>{t('edit.merge', { title: plan.title })}</Banner>
      {clashes.map(({ item, duplicate }) => (
        <fieldset
          key={item.id}
          className="m-0 flex flex-wrap gap-x-5 gap-y-1 rounded-md border border-accent bg-warning-surface px-3.5 py-3"
        >
          <legend className="px-1.5 text-caption font-bold text-warning-ink">
            {t('details.exists', {
              name: duplicate?.file ?? '',
              size: format.size(duplicate?.size ?? 0),
            })}
          </legend>
          {(['replace', 'skip'] as const).map((v) => (
            <label
              key={v}
              className="inline-flex min-h-control-sm cursor-pointer items-center gap-2 text-body-sm font-semibold"
            >
              <input
                type="radio"
                name={`merge-${String(item.id)}`}
                checked={decisions[item.id] === v}
                onChange={() => {
                  onChange(item.id, v)
                }}
                className="size-4.5 accent-accent"
              />
              {v === 'replace' ? t('details.replace') : t('edit.skip', { name: item.file })}
            </label>
          ))}
        </fieldset>
      ))}
    </>
  )
}

/**
 * Rename a game (with an IGDB suggestion or a name of its own) or move it to
 * another console (RF-24). The preview comes from the server; a merge with
 * a game of the same name asks what to do with each collision.
 */
export function EditGameDialog({
  game,
  mode,
  consoles,
  onClose,
  onDone,
}: {
  game: GameDetail
  mode: 'rename' | 'move'
  consoles: readonly Console[]
  onClose: () => void
  onDone: (result: GameEditResult) => void
}) {
  const { t } = useTranslation()
  const current = consoles.find((c) => c.slug === game.console)
  const targets = moveTargets(game.items, game.console, consoles)
  const [name, setName] = useState<NameValue>(() => ({
    text: game.title,
    ...(game.igdbId
      ? { game: { id: game.igdbId, name: game.title, genres: [], platformIds: [] } }
      : {}),
  }))
  const [target, setTarget] = useState(
    () => targets.find((x) => !x.blockedBy)?.console.slug ?? game.console,
  )
  const [decisions, setDecisions] = useState<Record<number, DuplicateAction>>({})
  const editGame = useEditGame(game.id)

  const title = mode === 'rename' ? name.text.trim() : game.title
  const igdbId = mode === 'rename' ? (linkedId(name) ?? null) : (game.igdbId ?? null)
  const console = mode === 'rename' ? game.console : target
  const unchanged =
    console === game.console && title === game.title && igdbId === (game.igdbId ?? null)
  const edit: GameEdit | undefined =
    title && !unchanged
      ? {
          console,
          title,
          igdbId,
          decisions: Object.entries(decisions).map(([itemId, onDuplicate]) => ({
            itemId: Number(itemId),
            onDuplicate,
          })),
        }
      : undefined
  const plan = useEditPlan(game.id, edit)
  const shown = edit && !plan.isPlaceholderData ? plan.data : undefined
  const undecided = shown?.items.some((p) => p.action === 'undecided') ?? false
  const names = new Map(consoles.map((c) => [c.slug, c.displayName]))

  const submit = () => {
    if (!edit || !shown || undecided) return
    editGame.mutate(edit, { onSuccess: onDone })
  }

  const confirmLabel =
    mode === 'rename'
      ? shown?.mergeInto
        ? t('edit.renameMerge')
        : t('edit.rename')
      : t('edit.moveTo', { name: names.get(target) ?? target })

  return (
    <Dialog
      title={mode === 'rename' ? t('edit.renameTitle', { title: game.title }) : t('edit.moveTitle')}
      onClose={onClose}
      initialFocus="first"
      actions={
        <>
          <Button onClick={onClose}>{t('common.cancel')}</Button>
          <Button
            variant="primary"
            loading={editGame.isPending}
            disabled={!shown || undecided || plan.isFetching}
            onClick={submit}
          >
            {confirmLabel}
          </Button>
        </>
      }
    >
      {mode === 'rename' ? (
        <NameCombobox
          label={t('upload.name')}
          platformId={current?.igdbPlatformId}
          value={name}
          onChange={(next) => {
            setName(next)
            setDecisions({})
          }}
          {...(!title ? { error: t('upload.needName') } : {})}
        />
      ) : (
        <>
          <p className="m-0 text-body text-ink-2">{t('edit.moveBody')}</p>
          <OptionCards
            legend={t('edit.target')}
            hideLegend
            name="move-target"
            appearance="row"
            value={target}
            onChange={(slug) => {
              setTarget(slug)
              setDecisions({})
            }}
            options={targets.map(({ console: c, blockedBy }) => ({
              value: c.slug,
              title: c.displayName,
              disabled: Boolean(blockedBy),
              detail: blockedBy
                ? t('edit.rejects', { ext: blockedBy })
                : t('edit.fits', { path: `${c.slug}/${game.folder}/` }),
            }))}
          />
        </>
      )}
      {shown && <ChangePreview game={game} plan={shown} />}
      {shown?.mergeInto && (
        <MergeDecisions
          plan={shown}
          decisions={decisions}
          onChange={(itemId, action) => {
            setDecisions((d) => ({ ...d, [itemId]: action }))
          }}
        />
      )}
      {plan.isError && edit && <Banner tone="danger">{describeError(t, plan.error)}</Banner>}
      {editGame.isError && <Banner tone="danger">{describeError(t, editGame.error)}</Banner>}
    </Dialog>
  )
}

/** Change a Switch file's kind, version or DLC name; it is renamed (RF-24). */
export function EditItemDialog({
  game,
  item,
  console,
  consoles,
  onClose,
}: {
  game: GameDetail
  item: LibraryItem
  console: Console
  consoles: readonly Console[]
  onClose: () => void
}) {
  const { t } = useTranslation()
  const format = useFormat()
  const editItem = useEditItem()
  const [draft, setDraft] = useState<KindDraft>({
    kind: item.kind,
    version: item.kind === 'update' ? (item.label ?? '') : '',
    dlcName: item.kind === 'dlc' ? (item.label ?? '') : '',
  })
  const [tried, setTried] = useState(false)
  const error = kindDraftError(draft)
  const ext = fileExtension(item.file, consoles) || item.file.slice(item.file.lastIndexOf('.'))
  const preview = previewFileName(game.title, draft, ext)
  const taken = isAppError(editItem.error, 'conflict')

  const save = (replace: boolean) => {
    setTried(true)
    if (error || !draft.kind) return
    const label = kindDraftLabel(draft)
    editItem.mutate(
      {
        id: item.id,
        edit: {
          kind: draft.kind,
          ...(label ? { label } : {}),
          ...(replace ? { onDuplicate: 'replace' as const } : {}),
        },
      },
      { onSuccess: onClose },
    )
  }

  return (
    <Dialog
      title={t('edit.itemTitle')}
      onClose={onClose}
      initialFocus="first"
      actions={
        <>
          <Button onClick={onClose}>{t('common.cancel')}</Button>
          <Button
            variant="primary"
            loading={editItem.isPending && !taken}
            disabled={preview.name === item.file}
            onClick={() => {
              save(false)
            }}
          >
            {t('edit.saveItem')}
          </Button>
        </>
      }
    >
      <span className="font-mono text-caption [overflow-wrap:anywhere] text-ink-2">
        {t('edit.itemMeta', { name: item.file, size: format.size(item.size) })}
      </span>
      <FileTypeFields
        name={`edit-${String(item.id)}`}
        fileName={item.file}
        kinds={console.kinds}
        value={draft}
        onChange={(next) => {
          setDraft(next)
          editItem.reset()
        }}
        error={tried ? error : null}
        preview={
          <PathPreview label={t('edit.renamedTo')} complete={preview.complete}>
            {preview.name}
          </PathPreview>
        }
      />
      {taken && (
        <Banner
          action={
            <Button
              size="sm"
              variant="primary"
              loading={editItem.isPending}
              onClick={() => {
                save(true)
              }}
            >
              {t('details.replace')}
            </Button>
          }
        >
          {t('edit.itemTaken', { name: preview.name })}
        </Banner>
      )}
      {editItem.isError && !taken && (
        <Banner tone="danger">{describeError(t, editItem.error)}</Banner>
      )}
    </Dialog>
  )
}
