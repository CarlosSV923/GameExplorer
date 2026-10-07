# Spike: large files (phase 0.5)

Throwaway prototype. It validates on the real NAS that tus uploads, 7zz extraction, `rename` into the library, ranged downloads and streamed zips handle files of tens of GB with flat memory. Delete this folder and `.github/workflows/spike-image.yml` once the results are recorded in `docs/spike-results.md`.

## Local run
```bash
docker build -t ge-spike:local .
docker run --rm -p 8090:8090 -v ge-spike-lib:/library ge-spike:local
```
Open http://localhost:8090.

## TrueNAS
See `truenas-app.yaml` (Apps → Discover Apps → ⋮ → Install via YAML).

## Local baseline (Docker Desktop, Windows, 2026-10-06)
| Step | Result |
|---|---|
| tus upload 3.1 GB (single PATCH) | 13.5 s, peak RSS 10 MB |
| 7z encrypted, no password | fails fast with "Wrong password?" |
| 7z extraction (store) | 11.6 s, 271 MB/s |
| rename into library | instant |
| Range request | 206, exact bytes |
| Full download 3.1 GB | 3.8 s |
| Store zip 3.1 GB | 6.3 s, valid per `7zz t` |
| Path traversal | 400 |
| Peak RSS overall | 12 MB |

Finding: extracted files keep the mode stored in the archive (`rw-r--r--`); umask alone is not enough. The real app must chmod/chown after extraction.
