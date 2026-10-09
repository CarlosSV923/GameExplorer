import type { TFunction } from 'i18next'

import { itemChip } from '../domain/items'
import type { LibraryItem } from '../domain/types'

export type ItemLike = Pick<LibraryItem, 'kind' | 'label'>

/** "Update v3.0.1", "DLC", "Juego"… */
export function itemLabel(t: TFunction, item: ItemLike): string {
  const chip = itemChip(item)
  return chip.label ? `${t(chip.key)} ${chip.label}` : t(chip.key)
}
