import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { cx, ImageIcon } from '@/shared/ui'

import { useMetadataPorts } from '../application/ports'

/** A game cover from IGDB, or a labelled placeholder without one. */
export function Cover({
  imageId,
  size,
  className,
}: {
  imageId: string | null | undefined
  size: 'small' | 'big'
  className?: string
}) {
  const { t } = useTranslation()
  const ports = useMetadataPorts()
  const [failed, setFailed] = useState(false)
  const box = size === 'big' ? 'h-[293px] w-[220px] rounded-md' : 'h-14 w-[42px] rounded-sm'

  if (!imageId || failed) {
    return (
      <span
        className={cx(
          'flex shrink-0 flex-col items-center justify-center gap-2 border border-line bg-surface-field text-ink-3',
          box,
          className,
        )}
      >
        <ImageIcon size={size === 'big' ? 30 : 18} />
        {size === 'big' && <span className="text-body-sm">{t('metadata.noCover')}</span>}
      </span>
    )
  }
  return (
    <img
      src={ports.imageUrl(size === 'big' ? 'cover_big' : 'cover_small', imageId)}
      alt=""
      loading="lazy"
      onError={() => {
        setFailed(true)
      }}
      className={cx('shrink-0 bg-surface-field object-cover', box, className)}
    />
  )
}

/** A console logo from IGDB, always white (RF-20), or nothing. */
export function ConsoleLogo({
  imageId,
  className,
  onMissing,
}: {
  imageId: string | null | undefined
  className?: string
  onMissing?: () => void
}) {
  const ports = useMetadataPorts()
  if (!imageId) return null
  return (
    <img
      src={ports.imageUrl('logo_med', imageId)}
      alt=""
      onError={onMissing}
      className={cx('object-contain brightness-0 invert', className)}
    />
  )
}
