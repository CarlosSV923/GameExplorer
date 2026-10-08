import { useState, type SubmitEvent } from 'react'
import { useTranslation } from 'react-i18next'

import type { MetadataPlatform } from '@/modules/metadata/domain/types'
import { minQuery, usePlatformSearch } from '@/modules/metadata/application/queries'
import { ConsoleLogo } from '@/modules/metadata/ui/Cover'
import { IgdbError } from '@/modules/metadata/ui/IgdbError'
import { describeError } from '@/shared/i18n/errors'
import { isAppError } from '@/shared/kernel/errors'
import {
  Banner,
  Button,
  ChevronDownIcon,
  ChevronUpIcon,
  ChoiceList,
  ConfirmDialog,
  cx,
  Dialog,
  EditIcon,
  IconButton,
  Spinner,
  TableBox,
  TextField,
  TrashIcon,
} from '@/shared/ui'

import {
  useConsoles,
  useCreateConsole,
  useDeleteConsole,
  useReorderConsoles,
  useUpdateConsole,
} from '../application/queries'
import { slugPattern, suggestSlug } from '../domain/items'
import type { Console, Detection } from '../domain/types'
import { ExtensionsField } from './ExtensionsField'

const detectionKeys: Record<Detection, `consoles.detection.${Detection}`> = {
  titleId: 'consoles.detection.titleId',
  header: 'consoles.detection.header',
  structure: 'consoles.detection.structure',
  extension: 'consoles.detection.extension',
}

interface Fields {
  displayName: string
  slug: string
  extensions: string[]
}

type FieldErrors = Partial<Record<keyof Fields, string>>

function useFieldErrors() {
  const { t } = useTranslation()
  return (f: Fields): FieldErrors => {
    const e: FieldErrors = {}
    if (!f.displayName.trim()) e.displayName = t('common.required')
    if (!slugPattern.test(f.slug)) e.slug = t('consoles.badSlug')
    if (f.extensions.length === 0) e.extensions = t('consoles.noExtensions')
    return e
  }
}

function ConsoleFields({
  value,
  onChange,
  errors,
  slugLocked,
}: {
  value: Fields
  onChange: (next: Fields) => void
  errors: FieldErrors
  slugLocked?: boolean
}) {
  const { t } = useTranslation()
  return (
    <>
      <TextField
        label={t('consoles.name')}
        value={value.displayName}
        onChange={(e) => {
          onChange({ ...value, displayName: e.target.value })
        }}
        {...(errors.displayName ? { error: errors.displayName } : {})}
      />
      <TextField
        label={t('consoles.folder')}
        className="font-mono"
        value={value.slug}
        disabled={slugLocked}
        hint={slugLocked ? t('consoles.slugLocked') : t('consoles.folderHint')}
        onChange={(e) => {
          onChange({ ...value, slug: e.target.value.toLowerCase() })
        }}
        {...(errors.slug ? { error: errors.slug } : {})}
      />
      <ExtensionsField
        value={value.extensions}
        onChange={(extensions) => {
          onChange({ ...value, extensions })
        }}
        {...(errors.extensions ? { error: errors.extensions } : {})}
      />
    </>
  )
}

function EditDialog({ console: c, onClose }: { console: Console; onClose: () => void }) {
  const { t } = useTranslation()
  const update = useUpdateConsole()
  const validate = useFieldErrors()
  const [fields, setFields] = useState<Fields>({
    displayName: c.displayName,
    slug: c.slug,
    extensions: c.extensions,
  })
  const [tried, setTried] = useState(false)
  const errors = tried ? validate(fields) : {}
  const slugTaken = isAppError(update.error, 'conflict')

  const save = () => {
    setTried(true)
    if (Object.keys(validate(fields)).length > 0) return
    update.mutate(
      { id: c.id, input: { ...fields, displayName: fields.displayName.trim() } },
      { onSuccess: onClose },
    )
  }

  return (
    <Dialog
      title={t('consoles.editTitle', { name: c.displayName })}
      onClose={onClose}
      initialFocus="first"
      actions={
        <>
          <Button onClick={onClose}>{t('common.cancel')}</Button>
          <Button variant="primary" loading={update.isPending} onClick={save}>
            {t('common.save')}
          </Button>
        </>
      }
    >
      <ConsoleFields
        value={fields}
        onChange={setFields}
        errors={slugTaken ? { ...errors, slug: describeError(t, update.error) } : errors}
        slugLocked={c.gameCount > 0}
      />
      {update.isError && !slugTaken && (
        <Banner tone="danger">{describeError(t, update.error)}</Banner>
      )}
    </Dialog>
  )
}

