import { useNavigate } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { useAction } from '@/shared/input'
import { BackLink } from '@/shared/routing/links'
import { BackLabel, Banner, HelpBar, PageHeader } from '@/shared/ui'

import { useUploadQueue } from '../application/queries'
import { DropZone, UploadsPanel } from './UploadsPanel'

/** Uploads: the drop zone and the live panel (RF-01, RF-13). */
export function UploadsScreen() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queue = useUploadQueue()

  useAction('back', () => {
    void navigate({ to: '/' })
    return true
  })

  return (
    <div className="flex min-h-dvh flex-col">
      <PageHeader
        back={
          <BackLink to="/">
            <BackLabel>{t('nav.consoles')}</BackLabel>
          </BackLink>
        }
        title={t('uploads.title')}
      />
      <main className="box-border flex flex-1 flex-wrap items-stretch gap-8 px-4 pt-7 pb-10 sm:px-6 lg:px-10">
        <DropZone
          title={t('drop.screenTitle')}
          onFiles={(files) => {
            queue.add(files)
          }}
        />
        <UploadsPanel />
      </main>
      <HelpBar
        actions={[
          { glyph: 'A', label: t('help.select') },
          { glyph: 'B', label: t('help.back') },
          { glyph: 'RT', label: t('help.upload') },
          { glyph: 'DPAD', label: t('help.choose') },
        ]}
      />
    </div>
  )
}

function hasFiles(e: DragEvent): boolean {
  return e.dataTransfer?.types.includes('Files') ?? false
}

/**
 * Dragging files over any screen shows the drop zone and the uploads
 * (DropOverlay.dc.html). Folders cannot be uploaded: they need an archive.
 */
export function DropOverlay({ consoleSlug }: { consoleSlug: string | undefined }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queue = useUploadQueue()
  const [active, setActive] = useState(false)
  const [folders, setFolders] = useState(false)
  const depth = useRef(0)

  useEffect(() => {
    const enter = (e: DragEvent) => {
      if (!hasFiles(e)) return
      e.preventDefault()
      depth.current++
      setActive(true)
    }
    const over = (e: DragEvent) => {
      if (hasFiles(e)) e.preventDefault()
    }
    const leave = (e: DragEvent) => {
      if (!hasFiles(e)) return
      depth.current = Math.max(0, depth.current - 1)
      if (depth.current === 0) setActive(false)
    }
    const drop = (e: DragEvent) => {
      if (!hasFiles(e) || !e.dataTransfer) return
      e.preventDefault()
      depth.current = 0
      setActive(false)
      const items = Array.from(e.dataTransfer.items)
      const files: File[] = []
      let skipped = false
      for (const item of items) {
        if (item.kind !== 'file') continue
        if (item.webkitGetAsEntry()?.isDirectory) {
          skipped = true
          continue
        }
        const file = item.getAsFile()
        if (file) files.push(file)
      }
      setFolders(skipped)
      if (files.length > 0) {
        queue.add(files, consoleSlug)
        void navigate({ to: '/subidas' })
      }
    }
    window.addEventListener('dragenter', enter)
    window.addEventListener('dragover', over)
    window.addEventListener('dragleave', leave)
    window.addEventListener('drop', drop)
    return () => {
      window.removeEventListener('dragenter', enter)
      window.removeEventListener('dragover', over)
      window.removeEventListener('dragleave', leave)
      window.removeEventListener('drop', drop)
    }
  }, [consoleSlug, navigate, queue])

  return (
    <>
      {folders && (
        <div className="fixed inset-x-4 bottom-24 z-40 mx-auto max-w-[560px]">
          <Banner
            tone="danger"
            action={
              <button
                type="button"
                className="cursor-pointer border-0 bg-transparent text-body-sm font-bold text-danger-ink underline"
                onClick={() => {
                  setFolders(false)
                }}
              >
                {t('common.close')}
              </button>
            }
          >
            {t('drop.noFolders')}
          </Banner>
        </div>
      )}
      {active && (
        <div className="pointer-events-none fixed inset-0 z-40 box-border flex animate-fade-in flex-wrap items-stretch gap-8 overflow-y-auto bg-overlay p-4 sm:p-10">
          <DropZone title={t('drop.overlayTitle')} onFiles={() => undefined} />
          <UploadsPanel />
        </div>
      )}
    </>
  )
}
