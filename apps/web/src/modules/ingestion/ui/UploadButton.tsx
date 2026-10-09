import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { ButtonLink } from '@/shared/routing/links'
import { FileButton, UploadIcon, type ButtonSize, type ButtonVariant } from '@/shared/ui'

import { useOpenUploadForm, useUploadRows } from '../application/queries'
import { activeRows, needsUser } from '../domain/uploads'

interface UploadButtonProps {
  /** The console of the screen: it comes preselected in the form (RF-03). */
  consoleSlug?: string
  children: ReactNode
  variant?: ButtonVariant
  size?: ButtonSize
  icon?: ReactNode
}

/**
 * "Subir juegos" / "Agregar juegos" (RF-01): the native picker; the chosen
 * files open the upload form (RF-03).
 */
export function UploadButton({
  consoleSlug,
  children,
  variant = 'primary',
  size = 'md',
  icon = <UploadIcon />,
}: UploadButtonProps) {
  const openForm = useOpenUploadForm()
  return (
    <FileButton
      shortcut
      variant={variant}
      size={size}
      icon={icon}
      onFiles={(files) => {
        openForm({ kind: 'files', files, ...(consoleSlug ? { console: consoleSlug } : {}) })
      }}
    >
      {children}
    </FileButton>
  )
}

/** "Subidas · 2" in the header while uploads are in progress or need you. */
export function UploadsIndicator() {
  const { t } = useTranslation()
  const rows = useUploadRows()
  const active = activeRows(rows)
  if (active.length === 0) return null
  const attention = active.some(needsUser)
  return (
    <ButtonLink to="/subidas" size="sm" icon={<UploadIcon />}>
      {t('uploads.indicator', { count: active.length })}
      {attention && (
        <span className="inline-flex items-center gap-1.5 text-accent">
          <span aria-hidden="true" className="size-2 rounded-full bg-accent" />
          <span className="sr-only">{t('uploads.needsYou')}</span>
        </span>
      )}
    </ButtonLink>
  )
}
