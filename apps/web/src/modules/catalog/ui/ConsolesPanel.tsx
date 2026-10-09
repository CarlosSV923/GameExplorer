import { useState, type SubmitEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { describeError } from '@/shared/i18n/errors'
import {
  Banner,
  Button,
  CheckIcon,
  ChevronDownIcon,
  ChevronUpIcon,
  CloseIcon,
  cx,
  IconButton,
  LockIcon,
  PlusIcon,
  Spinner,
  TextField,
} from '@/shared/ui'

import {
  useAddExtension,
  useConsoles,
  useRemoveExtension,
  useRenameConsole,
  useReorderConsoles,
} from '../application/queries'
import { singleFile } from '../domain/items'
import { normalizeExtension } from '../domain/naming'
import type { Console } from '../domain/types'

/** The display name, saved on leaving the field or with Enter (RF-42). */
function NameField({ console: c }: { console: Console }) {
  const { t } = useTranslation()
  const rename = useRenameConsole()
  const [name, setName] = useState(c.displayName)
  const [saved, setSaved] = useState(false)
  const clean = name.trim()
  const invalid = clean.length === 0 || clean.length > 60

  const save = () => {
    if (invalid || clean === c.displayName) return
    rename.mutate(
      { slug: c.slug, name: clean },
      {
        onSuccess: () => {
          setSaved(true)
        },
      },
    )
  }

  let hint = t('consoles.defaultName', { name: c.defaultName })
  if (saved && !rename.isPending) hint = t('consoles.saved')
  return (
    <form
      className="min-w-0 flex-[1_1_260px]"
      onSubmit={(e) => {
        e.preventDefault()
        save()
      }}
    >
      <TextField
        label={t('consoles.name')}
        value={name}
        maxLength={60}
        hint={hint}
        onChange={(e) => {
          setName(e.target.value)
          setSaved(false)
        }}
        onBlur={save}
        {...(invalid
          ? { error: t('consoles.badName') }
          : rename.isError
            ? { error: describeError(t, rename.error) }
            : {})}
      />
    </form>
  )
}

/** Fixed and custom extensions, and adding one (RF-41). */
function Extensions({ console: c }: { console: Console }) {
  const { t } = useTranslation()
  const add = useAddExtension()
  const remove = useRemoveExtension()
  const [text, setText] = useState('')
  const [invalid, setInvalid] = useState<string | null>(null)
  const inUse = c.customExtensions.filter((e) => e.fileCount > 0)

  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    const ext = normalizeExtension(text)
    if (!ext) {
      setInvalid(t('consoles.badExtension'))
      return
    }
    if (c.extensions.includes(ext) || c.customExtensions.some((x) => x.extension === ext)) {
      setInvalid(t('consoles.extensionExists', { ext }))
      return
    }
    add.mutate(
      { slug: c.slug, extension: ext },
      {
        onSuccess: () => {
          setText('')
        },
      },
    )
  }

  const chip = 'inline-flex h-control-sm items-center gap-1.5 rounded-sm font-mono text-caption'
  return (
    <div className="flex flex-col gap-3">
      <span className="text-caption font-bold text-ink-2">{t('consoles.extensions')}</span>
      <ul
        aria-label={t('consoles.extensionsOf', { name: c.displayName })}
        className="m-0 flex list-none flex-wrap gap-2 p-0"
      >
        {c.extensions.map((ext) => (
          <li key={ext} className={cx(chip, 'bg-surface-raised px-3 text-ink-1')}>
            <LockIcon size={14} className="text-ink-3" />
            {ext}
            <span className="sr-only">{t('consoles.fixed')}</span>
          </li>
        ))}
        {c.customExtensions.map(({ extension, fileCount }) => (
          <li key={extension} className={cx(chip, 'border border-control pl-3 text-ink-1')}>
            {extension}
            <button
              type="button"
              aria-label={
                fileCount > 0
                  ? t('consoles.removeBlocked', { ext: extension, count: fileCount })
                  : t('consoles.removeExtension', { ext: extension })
              }
              aria-disabled={fileCount > 0 || undefined}
              title={
                fileCount > 0
                  ? t('consoles.inUse', { count: fileCount })
                  : t('consoles.removeExtension', { ext: extension })
              }
              onClick={() => {
                if (fileCount === 0) remove.mutate({ slug: c.slug, extension })
              }}
              className={cx(
                'inline-flex size-control-sm items-center justify-center rounded-sm border-0 bg-transparent',
                fileCount > 0
                  ? 'cursor-default text-ink-3'
                  : 'cursor-pointer text-ink-2 hover:text-ink-1',
              )}
            >
              <CloseIcon size={14} />
            </button>
          </li>
        ))}
      </ul>
      {inUse.map((e) => (
        <span key={e.extension} className="text-caption text-ink-3">
          {t('consoles.cannotRemove', { ext: e.extension, count: e.fileCount })}
        </span>
      ))}
      {remove.isError && <Banner tone="danger">{describeError(t, remove.error)}</Banner>}
      <form onSubmit={submit} noValidate className="flex flex-wrap items-start gap-3">
        <div className="min-w-40 flex-[0_1_220px]">
          <TextField
            label={t('consoles.newExtension', { name: c.displayName })}
            hideLabel
            placeholder=".ext"
            className="font-mono"
            value={text}
            onChange={(e) => {
              setText(e.target.value)
              setInvalid(null)
              add.reset()
            }}
            {...(invalid
              ? { error: invalid }
              : add.isError
                ? { error: describeError(t, add.error, { conflict: t('consoles.extensionTaken') }) }
                : {})}
          />
        </div>
        <Button type="submit" icon={<PlusIcon />} loading={add.isPending}>
          {t('consoles.addExtension')}
        </Button>
      </form>
      <span className="text-caption text-ink-3">{t('consoles.fixedHint')}</span>
    </div>
  )
}

