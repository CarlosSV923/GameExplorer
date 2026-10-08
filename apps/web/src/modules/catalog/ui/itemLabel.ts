import type { TFunction } from 'i18next'

import { itemChip } from '../domain/items'
import type { LibraryItem } from '../domain/types'

export type ItemLike = Pick<LibraryItem, 'kind' | 'label' | 'discNumber'>

/** "Update v3.0.1", "Disco 2", "DLC"… */
export function itemLabel(t: TFunction, item: ItemLike): string {
  const chip = itemChip(item)
  const text = chip.key === 'kind.discNumber' ? t(chip.key, { number: chip.number }) : t(chip.key)
  return chip.label ? `${text} ${chip.label}` : text
}
