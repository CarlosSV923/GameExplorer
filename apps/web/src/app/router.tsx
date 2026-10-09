/* eslint-disable react-refresh/only-export-components -- route definitions, not a component module */
import type { QueryClient } from '@tanstack/react-query'
import {
  createRootRouteWithContext,
  createRoute,
  createRouter,
  Outlet,
  redirect,
  useRouter,
  type RouterHistory,
} from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { ConsoleScreen } from '@/modules/catalog/ui/ConsoleScreen'
import { HomeScreen } from '@/modules/catalog/ui/HomeScreen'
import { SearchScreen } from '@/modules/catalog/ui/SearchScreen'
import { UnassignedScreen } from '@/modules/catalog/ui/UnassignedScreen'
import type { IdentityPorts } from '@/modules/identity/application/ports'
import { sessionKey, sessionQuery } from '@/modules/identity/application/queries'
import { LoginScreen } from '@/modules/identity/ui/LoginScreen'
import { JobScreen } from '@/modules/ingestion/ui/JobScreen'
import { UploadsScreen } from '@/modules/ingestion/ui/UploadsScreen'
import { isAppError } from '@/shared/kernel/errors'
import { ButtonLink } from '@/shared/routing/links'
import { Button, EmptyState } from '@/shared/ui'

import { SettingsScreen } from './SettingsScreen'
import { settingsTabs, type SettingsTab } from './settingsTabs'
import { Shell } from './Shell'

export interface RouterContext {
  queryClient: QueryClient
  identity: IdentityPorts
}

/** Only same-site paths: never redirect to another origin after the login. */
export function safeRedirect(target: unknown): string {
  return typeof target === 'string' && target.startsWith('/') && !target.startsWith('//')
    ? target
    : '/'
}

/** A route failed to load (the NAS is restarting, the network dropped). */
function RouteError() {
  const { t } = useTranslation()
  const router = useRouter()
  return (
    <main className="box-border flex min-h-dvh items-center justify-center p-6">
      <EmptyState
        title={t('errors.loadTitle')}
        body={t('errors.loadBody')}
        action={
          <Button
            variant="primary"
            onClick={() => {
              void router.invalidate()
            }}
          >
            {t('common.retry')}
          </Button>
        }
      />
    </main>
  )
}

function NotFound() {
  const { t } = useTranslation()
  return (
    <main className="box-border flex min-h-dvh items-center justify-center p-6">
      <EmptyState
        title={t('notFound.title')}
        body={t('notFound.body')}
        action={
          <ButtonLink to="/" variant="primary">
            {t('nav.consoles')}
          </ButtonLink>
        }
      />
    </main>
  )
}

const optionalId = (raw: string): number | undefined => {
  const n = Number(raw)
  return Number.isInteger(n) && n > 0 ? n : undefined
}

const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: Outlet,
  notFoundComponent: NotFound,
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/entrar',
  validateSearch: (s: Record<string, unknown>): { redirect?: string } =>
    typeof s.redirect === 'string' ? { redirect: s.redirect } : {},
  beforeLoad: ({ context, search }) => {
    if (context.queryClient.getQueryData(sessionKey)) {
      // eslint-disable-next-line @typescript-eslint/only-throw-error -- the router's redirect
      throw redirect({ href: safeRedirect(search.redirect) })
    }
  },
  component: function Login() {
    const router = useRouter()
    const { redirect: target } = loginRoute.useSearch()
    return (
      <LoginScreen
        onSuccess={() => {
          router.history.push(safeRedirect(target))
        }}
      />
    )
  },
})

/** Every screen but the login needs a session (RF-50). */
const appRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'app',
  beforeLoad: async ({ context, location }) => {
    try {
      await context.queryClient.query(sessionQuery(context.identity))
    } catch (error) {
      if (isAppError(error, 'unauthorized')) {
        // eslint-disable-next-line @typescript-eslint/only-throw-error -- the router's redirect
        throw redirect({ to: '/entrar', search: { redirect: location.href } })
      }
      throw error
    }
  },
  component: Shell,
})

const homeRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/',
  validateSearch: (s: Record<string, unknown>): { consola?: string } =>
    typeof s.consola === 'string' ? { consola: s.consola } : {},
  component: function Home() {
    const { consola } = homeRoute.useSearch()
    const navigate = homeRoute.useNavigate()
    return (
      <HomeScreen
        selected={consola}
        onSelect={(slug) => {
          void navigate({ search: { consola: slug }, replace: true })
        }}
      />
    )
  },
})

const searchRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/buscar',
  validateSearch: (s: Record<string, unknown>): { q: string; juego?: number } => {
    const juego = optionalId(String(s.juego))
    return {
      q: typeof s.q === 'string' ? s.q : '',
      ...(juego === undefined ? {} : { juego }),
    }
  },
  component: function Search() {
    const { q, juego } = searchRoute.useSearch()
    return <SearchScreen key={q} query={q} gameId={juego} />
  },
})

const consoleRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/consolas/$slug',
  component: function ConsoleList() {
    const { slug } = consoleRoute.useParams()
    return <ConsoleScreen slug={slug} gameId={undefined} />
  },
})

const gameRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/consolas/$slug/$gameId',
  component: function Game() {
    const { slug, gameId } = gameRoute.useParams()
    return <ConsoleScreen slug={slug} gameId={optionalId(gameId)} />
  },
})

const unassignedRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/no-asignados',
  component: UnassignedScreen,
})

const uploadsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/subidas',
  component: UploadsScreen,
})

const jobRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/subidas/$jobId',
  component: function Job() {
    const { jobId } = jobRoute.useParams()
    return <JobScreen key={jobId} jobId={jobId} />
  },
})

const settingsIndexRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/ajustes',
  beforeLoad: () => {
    // eslint-disable-next-line @typescript-eslint/only-throw-error -- the router's redirect
    throw redirect({ to: '/ajustes/$tab', params: { tab: 'consolas' } })
  },
})

const settingsRoute = createRoute({
  getParentRoute: () => appRoute,
  path: '/ajustes/$tab',
  params: {
    parse: ({ tab }): { tab: SettingsTab } => ({
      tab: (settingsTabs as readonly string[]).includes(tab) ? (tab as SettingsTab) : 'consolas',
    }),
    stringify: ({ tab }) => ({ tab }),
  },
  component: function Settings() {
    const { tab } = settingsRoute.useParams()
    return <SettingsScreen tab={tab} />
  },
})

const routeTree = rootRoute.addChildren([
  loginRoute,
  appRoute.addChildren([
    homeRoute,
    searchRoute,
    consoleRoute,
    gameRoute,
    unassignedRoute,
    uploadsRoute,
    jobRoute,
    settingsIndexRoute,
    settingsRoute,
  ]),
])

export function createAppRouter(context: RouterContext, history?: RouterHistory) {
  return createRouter({
    routeTree,
    context,
    ...(history ? { history } : {}),
    scrollRestoration: true,
    defaultErrorComponent: RouteError,
    defaultPendingMs: 300,
  })
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof createAppRouter>
  }
}
