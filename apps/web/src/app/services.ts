import type { CatalogPorts } from '@/modules/catalog/application/ports'
import { createCatalogHttp } from '@/modules/catalog/infrastructure/http/catalogHttp'
import { seededLibrary } from '@/modules/catalog/infrastructure/demo/seed'
import type { IdentityPorts } from '@/modules/identity/application/ports'
import { createIdentityDemo } from '@/modules/identity/infrastructure/demo/identityDemo'
import { createIdentityHttp } from '@/modules/identity/infrastructure/http/identityHttp'
import type { IngestionPorts } from '@/modules/ingestion/application/ports'
import { createIngestionDemo } from '@/modules/ingestion/infrastructure/demo/ingestionDemo'
import { createIngestionHttp } from '@/modules/ingestion/infrastructure/http/ingestionHttp'
import type { MetadataPorts } from '@/modules/metadata/application/ports'
import {
  createMetadataDemo,
  demoCatalog,
} from '@/modules/metadata/infrastructure/demo/metadataDemo'
import { createMetadataHttp } from '@/modules/metadata/infrastructure/http/metadataHttp'
import type { SystemPorts } from '@/modules/system/application/ports'
import { createSystemDemo } from '@/modules/system/infrastructure/demo/systemDemo'
import { createSystemHttp } from '@/modules/system/infrastructure/http/systemHttp'
import { createApiClient } from '@/shared/api/client'

import type { DataSource } from './config'

/** Every module's ports: one adapter set, chosen here only. */
export interface Services {
  identity: IdentityPorts
  system: SystemPorts
  catalog: CatalogPorts
  metadata: MetadataPorts
  ingestion: IngestionPorts
}

export function createHttpServices(): Services {
  const client = createApiClient()
  return {
    identity: createIdentityHttp(client),
    system: createSystemHttp(client),
    catalog: createCatalogHttp(client),
    metadata: createMetadataHttp(client),
    ingestion: createIngestionHttp(client),
  }
}

/**
 * The demo without backend (RF-60): in-memory adapters sharing one
 * library, seeded from the fixed IGDB catalog. Reloading starts over.
 */
export function createDemoServices(): Services {
  const logos = new Map(demoCatalog.platforms.map((p) => [p.id, p.logoImageId]))
  const games = new Map(demoCatalog.games.map((g) => [g.id, g]))
  const library = seededLibrary(logos, (id) => games.get(id))
  return {
    identity: createIdentityDemo(),
    system: createSystemDemo(),
    catalog: library.ports(),
    metadata: createMetadataDemo(),
    ingestion: createIngestionDemo(library),
  }
}

/** The adapters for a data source. */
export function createServices(source: DataSource): Services {
  switch (source) {
    case 'api':
      return createHttpServices()
    case 'demo':
      return createDemoServices()
  }
}