/** Settings › Consoles: order, display names and extensions (RF-40 to RF-42). */
export function ConsolesPanel() {
  const { t } = useTranslation()
  const consoles = useConsoles()
  const reorder = useReorderConsoles()
  const list = consoles.data ?? []

  const move = (index: number, by: -1 | 1) => {
    const slugs = list.map((c) => c.slug)
    const [slug] = slugs.splice(index, 1)
    if (slug === undefined) return
    slugs.splice(index + by, 0, slug)
    reorder.mutate(slugs)
  }

  return (
    <div className="flex max-w-[1100px] flex-col gap-5">
      <div className="flex flex-col gap-1.5">
        <h2 className="m-0 text-body-sm font-bold tracking-label text-ink-2 uppercase">
          {t('consoles.count', { count: list.length })}
        </h2>
        <span className="text-body-sm text-ink-3">{t('consoles.intro')}</span>
      </div>
      {consoles.isPending && <Spinner label={t('common.loading')} />}
      {reorder.isError && <Banner tone="danger">{describeError(t, reorder.error)}</Banner>}
      {list.map((c, i) => (
        <section
          key={c.slug}
          aria-label={c.displayName}
          className="flex flex-wrap items-start gap-x-8 gap-y-5 rounded-xl border border-line bg-surface p-5"
        >
          <div className="flex flex-col">
            <IconButton
              label={t('consoles.moveUp', { name: c.displayName })}
              disabled={i === 0 || reorder.isPending}
              onClick={() => {
                move(i, -1)
              }}
            >
              <ChevronUpIcon size={16} />
            </IconButton>
            <IconButton
              label={t('consoles.moveDown', { name: c.displayName })}
              disabled={i === list.length - 1 || reorder.isPending}
              onClick={() => {
                move(i, 1)
              }}
            >
              <ChevronDownIcon size={16} />
            </IconButton>
          </div>
          <div className="flex min-w-0 flex-[1_1_300px] flex-col gap-3">
            <NameField key={c.displayName} console={c} />
            <span className="flex flex-wrap gap-x-4 gap-y-1 text-body-sm text-ink-2">
              <span>
                {t('consoles.folder')}{' '}
                <span className="font-mono text-caption text-ink-1">{c.slug}/</span>{' '}
                {t('consoles.fixedFolder')}
              </span>
              <span>{t('home.games', { count: c.gameCount })}</span>
              <span className="inline-flex items-center gap-1.5">
                <CheckIcon size={14} className="text-success" />
                {singleFile(c) ? t('consoles.oneFile') : t('consoles.addOns')}
              </span>
            </span>
          </div>
          <div className="min-w-0 flex-[2_1_360px]">
            <Extensions console={c} />
          </div>
        </section>
      ))}
    </div>
  )
}
