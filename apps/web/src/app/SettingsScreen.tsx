import { Link, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { ConsolesPanel } from '@/modules/catalog/ui/ConsolesPanel'
import { ScanCard } from '@/modules/catalog/ui/ScanCard'
import { TrashPanel } from '@/modules/catalog/ui/TrashPanel'
import { useLogout } from '@/modules/identity/application/queries'
import { UploadsIndicator } from '@/modules/ingestion/ui/UploadButton'
import { useMetadataStatus } from '@/modules/metadata/application/queries'
import { LanguageToggle } from '@/shared/i18n/LanguageToggle'
import { useAction } from '@/shared/input'
import { BackLink } from '@/shared/routing/links'
import { BackLabel, Button, cx, HelpBar, LogoutIcon, PageHeader } from '@/shared/ui'

import { settingsTabs, type SettingsTab } from './settingsTabs'

/** IGDB is optional (RF-54): what it adds, or how to turn it on. */
function Igdb() {
  const { t } = useTranslation()
  const status = useMetadataStatus()
  if (!status.data) return null
  return (
    <section
      aria-labelledby="igdb-title"
      className="flex flex-col gap-2 rounded-lg border border-line px-5 py-4.5"
    >
      <h2 id="igdb-title" className="m-0 flex flex-wrap items-center gap-3 text-body-lg font-bold">
        {t('settings.igdbTitle')}
        {!status.data.configured && (
          <span className="rounded-full border border-control px-2.5 py-0.5 text-chip font-bold text-ink-2">
            {t('settings.igdbOff')}
          </span>
        )}
      </h2>
      <p className="m-0 text-body-sm leading-normal text-ink-2">
        {status.data.configured ? t('settings.igdbOn') : t('settings.igdbHowTo')}
      </p>
    </section>
  )
}

function General() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const logout = useLogout()
  return (
    <div className="flex max-w-[860px] flex-col gap-4">
      <div className="flex flex-wrap items-center gap-x-6 gap-y-4 rounded-lg border border-line px-5 py-4.5">
        <span className="text-body-lg font-bold">{t('settings.language')}</span>
        <LanguageToggle long />
      </div>
      <ScanCard />
      <Igdb />
      <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-4 rounded-lg border border-line px-5 py-4.5">
        <span className="flex flex-col gap-1">
          <span className="text-body-lg font-bold">{t('settings.session')}</span>
          <span className="text-body-sm text-ink-2">{t('settings.sessionBody')}</span>
        </span>
        <Button
          size="sm"
          icon={<LogoutIcon size={16} />}
          loading={logout.isPending}
          onClick={() => {
            logout.mutate(undefined, {
              onSuccess: () => {
                void navigate({ to: '/entrar', search: {} })
              },
            })
          }}
        >
          {t('settings.logout')}
        </Button>
      </div>
      <p className="m-0 text-body-sm text-ink-3">{t('settings.legal')}</p>
    </div>
  )
}

/** Settings (Menu): consoles, trash and general, as tabs (Settings.dc.html). */
export function SettingsScreen({ tab }: { tab: SettingsTab }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const index = settingsTabs.indexOf(tab)

  const switchTab = (by: number) => {
    const next =
      settingsTabs[(index + by + settingsTabs.length) % settingsTabs.length] ?? 'consolas'
    void navigate({ to: '/ajustes/$tab', params: { tab: next } })
  }
  useAction('prev', () => {
    switchTab(-1)
    return true
  })
  useAction('next', () => {
    switchTab(1)
    return true
  })
  useAction('back', () => {
    void navigate({ to: '/' })
    return true
  })

  return (
    <div className="flex min-h-dvh flex-col">
      <PageHeader
        back={
          <BackLink to="/">
            <BackLabel>{t('nav.consoles')}</BackLabel>
          </BackLink>
        }
        title={t('settings.title')}
        aside={
          <>
            <UploadsIndicator />
            <nav aria-label={t('settings.sections')} className="-mb-5 flex gap-1 self-end">
              {settingsTabs.map((name) => (
                <Link
                  key={name}
                  to="/ajustes/$tab"
                  params={{ tab: name }}
                  aria-current={name === tab ? 'page' : undefined}
                  className={cx(
                    'inline-flex min-h-control-md items-center border-b-3 px-4.5 text-body-lg no-underline',
                    name === tab
                      ? 'border-accent font-bold text-ink-1 hover:text-ink-1'
                      : 'border-transparent text-ink-2 hover:text-ink-1',
                  )}
                >
                  {t(`settings.tabs.${name}`)}
                </Link>
              ))}
            </nav>
          </>
        }
      />
      <main className="box-border flex flex-1 flex-col px-4 pt-7 pb-10 sm:px-6 lg:px-10">
        {tab === 'consolas' && <ConsolesPanel />}
        {tab === 'papelera' && <TrashPanel />}
        {tab === 'general' && <General />}
      </main>
      <HelpBar
        actions={[
          { glyph: 'A', label: t('help.select') },
          { glyph: 'B', label: t('help.back') },
          { glyph: 'LB', label: t('help.prevSection') },
          { glyph: 'RB', label: t('help.nextSection') },
          { glyph: 'DPAD', label: t('help.choose') },
        ]}
      />
    </div>
  )
}