function AddConsole() {
  const { t } = useTranslation()
  const create = useCreateConsole()
  const validate = useFieldErrors()
  const [query, setQuery] = useState('')
  const [platform, setPlatform] = useState<MetadataPlatform | undefined>()
  const [fields, setFields] = useState<Fields>({ displayName: '', slug: '', extensions: [] })
  const [tried, setTried] = useState(false)
  const [added, setAdded] = useState<string | null>(null)
  const search = usePlatformSearch(query)
  const errors = tried ? validate(fields) : {}

  const choose = (p: MetadataPlatform) => {
    setPlatform(p)
    setFields((f) => ({
      ...f,
      displayName: p.name,
      slug: suggestSlug(p.abbreviation ?? p.name),
    }))
  }

  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    setTried(true)
    if (!platform || Object.keys(validate(fields)).length > 0) return
    create.mutate(
      { ...fields, displayName: fields.displayName.trim(), igdbPlatformId: platform.id },
      {
        onSuccess: (c) => {
          setAdded(c.displayName)
          setPlatform(undefined)
          setQuery('')
          setFields({ displayName: '', slug: '', extensions: [] })
          setTried(false)
        },
      },
    )
  }

  const results = search.data ?? []
  const options =
    platform && !results.some((p) => p.id === platform.id) ? [platform, ...results] : results

  return (
    <aside
      aria-labelledby="add-console"
      className="box-border flex min-w-0 flex-[1_1_360px] flex-col gap-4.5 self-start rounded-xl border border-line bg-surface p-6 lg:max-w-[460px]"
    >
      <h2 id="add-console" className="m-0 text-heading font-bold">
        {t('consoles.add')}
      </h2>
      <form onSubmit={submit} className="flex flex-col gap-4.5" noValidate>
        <TextField
          type="search"
          label={t('consoles.platform')}
          value={query}
          onChange={(e) => {
            setQuery(e.target.value)
            setAdded(null)
          }}
          {...(tried && !platform ? { error: t('consoles.pickPlatform') } : {})}
        />
        <div aria-live="polite" className="flex flex-col gap-2">
          {search.isError ? (
            <IgdbError
              error={search.error}
              onRetry={() => {
                void search.refetch()
              }}
            />
          ) : query.trim().length >= minQuery && search.isPending ? (
            <Spinner label={t('igdb.searching')} />
          ) : (
            options.length > 0 && (
              <ChoiceList
                legend={t('consoles.platformResults')}
                name="igdb-platform"
                value={platform ? String(platform.id) : undefined}
                onChange={(id) => {
                  const p = options.find((o) => String(o.id) === id)
                  if (p) choose(p)
                }}
                options={options.map((p) => ({
                  value: String(p.id),
                  title: p.name,
                  meta: [p.releaseYear, p.abbreviation].filter(Boolean).join(' · '),
                  art: (
                    <span className="flex h-9 w-14 shrink-0 items-center justify-center rounded-sm bg-on-accent">
                      <ConsoleLogo imageId={p.logoImageId} className="max-h-7 max-w-12" />
                    </span>
                  ),
                }))}
              />
            )
          )}
        </div>
        <ConsoleFields value={fields} onChange={setFields} errors={errors} />
        <p className="m-0 text-caption text-ink-3">{t('consoles.addHint')}</p>
        {create.isError && (
          <Banner tone="danger">
            {describeError(t, create.error, { conflict: t('consoles.taken') })}
          </Banner>
        )}
        {added && <Banner tone="info">{t('consoles.added', { name: added })}</Banner>}
        <Button type="submit" variant="primary" size="lg" loading={create.isPending}>
          {t('consoles.add')}
        </Button>
      </form>
    </aside>
  )
}

