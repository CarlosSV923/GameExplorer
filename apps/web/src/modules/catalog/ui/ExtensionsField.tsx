import { useId, useState, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { CloseIcon, ErrorIcon } from '@/shared/ui'

import { normalizeExtension } from '../domain/items'

/**
 * Extensions as removable tags plus a text box: Enter, comma or leaving the
 * box adds what was typed; Backspace on an empty box removes the last one.
 */
export function ExtensionsField({
  value,
  onChange,
  error,
  hint,
}: {
  value: readonly string[]
  onChange: (next: string[]) => void
  error?: string
  hint?: string
}) {
  const { t } = useTranslation()
  const id = useId()
  const [text, setText] = useState('')
  const [invalid, setInvalid] = useState(false)

  const commit = () => {
    const parts = text.split(/[,\s]+/).filter(Boolean)
    if (parts.length === 0) return
    const next = [...value]
    let bad = false
    for (const p of parts) {
      const ext = normalizeExtension(p)
      if (!ext) bad = true
      else if (!next.includes(ext)) next.push(ext)
    }
    setInvalid(bad)
    setText(bad ? text : '')
    onChange(next)
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      commit()
    } else if (e.key === 'Backspace' && text === '' && value.length > 0) {
      onChange(value.slice(0, -1))
    }
  }

  const message = invalid ? t('consoles.badExtension') : error
  return (
    <div className="flex flex-col gap-2">
      <label htmlFor={id} className="text-caption font-bold text-ink-2">
        {t('consoles.extensions')}
      </label>
      <div className="flex flex-wrap gap-2 rounded-md border border-control bg-surface-input p-2 focus-within:outline-3 focus-within:outline-offset-2 focus-within:outline-accent">
        {value.map((ext) => (
          <span
            key={ext}
            className="inline-flex items-center rounded-sm bg-surface-raised pl-2.5 font-mono text-caption"
          >
            {ext}
            <button
              type="button"
              aria-label={t('consoles.removeExtension', { ext })}
              onClick={() => {
                onChange(value.filter((v) => v !== ext))
              }}
              className="inline-flex size-control-sm cursor-pointer items-center justify-center rounded-sm border-0 bg-transparent text-ink-2 hover:text-ink-1"
            >
              <CloseIcon size={14} />
            </button>
          </span>
        ))}
        <input
          id={id}
          value={text}
          placeholder=".ext"
          aria-invalid={message ? true : undefined}
          onChange={(e) => {
            setText(e.target.value)
            setInvalid(false)
          }}
          onKeyDown={onKeyDown}
          onBlur={commit}
          className="h-control-sm min-w-20 flex-1 border-0 bg-transparent font-mono text-caption text-ink-1 outline-0"
        />
      </div>
      {message ? (
        <span className="flex items-start gap-2 text-body-sm text-danger">
          <ErrorIcon className="mt-px shrink-0" />
          {message}
        </span>
      ) : (
        hint && <span className="text-caption text-ink-3">{hint}</span>
      )}
    </div>
  )
}
