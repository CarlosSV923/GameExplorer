import { useTranslation } from 'react-i18next'

import { useFormat } from '@/shared/i18n/hooks'
import { Button, FileIcon } from '@/shared/ui'

import { useIngestionPorts } from '../application/ports'
import { useOpenUploadForm } from '../application/queries'

/**
 * "Probar con archivos de ejemplo" (RF-64): ready-made files that open the
 * upload form. Only adapters that have samples show it (the demo).
 */
export function SampleFiles() {
  const { t } = useTranslation()
  const format = useFormat()
  const ports = useIngestionPorts()
  const openForm = useOpenUploadForm()
  const samples = ports.samples?.() ?? []
  if (samples.length === 0) return null

  return (
    <section
      aria-labelledby="samples-title"
      className="flex w-full flex-col gap-3 rounded-xl border border-line bg-surface p-5"
    >
      <div className="flex flex-col gap-1">
        <h2 id="samples-title" className="m-0 text-heading font-bold">
          {t('samples.title')}
        </h2>
        <span className="text-body-sm leading-snug text-ink-2">{t('samples.intro')}</span>
      </div>
      <ul className="m-0 grid list-none grid-cols-[repeat(auto-fit,minmax(240px,1fr))] gap-3 p-0">
        {samples.map((s) => (
          <li
            key={s.id}
            className="flex flex-col gap-2 rounded-lg border border-line bg-surface-card p-4"
          >
            <span className="flex items-center gap-2 font-mono text-caption [overflow-wrap:anywhere] text-ink-1">
              <FileIcon size={16} className="shrink-0 text-ink-2" />
              {s.file.name}
              <span className="ml-auto shrink-0 font-sans font-semibold text-ink-2">
                {format.size(s.file.size)}
              </span>
            </span>
            <span className="text-body font-bold">{t(`samples.${s.id}.title`)}</span>
            <span className="flex-1 text-body-sm leading-snug text-ink-2">
              {t(`samples.${s.id}.detail`)}
            </span>
            <Button
              size="sm"
              className="self-start"
              onClick={() => {
                openForm({
                  kind: 'files',
                  files: [s.file],
                  ...(s.console ? { console: s.console } : {}),
                })
              }}
            >
              {t('samples.try')}
            </Button>
          </li>
        ))}
      </ul>
    </section>
  )
}
