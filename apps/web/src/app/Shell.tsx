import { Outlet, useNavigate, useRouterState } from '@tanstack/react-router'
import { useEffect } from 'react'

import { useJobFeed, useUploadQueue } from '@/modules/ingestion/application/queries'
import { DropOverlay } from '@/modules/ingestion/ui/UploadsScreen'
import { HealthBanner } from '@/modules/system/ui/HealthBanner'
import { useAction } from '@/shared/input'

function visible(el: HTMLElement): boolean {
  return el.getClientRects().length > 0
}

/**
 * Around every signed-in screen: the health alert, the live job feed, the
 * drop overlay and the actions that work everywhere (Menu, Y, RT).
 */
export function Shell() {
  const navigate = useNavigate()
  const queue = useUploadQueue()
  const slug = useRouterState({
    select: (s) => {
      for (const m of s.matches) {
        const params = m.params as { slug?: string }
        if (params.slug) return params.slug
      }
      return undefined
    },
  })

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
    <>
      <HealthBanner />
      <Outlet />
      <DropOverlay consoleSlug={slug} />
    </>
  )
}
