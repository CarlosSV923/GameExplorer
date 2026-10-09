import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { cx, GamepadIcon, ImageIcon } from '@/shared/ui'

import { useMetadataPorts } from '../application/ports'

/**
 * A game cover from IGDB, or a placeholder without one. A game with a name
 * of its own (no IGDB) gets the generic cover with its title (RF-11).
 */
export function Cover({
  imageId,
  size,
  generic,
  className,
}: {
  imageId: string | null | undefined
  size: 'small' | 'big'
  /** The generic cover: the title and a caption (the folder). */
  generic?: { title: string; caption: string }
  className?: string
}) {
  const { t } = useTranslation()
  const ports = useMetadataPorts()
  const [failed, setFailed] = useState(false)
  const box = size === 'big' ? 'h-[293px] w-[220px] rounded-md' : 'h-14 w-[42px] rounded-sm'

  if ((!imageId || failed) && generic && size === 'big') {
    return (
      <span
        aria-hidden="true"
        className={cx(
          'flex shrink-0 flex-col items-center justify-center gap-3 border border-line bg-surface-card p-5 text-center',
          box,
          className,
        )}
      >
        <GamepadIcon size={40} strokeWidth={1.6} className="text-ink-3" />
        <span className="line-clamp-4 text-heading leading-tight font-bold break-words text-ink-1">
          {generic.title}
        </span>
        <span className="font-mono text-chip [overflow-wrap:anywhere] text-ink-3">
          {generic.caption}
        </span>
      </span>
    )
  }
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
