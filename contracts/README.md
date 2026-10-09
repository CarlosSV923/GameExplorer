# Golden vectors

Business rules that exist in **both** the Go backend (authoritative) and the
TypeScript frontend (live previews and demo mode) are pinned here as data.
Both test suites load these files and must produce exactly the expected output,
so the two implementations cannot drift apart.

| File | Rule | Go consumer | TS consumer |
|---|---|---|---|
| `naming-cases.json` | Folder/file naming for each kind (spec §5), sanitization, rejected inputs (`expected.error` names the field) and a file's extension: the longest known one (`extensionCases`) | `catalog/domain` | `modules/ingestion/domain` |

Changing a rule = change the vector first, then make both suites pass.