/** Settings › Consoles: order, names, folders and extensions (RF-41). */
export function ConsolesPanel() {
  const { t } = useTranslation()
  const consoles = useConsoles()
  const reorder = useReorderConsoles()
  const remove = useDeleteConsole()
  const [editing, setEditing] = useState<Console | null>(null)
  const [deleting, setDeleting] = useState<Console | null>(null)
  const list = consoles.data ?? []

  const move = (index: number, by: -1 | 1) => {
    const ids = list.map((c) => c.id)
    const [id] = ids.splice(index, 1)
    if (id === undefined) return
    ids.splice(index + by, 0, id)
    reorder.mutate(ids)
  }

  return (
    <div className="flex flex-1 flex-wrap items-start gap-8">
      <section className="flex min-w-0 flex-[999_1_600px] flex-col gap-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="m-0 text-body-sm font-bold tracking-label text-ink-2 uppercase">
            {t('consoles.count', { count: list.length })}
          </h2>
          <span className="text-body-sm text-ink-3">{t('consoles.orderHint')}</span>
        </div>
        {consoles.isPending && <Spinner label={t('common.loading')} />}
        {(reorder.isError || remove.isError) && (
          <Banner tone="danger">
            {describeError(t, reorder.error ?? remove.error, {
              conflict: t('consoles.deleteInUse'),
            })}
          </Banner>
        )}
        <TableBox>
          <table className="w-full min-w-[760px] border-collapse">
            <thead>
              <tr className="text-left text-caption font-bold tracking-[0.06em] text-ink-3 uppercase">
                <th className="w-28 py-3 pr-2 pl-4">
                  <span className="sr-only">{t('consoles.order')}</span>
                </th>
                <th className="px-4 py-3">{t('consoles.name')}</th>
                <th className="px-4 py-3">{t('consoles.folderShort')}</th>
                <th className="px-4 py-3">{t('consoles.extensions')}</th>
                <th className="px-4 py-3">{t('consoles.detectionLabel')}</th>
                <th className="px-4 py-3 text-right">{t('consoles.games')}</th>
                <th className="px-4 py-3">
                  <span className="sr-only">{t('game.actions')}</span>
                </th>
              </tr>
            </thead>
            <tbody className="text-body">
              {list.map((c, i) => (
                <tr key={c.id} className="border-t border-line">
                  <td className="py-1 pr-1 pl-4 whitespace-nowrap">
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
                  </td>
                  <td className="px-4 py-3 font-bold">{c.displayName}</td>
                  <td className="px-4 py-3 font-mono text-caption text-ink-1">{c.slug}/</td>
                  <td className="px-4 py-3 font-mono text-chip text-ink-2">
                    {c.detection === 'structure' && `${t('consoles.folderGame')} `}
                    {c.extensions.join(' ')}
                  </td>
                  <td
                    className={cx(
                      'px-4 py-3 text-body-sm',
                      c.detection === 'extension' ? 'text-ink-2' : 'text-success',
                    )}
                  >
                    {t(detectionKeys[c.detection])}
                  </td>
                  <td className="px-4 py-3 text-right text-ink-2">{c.gameCount}</td>
                  <td className="px-3 py-1 text-right whitespace-nowrap">
                    <IconButton
                      label={t('consoles.edit', { name: c.displayName })}
                      onClick={() => {
                        setEditing(c)
                      }}
                    >
                      <EditIcon size={16} />
                    </IconButton>
                    {!c.builtIn && (
                      <IconButton
                        tone="danger"
                        label={
                          c.gameCount > 0
                            ? t('consoles.deleteBlocked', { name: c.displayName })
                            : t('consoles.delete', { name: c.displayName })
                        }
                        disabled={c.gameCount > 0}
                        onClick={() => {
                          setDeleting(c)
                        }}
                      >
                        <TrashIcon size={16} />
                      </IconButton>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableBox>
        <p className="m-0 text-body-sm leading-normal text-ink-3">{t('consoles.rules')}</p>
      </section>

      <AddConsole />

      {editing && (
        <EditDialog
          console={editing}
          onClose={() => {
            setEditing(null)
          }}
        />
      )}
      {deleting && (
        <ConfirmDialog
          title={t('consoles.deleteTitle', { name: deleting.displayName })}
          confirmLabel={t('consoles.deleteConfirm')}
          tone="danger"
          busy={remove.isPending}
          onCancel={() => {
            setDeleting(null)
          }}
          onConfirm={() => {
            remove.mutate(deleting.id, {
              onSettled: () => {
                setDeleting(null)
              },
            })
          }}
        >
          <p className="m-0">{t('consoles.deleteBody', { folder: `${deleting.slug}/` })}</p>
        </ConfirmDialog>
      )}
    </div>
  )
}
