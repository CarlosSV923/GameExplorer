import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useConsoles } from '@/modules/catalog/application/queries'
import { consoleExtensions, singleFile } from '@/modules/catalog/domain/items'
import { gameFolder } from '@/modules/catalog/domain/naming'
import { linkedId, type NameValue } from '@/modules/metadata/domain/names'
import { NameCombobox } from '@/modules/metadata/ui/NameCombobox'
import { describeError } from '@/shared/i18n/errors'
import { useFormat } from '@/shared/i18n/hooks'
import { useAction } from '@/shared/input'
import {
  Banner,
  Button,
  CloseIcon,
  cx,
  Dialog,
  ErrorIcon,
  FileIcon,
  FolderIcon,
  IconButton,
  OptionCards,
  PathPreview,
  UploadIcon,
} from '@/shared/ui'

import { useAssign, useUploadQueue, type UploadFormRequest } from '../application/queries'
import type { UploadSpec } from '../domain/types'
import {
  checkFile,
  defaultMode,
  specsFor,
  type FormFile,
  type UploadMode,
} from '../domain/uploadForm'

interface Entry extends FormFile {
  key: string
  file?: File
}

function entriesOf(request: UploadFormRequest): Entry[] {
  if (request.kind === 'unassigned') {
    const f = request.file
    return [{ key: `u${String(f.id)}`, name: f.name, size: f.size, archive: f.archive }]
  }
  return request.files.map((file, i) => ({
    key: `${String(i)}-${file.name}`,
    name: file.name,
    size: file.size,
    file,
  }))
}

const modes: readonly UploadMode[] = ['parts', 'same', 'separate']

/**
 * The upload form (RF-03, RF-03a, RF-27): the game's name (IGDB suggestions
 * or free text) and console, before anything is sent. Files that are not
 * archives must fit the console. "Juegos distintos" asks once per file.
 */
