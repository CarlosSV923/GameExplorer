import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { changeLanguage, isLanguage, type Language } from '@/shared/i18n'
import { formatSize } from '@/shared/i18n/format'
import { useGamepadConnected } from '@/shared/input'
import {
  Banner,
  Button,
  Chip,
  ChoiceChips,
  ChoiceList,
  ConfirmDialog,
  DownloadIcon,
  EmptyState,
  FileButton,
  HelpBar,
  IconButton,
  MenuIcon,
  PageHeader,
  ProgressBar,
  RematchIcon,
  SearchField,
  SegmentedToggle,
  TextField,
  TrashIcon,
  UploadIcon,
} from '@/shared/ui'

type Kind = 'base' | 'update' | 'dlc' | 'disc'

/**
 * The phase 8 deliverable on screen: tokens, components, languages and
 * input in one page. Phase 9 replaces it with the real screens.
 */
export function UiKit() {
  const { t, i18n } = useTranslation()
  const language: Language = isLanguage(i18n.language) ? i18n.language : 'es'
  const gamepad = useGamepadConnected()
  const [kind, setKind] = useState<Kind>('update')
  const [game, setGame] = useState('26764')
  const [confirming, setConfirming] = useState(false)

  const kinds = [
    { value: 'base', label: t('kind.base') },
    { value: 'update', label: t('kind.update') },
    { value: 'dlc', label: t('kind.dlc') },
    { value: 'disc', label: t('kind.disc') },
  ] as const

  return (
    <div className="flex min-h-screen flex-col">
      <PageHeader
        title={t('kit.title')}
        aside={
          <SegmentedToggle
            label={t('language.label')}
            value={language}
            onChange={(lng) => void changeLanguage(i18n, lng)}
            options={[
              { value: 'es', label: t('language.esShort') },
              { value: 'en', label: t('language.enShort') },
            ]}
          />
        }
      />

      <main className="flex flex-1 flex-col gap-10 px-4 py-7 sm:px-6 lg:px-10">
        <p className="m-0 max-w-[60ch] text-body-lg text-ink-2">{t('kit.intro')}</p>

        <Section title={t('kit.buttons')}>
          <div className="flex flex-wrap items-start gap-3">
            <FileButton onFiles={() => undefined} icon={<UploadIcon />}>
              {t('kit.primary')}
            </FileButton>
            <Button icon={<RematchIcon />}>{t('kit.secondary')}</Button>
            <Button variant="danger" icon={<TrashIcon />}>
              {t('kit.danger')}
            </Button>
            <Button disabled disabledReason={t('kit.disabledReason')}>
              {t('kit.disabled')}
            </Button>
            <Button variant="primary" loading>
              {t('kit.busy')}
            </Button>
            <IconButton label={t('help.download')}>
              <DownloadIcon />
            </IconButton>
            <IconButton label={t('help.delete')} tone="danger">
              <TrashIcon />
            </IconButton>
          </div>
        </Section>

        <Section title={t('kit.fields')}>
          <div className="grid max-w-[720px] gap-6 sm:grid-cols-2">
            <SearchField
              label={t('kit.search')}
              placeholder={t('kit.searchPlaceholder')}
              shortcut
            />
            <TextField
              label={t('kit.password')}
              type="password"
              size="lg"
              error={t('kit.passwordError')}
            />
          </div>
        </Section>

        <Section title={t('kit.chips')}>
          <div className="flex flex-wrap gap-2">
            <Chip kind="base">{t('kind.base')}</Chip>
            <Chip kind="update">{`${t('kind.update')} v3.0.1`}</Chip>
            <Chip kind="dlc">{t('kind.dlc')}</Chip>
            <Chip kind="disc">{t('kind.discNumber', { number: 2 })}</Chip>
            <Chip kind="whole">{t('kind.whole')}</Chip>
            <Chip kind="missing">{t('kind.missing')}</Chip>
          </div>
          <ChoiceChips
            legend={t('kit.type')}
            name="kind"
            options={kinds}
            value={kind}
            onChange={setKind}
          />
          <div className="max-w-[520px]">
            <ChoiceList
              legend={t('kit.search')}
              name="igdb"
              value={game}
              onChange={setGame}
              options={[
                { value: '26764', title: 'Mario Kart 8 Deluxe', meta: '2017 · Nintendo Switch' },
                {
                  value: '191419',
                  title: 'Mario Kart 8 Deluxe: Booster Course Pass',
                  meta: '2022 · Nintendo Switch',
                },
              ]}
            />
          </div>
        </Section>

        <Section title={t('kit.feedback')}>
          <div className="flex max-w-[720px] flex-col gap-4">
            <Banner tone="warning">{t('kit.warning')}</Banner>
            <Banner tone="danger">{t('kit.dangerBanner')}</Banner>
            <div className="flex flex-col gap-2.5 rounded-lg bg-surface-card p-4">
              <ProgressBar value={62} label={t('kit.upload')} />
              <span className="text-caption text-ink-2">
                {t('kit.uploading', {
                  done: formatSize(4.3e9, language, t),
                  total: formatSize(7e9, language, t),
                })}
              </span>
              <ProgressBar value={34} label={t('kit.extract')} tone="extract" />
              <span className="text-caption text-ink-2">{t('kit.extracting')}</span>
            </div>
          </div>
        </Section>

        <Section title={t('kit.dialogs')}>
          <div>
            <Button
              variant="danger"
              icon={<TrashIcon />}
              onClick={() => {
                setConfirming(true)
              }}
            >
              {t('kit.openDialog')}
            </Button>
          </div>
          {confirming && (
            <ConfirmDialog
              tone="danger"
              title={t('kit.dialogTitle')}
              confirmLabel={t('kit.openDialog')}
              onConfirm={() => {
                setConfirming(false)
              }}
              onCancel={() => {
                setConfirming(false)
              }}
            >
              {t('kit.dialogBody', { count: 3, size: formatSize(9.4e9, language, t) })}
            </ConfirmDialog>
          )}
        </Section>

        <Section title={t('kit.empty')}>
          <div className="max-w-[520px]">
            <EmptyState
              icon={<TrashIcon size={40} />}
              title={t('kit.emptyTitle')}
              body={t('kit.emptyBody')}
            />
          </div>
        </Section>

        <Section title={t('kit.input')}>
          <p className="m-0 text-body text-ink-2">
            {gamepad ? t('kit.gamepadOn') : t('kit.gamepadOff')}
          </p>
        </Section>
      </main>

      <HelpBar
        actions={[
          { glyph: 'MENU', label: t('help.menu') },
          { glyph: 'A', label: t('help.select') },
          { glyph: 'Y', label: t('help.search') },
          { glyph: 'RT', label: t('help.upload') },
          { glyph: 'DPAD', label: t('help.choose') },
        ]}
      >
        <Button icon={<MenuIcon />}>{t('help.menu')}</Button>
        <span className="inline-flex items-center gap-2.5 text-body text-ink-3">
          <UploadIcon />
          {t('help.dropHint')}
        </span>
        <FileButton onFiles={() => undefined} icon={<UploadIcon />}>
          {t('help.upload')}
        </FileButton>
      </HelpBar>
    </div>
  )
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-4">
      <h2 className="m-0 text-body-sm font-bold tracking-label text-ink-2 uppercase">{title}</h2>
      {children}
    </section>
  )
}
