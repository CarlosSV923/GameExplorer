import { useId, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { ChoiceChips, cx, ErrorIcon } from '@/shared/ui'

import { cleanVersion, type KindDraft, type KindDraftError } from '../domain/naming'
import type { ItemKind } from '../domain/types'

/** The kind colors of the chips (Chip): base white, update blue, DLC amber. */
const kindColors: Record<ItemKind, string> = {
  base: 'bg-kind-base text-on-accent',
  update: 'bg-kind-update text-on-kind-update',
  dlc: 'bg-kind-dlc text-on-accent',
  game: 'bg-kind-game text-on-accent',
  disc: 'bg-kind-game text-on-accent',
}

function FieldError({ id, children }: { id: string; children: ReactNode }) {
  return (
    <span id={id} className="flex items-start gap-2 text-body-sm font-bold text-danger">
      <ErrorIcon className="mt-px shrink-0" />
      {children}
    </span>
  )
}

/**
 * A Switch file's data (FileTypeChips + VersionField, docs/design-handoff.md
 * §9): its kind, the update's version or the DLC's name, and the final name
 * next to them.
 */
export function FileTypeFields({
  name,
  fileName,
  kinds,
  value,
  onChange,
  error,
  preview,
}: {
  /** Radio group name (unique per file). */
  name: string
  /** The file, for the group's accessible name. */
  fileName: string
  kinds: readonly ItemKind[]
  value: KindDraft
  onChange: (next: KindDraft) => void
  /** Shown once the user tried to save. */
  error: KindDraftError | null
  /** The PathPreview with the final name. */
  preview: ReactNode
}) {
  const { t } = useTranslation()
  const id = useId()
  const set = (patch: Partial<KindDraft>) => {
    onChange({ ...value, ...patch })
  }
  // A console with discs chooses between the whole game and one of its discs.
  const choices = kinds.filter((k) => k !== 'game' || kinds.includes('disc'))

  return (
    <>
      <div className="flex flex-wrap items-center gap-2">
        <span aria-hidden="true" className="mr-2 text-caption font-bold text-ink-2">
          {t('game.kind')}
        </span>
        <ChoiceChips<ItemKind>
          legend={t('details.kindOf', { name: fileName })}
          name={name}
          value={value.kind}
          onChange={(kind) => {
            set({ kind })
          }}
          options={choices.map((k) => ({
            value: k,
            label: t(`kind.${k}Long`),
            checkedClass: kindColors[k],
          }))}
        />
      </div>
      {error === 'kind' && <FieldError id={`${id}-kind`}>{t('details.pickKind')}</FieldError>}

      <div className="flex flex-wrap items-start gap-4">
        {value.kind === 'update' && (
          <div className="flex flex-[0_1_220px] flex-col gap-1.5">
            <label htmlFor={`${id}-version`} className="text-caption font-bold text-ink-2">
              {t('details.version')}{' '}
              <span className="font-semibold text-ink-3">{t('details.required')}</span>
            </label>
            <div
              className={cx(
                'flex h-control-sm items-center overflow-hidden rounded-md border bg-surface-input focus-within:outline-3 focus-within:outline-offset-2 focus-within:outline-accent',
                error === 'version' ? 'border-danger' : 'border-control',
              )}
            >
              <span aria-hidden="true" className="pr-1 pl-3.5 font-mono text-body text-ink-3">
                {t('details.versionPrefix')}
              </span>
              <input
                id={`${id}-version`}
                type="text"
                inputMode="decimal"
                placeholder="1.0.4"
                autoComplete="off"
                value={value.version}
                aria-invalid={error === 'version' ? true : undefined}
                aria-describedby={error === 'version' ? `${id}-version-error` : undefined}
                onChange={(e) => {
                  set({ version: cleanVersion(e.target.value) })
                }}
                className="h-full min-w-0 flex-1 border-0 bg-transparent pr-3.5 font-mono text-body text-ink-1 outline-0 placeholder:text-ink-3"
              />
            </div>
            {error === 'version' && (
              <FieldError id={`${id}-version-error`}>{t('details.needVersion')}</FieldError>
            )}
          </div>
        )}
        {value.kind === 'dlc' && (
          <div className="flex flex-[0_1_280px] flex-col gap-1.5">
            <label htmlFor={`${id}-dlc`} className="text-caption font-bold text-ink-2">
              {t('details.dlcName')}{' '}
              <span className="font-semibold text-ink-3">{t('details.required')}</span>
            </label>
            <input
              id={`${id}-dlc`}
              type="text"
              autoComplete="off"
              value={value.dlcName}
              aria-invalid={error === 'dlcName' ? true : undefined}
              aria-describedby={error === 'dlcName' ? `${id}-dlc-error` : undefined}
              onChange={(e) => {
                set({ dlcName: e.target.value })
              }}
              className={cx(
                'box-border h-control-sm rounded-md border bg-surface-input px-3.5 text-body text-ink-1',
                error === 'dlcName' ? 'border-danger' : 'border-control',
              )}
            />
            {error === 'dlcName' && (
              <FieldError id={`${id}-dlc-error`}>{t('details.needDlcName')}</FieldError>
            )}
          </div>
        )}
        {value.kind === 'disc' && (
          <DiscField
            value={value.disc ?? ''}
            onChange={(disc) => {
              set({ disc })
            }}
            error={error === 'disc'}
          />
        )}
        <div className="min-w-0 flex-[1_1_280px]">{preview}</div>
      </div>
    </>
  )
}

/** A disc's number, 1 to 99 (GameCube, PS2; spec §5). */
export function DiscField({
  value,
  onChange,
  error,
  label,
}: {
  value: string
  onChange: (next: string) => void
  /** Shown once the user tried to save. */
  error: boolean
  /** Overrides "Número de disco" (e.g. for the disc already stored). */
  label?: string
}) {
  const { t } = useTranslation()
  const id = useId()
  return (
    <div className="flex flex-[0_1_180px] flex-col gap-1.5">
      <label htmlFor={`${id}-disc`} className="text-caption font-bold text-ink-2">
        {label ?? t('details.disc')}{' '}
        <span className="font-semibold text-ink-3">{t('details.required')}</span>
      </label>
      <input
        id={`${id}-disc`}
        type="text"
        inputMode="numeric"
        autoComplete="off"
        maxLength={2}
        value={value}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${id}-disc-error` : undefined}
        onChange={(e) => {
          onChange(e.target.value.replace(/[^0-9]/g, ''))
        }}
        className={cx(
          'box-border h-control-sm w-24 rounded-md border bg-surface-input px-3.5 font-mono text-body text-ink-1',
          error ? 'border-danger' : 'border-control',
        )}
      />
      {error && <FieldError id={`${id}-disc-error`}>{t('details.needDisc')}</FieldError>}
    </div>
  )
}
