# Golden vectors

Business rules that exist in **both** the Go backend (authoritative) and the
TypeScript frontend (live previews and demo mode) are pinned here as data.
Both test suites load these files and must produce exactly the expected output,
so the two implementations cannot drift apart.

| File | Rule | Go consumer | TS consumer |
|---|---|---|---|
| `naming-cases.json` | Folder/file naming for each item kind, including filename sanitization | `catalog/domain` `NamingPolicy` | `modules/ingestion/domain` |
| `detection-cases.json` | Console and item-kind suggestion from file name/extension (header sniffing is covered by synthetic fixtures in each suite) | `ingestion/domain/detection` | `modules/ingestion/domain` |

Changing a rule = change the vector first, then make both suites pass.
