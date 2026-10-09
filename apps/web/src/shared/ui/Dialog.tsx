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
import { cx } from './cx'

interface DialogProps {
  title: string
  onClose: () => void
  /** Buttons, the main action last (drawn on the right). */
  actions: ReactNode
  /**
   * What gets focus on open: the main action (default), the first control,
   * or the first text field (forms whose first controls are optional).
   */
  initialFocus?: 'action' | 'first' | 'field'
  /** A line under the title. */
  subtitle?: string
  /** lg: forms with several sections (the upload form). */
  size?: 'md' | 'lg'
  children?: ReactNode
}

/**
 * Modal dialog: the rest of the page becomes inert, focus moves inside (to
 * the main action) and returns where it was on close; Escape and the B
 * button close it (docs/design-handoff.md §2).
 */
export function Dialog({
  title,
  onClose,
  actions,
  initialFocus = 'action',
  subtitle,
  size = 'md',
  children,
}: DialogProps) {
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
    const field = items.find(
      (el) => el instanceof HTMLInputElement && (el.type === 'text' || el.type === 'search'),
    )
    const target =
      initialFocus === 'field'
        ? (field ?? items[0])
        : initialFocus === 'first'
          ? items[0]
          : (buttons[buttons.length - 1] ?? items[0])
    target?.focus()
    // Only on open.
    // eslint-disable-next-line react-hooks/exhaustive-deps
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
        className={cx(
          'box-border flex max-h-full w-full animate-dialog-in flex-col gap-4.5 overflow-y-auto rounded-xl border border-control bg-surface p-7',
          size === 'lg' ? 'max-w-[720px] gap-6' : 'max-w-[560px]',
        )}
      >
        <div className="flex flex-col gap-1.5">
          <h2 id={titleId} className="m-0 text-title font-bold">
            {title}
          </h2>
          {subtitle && <p className="m-0 text-body font-semibold text-ink-2">{subtitle}</p>}
        </div>
        {children && <div className="flex flex-col gap-4 text-body-lg text-ink-1">{children}</div>}
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
