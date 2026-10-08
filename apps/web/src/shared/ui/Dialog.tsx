import {
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'

import { useAction } from '../input'
import { tabbables } from '../input/spatial'
import { Button } from './Button'

interface DialogProps {
  title: string
  onClose: () => void
  /** Buttons, the main action last (drawn on the right). */
  actions: ReactNode
  children?: ReactNode
}

/**
 * Modal dialog: the rest of the page becomes inert, focus moves inside (to
 * the main action) and returns where it was on close; Escape and the B
 * button close it (docs/design-handoff.md §2).
 */
export function Dialog({ title, onClose, actions, children }: DialogProps) {
  const titleId = useId()
  const box = useRef<HTMLDivElement>(null)
  const [host] = useState(() => document.createElement('div'))

  useAction('back', () => {
    onClose()
    return true
  })

  useLayoutEffect(() => {
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
    document.body.appendChild(host)
    const others = Array.from(document.body.children).filter(
      (el) => el !== host && !el.hasAttribute('inert'),
    )
    for (const el of others) el.setAttribute('inert', '')
    return () => {
      for (const el of others) el.removeAttribute('inert')
      host.remove()
      opener?.focus()
    }
  }, [host])

  useEffect(() => {
    const items = box.current ? tabbables(box.current) : []
    // The main action is the last button; fall back to the first control.
    const buttons = items.filter((el) => el instanceof HTMLButtonElement)
    ;(buttons[buttons.length - 1] ?? items[0])?.focus()
  }, [])

  // Tab stays inside the dialog.
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key !== 'Tab' || !box.current) return
    const items = tabbables(box.current)
    const first = items[0]
    const last = items[items.length - 1]
    if (!first || !last) return
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault()
      last.focus()
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault()
      first.focus()
    }
  }

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-overlay p-6">
      <div
        ref={box}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        onKeyDown={onKeyDown}
        className="box-border flex w-full max-w-[560px] flex-col gap-4.5 rounded-xl border border-control bg-surface p-7"
      >
        <h2 id={titleId} className="m-0 text-title font-bold">
          {title}
        </h2>
        {children && <div className="text-body-lg text-ink-1">{children}</div>}
        <div className="flex flex-wrap justify-end gap-3">{actions}</div>
      </div>
    </div>,
    host,
  )
}

interface ConfirmDialogProps {
  title: string
  confirmLabel: string
  onConfirm: () => void
  onCancel: () => void
  /** Irreversible actions (empty the trash) use the danger tone. */
  tone?: 'primary' | 'danger'
  busy?: boolean
  children?: ReactNode
}

export function ConfirmDialog({
  title,
  confirmLabel,
  onConfirm,
  onCancel,
  tone = 'primary',
  busy = false,
  children,
}: ConfirmDialogProps) {
  const { t } = useTranslation()
  return (
    <Dialog
      title={title}
      onClose={onCancel}
      actions={
        <>
          <Button onClick={onCancel}>{t('common.cancel')}</Button>
          <Button
            variant={tone === 'danger' ? 'danger-solid' : 'primary'}
            onClick={onConfirm}
            loading={busy}
          >
            {confirmLabel}
          </Button>
        </>
      }
    >
      {children}
    </Dialog>
  )
}