export function UploadFormDialog({
  request,
  onClose,
  onDone,
}: {
  request: UploadFormRequest
  onClose: () => void
  /** Everything was queued (or assigned): show the uploads. */
  onDone: () => void
}) {
  const { t } = useTranslation()
  const format = useFormat()
  const queue = useUploadQueue()
  const assign = useAssign()
  const consoles = useConsoles()
  const [entries, setEntries] = useState(() => entriesOf(request))
  const [mode, setMode] = useState<UploadMode>(() => defaultMode(entries.map((e) => e.name)))
  const [step, setStep] = useState(0)
  const [slug, setSlug] = useState(request.kind === 'files' ? (request.console ?? '') : '')
  const [name, setName] = useState<NameValue>({ text: '' })
  const [tried, setTried] = useState(false)

  const all = consoles.data ?? []
  const target = all.find((c) => c.slug === slug)
  const multi = entries.length > 1
  const separate = multi && mode === 'separate'
  // In "juegos distintos" the form is about one file at a time.
  const current = separate ? entries.slice(step, step + 1) : entries
  const checks = current.map((e) => checkFile(e, target, all))
  const badFiles = checks.some((c) => c === 'invalid')
  const folder = gameFolder(name.text)
  const nameError = !folder.ok ? t('upload.needName') : undefined
  const blocked = !target || !folder.ok || badFiles || current.length === 0
  const last = !separate || step === entries.length - 1

  const remove = (key: string) => {
    const next = entries.filter((e) => e.key !== key)
    if (next.length === 0) {
      onClose()
      return
    }
    setEntries(next)
    setStep((s) => Math.min(s, next.length - 1))
  }

  const submit = () => {
    setTried(true)
    if (blocked) return
    const igdbId = linkedId(name)
    const spec: UploadSpec = {
      console: slug,
      title: name.text.trim(),
      ...(igdbId === undefined ? {} : { igdbId }),
    }
    if (request.kind === 'unassigned') {
      assign.mutate({ id: request.file.id, spec }, { onSuccess: onDone })
      return
    }
    const files = current.flatMap((e) => (e.file ? [e.file] : []))
    const specs = specsFor(files.length, mode === 'parts' ? 'parts' : 'same', spec, () =>
      crypto.randomUUID(),
    )
    queue.add(files.map((file, i) => ({ file, spec: specs[i] ?? spec })))
    if (last) {
      onDone()
      return
    }
    setStep(step + 1)
    setName({ text: '' })
    setTried(false)
  }

  // X confirms the form (docs/design-handoff.md §9).
  useAction('action1', () => {
    submit()
    return true
  })

  let title: string
  if (request.kind === 'unassigned') title = t('upload.assignTitle', { name: request.file.name })
  else if (multi) title = t('upload.titleMany', { count: entries.length })
  else title = t('upload.title')

  let submitLabel: string
  if (request.kind === 'unassigned') submitLabel = t('upload.assign')
  else if (separate && !last) submitLabel = t('upload.submitNext')
  else if (current.length > 1) submitLabel = t('upload.submitMany', { count: current.length })
  else submitLabel = t('upload.submit')

  const afterHint = target
    ? singleFile(target)
      ? t('upload.afterOne', { exts: consoleExtensions(target).join(' ') })
      : t('upload.afterKinds')
    : undefined

  return (
    <Dialog
      size="lg"
      title={title}
      subtitle={request.kind === 'unassigned' ? t('upload.assignSubtitle') : t('upload.subtitle')}
      onClose={onClose}
      initialFocus="field"
      actions={
        <>
          <Button size="lg" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button
            size="lg"
            variant="primary"
            icon={<UploadIcon />}
            loading={assign.isPending}
            disabled={tried && blocked}
            onClick={submit}
          >
            {submitLabel}
          </Button>
        </>
      }
    >
      <ul aria-label={t('upload.files')} className="m-0 flex list-none flex-col gap-2 p-0">
        {entries.map((e, i) => {
          const inStep = !separate || i === step
          const check = inStep ? checks[separate ? 0 : i] : undefined
          return (
            <li
              key={e.key}
              className={cx(
                'flex flex-col gap-2 rounded-lg border bg-surface-card py-1.5 pr-1.5 pl-4',
                check === 'invalid' ? 'border-danger' : 'border-line',
                separate && !inStep && 'opacity-60',
              )}
            >
              <div className="flex items-center gap-3">
                <FileIcon className="shrink-0 text-ink-2" />
                <span
                  className="min-w-0 flex-1 truncate font-mono text-caption text-ink-1"
                  title={e.name}
                >
                  {e.name}
                </span>
                <span className="shrink-0 text-caption font-semibold text-ink-2">
                  {format.size(e.size)}
                </span>
                {request.kind === 'files' && (!separate || i >= step) ? (
                  <IconButton
                    label={t('upload.removeFile', { name: e.name })}
                    className="text-ink-2"
                    onClick={() => {
                      remove(e.key)
                    }}
                  >
                    <CloseIcon size={16} />
                  </IconButton>
                ) : (
                  <span className="size-control-sm shrink-0" />
                )}
              </div>
              {check === 'invalid' && target && (
                <span
                  role="alert"
                  className="flex items-start gap-2 pr-2 pb-1.5 text-body-sm leading-snug font-semibold text-danger"
                >
                  <ErrorIcon className="mt-0.5 shrink-0" />
                  {t('upload.badFile', {
                    console: target.displayName,
                    exts: consoleExtensions(target).join(', '),
                  })}
                </span>
              )}
            </li>
          )
        })}
      </ul>

      {multi && request.kind === 'files' && step === 0 && (
        <OptionCards<UploadMode>
          legend={t('upload.modeLegend')}
          name="upload-mode"
          appearance="row"
          value={mode}
          onChange={setMode}
          options={modes.map((m) => ({
            value: m,
            title: t(`upload.mode.${m}`),
            detail: t(`upload.mode.${m}Hint`),
          }))}
        />
      )}

      {separate && (
        <div
          role="status"
          className="flex flex-wrap items-center justify-between gap-3 rounded-lg bg-surface-card px-4 py-3 text-body font-bold"
        >
          <span>
            {t('upload.step', { n: step + 1, total: entries.length })}{' '}
            <span className="font-mono text-caption font-medium">{entries[step]?.name}</span>
          </span>
          {!last && <span className="text-caption text-ink-2">{t('upload.stepHint')}</span>}
        </div>
      )}

      <OptionCards
        legend={t('upload.console')}
        name="upload-console"
        value={slug || undefined}
        onChange={(next) => {
          setSlug(next)
        }}
        options={all.map((c) => ({
          value: c.slug,
          title: c.displayName,
          detail: consoleExtensions(c).join(' '),
        }))}
        hint={
          <>
            {request.kind === 'files' && request.console && (
              <span className="inline-flex items-center gap-2 text-body-sm font-semibold text-ink-2">
                <FolderIcon size={16} />
                {t('upload.preselected')}
              </span>
            )}
            {tried && !target && (
              <span className="flex items-start gap-2 text-body-sm text-danger">
                <ErrorIcon className="mt-px shrink-0" />
                {t('upload.needConsole')}
              </span>
            )}
          </>
        }
      />

      <NameCombobox
        key={step}
        label={t('upload.name')}
        platformId={target?.igdbPlatformId}
        value={name}
        onChange={setName}
        {...(tried && nameError ? { error: nameError } : {})}
      />

      <div className="flex flex-col gap-1.5">
        <PathPreview label={t('upload.storedIn')} complete={Boolean(target) && folder.ok}>
          {`${slug || '…'}/${folder.ok ? folder.name : '…'}/`}
        </PathPreview>
        {afterHint && <span className="text-caption font-semibold text-ink-3">{afterHint}</span>}
      </div>

      {assign.isError && <Banner tone="danger">{describeError(t, assign.error)}</Banner>}
    </Dialog>
  )
}
