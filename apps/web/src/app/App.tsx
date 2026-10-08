import { MutationCache, QueryCache, QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, type RouterHistory } from '@tanstack/react-router'
import { useState, type ReactNode } from 'react'
import type { i18n } from 'i18next'
import { I18nextProvider } from 'react-i18next'

import { CatalogContext } from '@/modules/catalog/application/ports'
import { IdentityContext } from '@/modules/identity/application/ports'
import { sessionKey } from '@/modules/identity/application/queries'
import { IngestionContext } from '@/modules/ingestion/application/ports'
import { UploadQueueContext } from '@/modules/ingestion/application/queries'
import { UploadQueue } from '@/modules/ingestion/application/uploadQueue'
import { MetadataContext } from '@/modules/metadata/application/ports'
import { SystemContext } from '@/modules/system/application/ports'
import { createI18n } from '@/shared/i18n'
import { InputProvider } from '@/shared/input'
import { isAppError } from '@/shared/kernel/errors'

import { dataSource } from './config'
import { createAppRouter } from './router'
import { createServices, type Services } from './services'

/** Retry only what may pass on its own: the network and IGDB hiccups. */
function retry(count: number, error: unknown): boolean {
  return count < 2 && (isAppError(error, 'network') || isAppError(error, 'igdbUnavailable'))
}

function createEnvironment(services: Services, history: RouterHistory | undefined) {
  let onUnauthorized: () => void = () => undefined
  const handle = (error: unknown) => {
    if (isAppError(error, 'unauthorized')) onUnauthorized()
  }
  const queryClient = new QueryClient({
    queryCache: new QueryCache({ onError: handle }),
    mutationCache: new MutationCache({ onError: handle }),
    defaultOptions: { queries: { retry, staleTime: 10_000 }, mutations: { retry: false } },
  })
  const router = createAppRouter({ queryClient, identity: services.identity }, history)
  // The session expired (or was revoked) while using the app: back to the
  // login, which returns here afterwards.
  onUnauthorized = () => {
    const { pathname, href } = router.state.location
    if (pathname === '/entrar') return
    queryClient.removeQueries({ queryKey: sessionKey })
    void router.navigate({ to: '/entrar', search: { redirect: href } })
  }
  const queue = new UploadQueue(services.ingestion)
  return { services, queryClient, router, queue }
}

function Ports({
  services,
  queue,
  children,
}: {
  services: Services
  queue: UploadQueue
  children: ReactNode
}) {
  return (
    <IdentityContext value={services.identity}>
      <SystemContext value={services.system}>
        <CatalogContext value={services.catalog}>
          <MetadataContext value={services.metadata}>
            <IngestionContext value={services.ingestion}>
              <UploadQueueContext value={queue}>{children}</UploadQueueContext>
            </IngestionContext>
          </MetadataContext>
        </CatalogContext>
      </SystemContext>
    </IdentityContext>
  )
}

/** Composition root: adapters (VITE_DATA_SOURCE), providers and the router. */
export function App({
  i18n: injectedI18n,
  services,
  history,
}: {
  i18n?: i18n
  services?: Services
  history?: RouterHistory
}) {
  const [instance] = useState(() => injectedI18n ?? createI18n())
  const [env] = useState(() => createEnvironment(services ?? createServices(dataSource), history))
  return (
    <I18nextProvider i18n={instance}>
      <InputProvider>
        <QueryClientProvider client={env.queryClient}>
          <Ports services={env.services} queue={env.queue}>
            <RouterProvider router={env.router} />
          </Ports>
        </QueryClientProvider>
      </InputProvider>
    </I18nextProvider>
  )
}
