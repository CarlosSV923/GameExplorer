import { useTranslation } from 'react-i18next'

import { Chip } from '@/shared/ui'

import { itemChip } from '../domain/items'
import { itemLabel, type ItemLike } from './itemLabel'

export function ItemChip({ item }: { item: ItemLike }) {
  const { t } = useTranslation()
  return <Chip kind={itemChip(item).kind}>{itemLabel(t, item)}</Chip>
}
