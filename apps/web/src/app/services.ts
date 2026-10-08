import type { CatalogPorts } from '@/modules/catalog/application/ports'
import { createCatalogHttp } from '@/modules/catalog/infrastructure/http/catalogHttp'
import type { IdentityPorts } from '@/modules/identity/application/ports'
import { createIdentityHttp } from '@/modules/identity/infrastructure/http/identityHttp'
import type { IngestionPorts } from '@/modules/ingestion/application/ports'
import { createIngestionHttp } from '@/modules/ingestion/infrastructure/http/ingestionHttp'
import type { MetadataPorts } from '@/modules/metadata/application/ports'
import { createMetadataHttp } from '@/modules/metadata/infrastructure/http/metadataHttp'
import type { SystemPorts } from '@/modules/system/application/ports'
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

/** The adapters for a data source. The demo adapters arrive in phase 10. */
export function createServices(source: DataSource): Services {
  switch (source) {
    case 'api':
    case 'demo':
      return createHttpServices()
  }
}
