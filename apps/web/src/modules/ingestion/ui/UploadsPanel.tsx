import { useTranslation } from 'react-i18next'

import { useConsoles } from '@/modules/catalog/application/queries'
import { knownExtensions } from '@/modules/catalog/domain/items'
import { FileButton, FolderIcon, UploadIcon } from '@/shared/ui'

import { useUploadRows } from '../application/queries'
import { UploadCard } from './UploadCard'

/** Archives the app extracts itself (RF-06). */
const archives = ['.rar', '.zip', '.7z']

/** The live list of uploads (RF-13): a polite live region. */
export function UploadsPanel() {
  const { t } = useTranslation()
  const rows = useUploadRows()
  return (
    <aside
      aria-labelledby="uploads-title"
      aria-live="polite"
      className="box-border flex min-w-0 flex-[1_1_380px] flex-col gap-4 self-start rounded-xl border border-line bg-surface p-6 lg:max-w-[480px]"
    >
      <div className="flex items-center justify-between gap-3">
        <h2 id="uploads-title" className="m-0 text-heading font-bold">
          {t('uploads.title')}
        </h2>
        <span className="text-body-sm text-ink-2">
          {t('uploads.count', { count: rows.length })}
        </span>
      </div>
      {rows.length === 0 ? (
        <p className="m-0 text-body-sm text-ink-3">{t('uploads.none')}</p>
      ) : (
        rows.map((row) => <UploadCard key={row.key} row={row} />)
      )}
    </aside>
  )
}

/**
 * The drop target (RF-01): a dashed box with the accepted formats and the
 * native picker for touch screens, where there is no drag and drop.
 */
export function DropZone({
  title,
  onFiles,
  as: Heading = 'h2',
}: {
  title: string
  onFiles: (files: File[]) => void
  as?: 'h1' | 'h2'
}) {
  const { t } = useTranslation()
  const consoles = useConsoles()
  const extensions = [...new Set([...archives, ...knownExtensions(consoles.data ?? [])])]
  return (
    <div className="box-border flex min-h-[420px] min-w-0 flex-[999_1_520px] flex-col items-center justify-center gap-5 rounded-xl border-3 border-dashed border-accent p-10 text-center">
      <span className="flex size-28 items-center justify-center rounded-full bg-accent text-on-accent">
        <UploadIcon size={52} strokeWidth={2} />
      </span>
      <Heading className="m-0 text-display leading-[1.05] font-bold">{title}</Heading>
      <p className="m-0 max-w-[46ch] text-body-lg text-ink-1">{t('drop.body')}</p>
      <ul className="m-0 flex max-w-[640px] list-none flex-wrap justify-center gap-2 p-0 font-mono text-caption">
        {extensions.map((ext) => (
          <li key={ext} className="rounded-sm bg-surface-field px-2.5 py-1 text-ink-1">
            {ext}
          </li>
        ))}
      </ul>
      <FileButton variant="secondary" icon={<FolderIcon />} onFiles={onFiles}>
        {t('drop.choose')}
      </FileButton>
    </div>
  )
}
