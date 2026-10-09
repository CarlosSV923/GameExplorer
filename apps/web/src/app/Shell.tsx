import { Outlet, useNavigate, useRouterState } from '@tanstack/react-router'
import { useCallback, useEffect, useState } from 'react'

import { useCarousel } from '@/modules/catalog/application/carousel'
import {
  UploadFormContext,
  useJobFeed,
  useUploadQueue,
  type UploadFormRequest,
} from '@/modules/ingestion/application/queries'
import { UploadFormDialog } from '@/modules/ingestion/ui/UploadFormDialog'
import { DropOverlay } from '@/modules/ingestion/ui/UploadsScreen'
import { HealthBanner } from '@/modules/system/ui/HealthBanner'
import { useAction } from '@/shared/input'

import { dataSource } from './config'
import { DemoBanner } from './DemoBanner'

function visible(el: HTMLElement): boolean {
  return el.getClientRects().length > 0
}

/**
 * The console a drop preselects (RF-03): the console screen's, or the one
 * in the middle of the carousel; none elsewhere.
 */
function useScreenConsole(): string | undefined {
  const carousel = useCarousel()
  const { slug, home, selected } = useRouterState({
    select: (s) => {
      let slug: string | undefined
      for (const m of s.matches) {
        const params = m.params as { slug?: string }
        if (params.slug) slug = params.slug
      }
      const search = s.location.search as { consola?: unknown }
      return {
        slug,
        home: s.location.pathname === '/',
        selected: typeof search.consola === 'string' ? search.consola : undefined,
      }
    },
  })
  if (slug) return slug
  if (!home) return undefined
  const entry = carousel.entries.find((e) => e.slug === selected) ?? carousel.entries[0]
  return entry && !entry.unassigned ? entry.slug : undefined
}

/**
 * Around every signed-in screen: the health alert, the live job feed, the
 * drop overlay, the upload form and the actions that work everywhere
 * (Menu, Y, RT).
 */
export function Shell() {
  const navigate = useNavigate()
  const queue = useUploadQueue()
  const screenConsole = useScreenConsole()
  const [form, setForm] = useState<{ request: UploadFormRequest; key: number } | null>(null)
  // A new request (another drop) starts a fresh form.
  const openForm = useCallback((request: UploadFormRequest) => {
    setForm((f) => ({ request, key: (f?.key ?? 0) + 1 }))
  }, [])

  useJobFeed(true)

  // Closing the tab would interrupt an upload: ask first (RF-02).
  useEffect(() => {
    const onUnload = (e: BeforeUnloadEvent) => {
      if (queue.busy()) e.preventDefault()
    }
    window.addEventListener('beforeunload', onUnload)
    return () => {
      window.removeEventListener('beforeunload', onUnload)
    }
  }, [queue])

  useAction('menu', () => {
    void navigate({ to: '/ajustes/$tab', params: { tab: 'consolas' } })
    return true
  })
  // Y focuses the screen's search box, like "/".
  useAction('action2', () => {
    const field = document.querySelector<HTMLElement>('[data-shortcut="search"]')
    field?.focus()
    return field !== null
  })
  // RT opens the file picker of the screen's upload button. Browsers only
  // open it after a click, tap or key press: otherwise the button gets focus.
  useAction('upload', () => {
    const input = Array.from(
      document.querySelectorAll<HTMLInputElement>('input[data-shortcut="upload"]'),
    ).find(visible)
    if (!input) {
      void navigate({ to: '/subidas' })
      return true
    }
    try {
      input.showPicker()
    } catch {
      input.focus()
    }
    return true
  })

  return (
    <UploadFormContext value={openForm}>
      <HealthBanner />
      {dataSource === 'demo' && <DemoBanner />}
      <Outlet />
      <DropOverlay consoleSlug={screenConsole} />
      {form && (
        <UploadFormDialog
          key={form.key}
          request={form.request}
          onClose={() => {
            setForm(null)
          }}
          onDone={() => {
            setForm(null)
            void navigate({ to: '/subidas' })
          }}
        />
      )}
    </UploadFormContext>
  )
}
