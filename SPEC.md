# Open Charts (working title) - SPEC

Open-source aeronautical chart manager and in-sim viewer for Microsoft Flight Simulator 2024.
Status: planning. Decisions below were taken in conversation on 2026-10-11/12.

## Goal
A small desktop app that manages aeronautical charts from **sources**, converts them to
georeferenced images, and shows them inside the simulator (EFB iframe) with the aircraft
position - either **over OpenStreetMap** or **the chart alone**.

## Decisions
| Topic | Decision |
|---|---|
| Core language | **Go**, built **without cgo** (single static `.exe`) |
| PDF rendering | PDFium (WebAssembly via go-pdfium, wazero). ~135-350 ms per chart |
| Georeference | Read ISO 32000 geospatial PDF (`/VP` -> `/Measure` GEO). UTM and Lambert supported |
| Scale | Use the **effective** pixel scale (rendered px / page pt, per axis), never assume dpi/72 |
| SimConnect | Official `SimConnect.dll` loaded at runtime (no cgo). Redistribution confirmed by the author |
| Sources | Pluggable. First: DECEA (Brazil) - guides the user to download. Also: manual PDF upload |
| Pins | Charts are **pinned per flight** (routes may come later, not now) |
| Manager UI | Modern web UI, **concise and direct**; embedded in the binary |
| In-sim UI | Conservative JS (Coherent GT = Chrome 49). Search + pins + two view modes |
| Languages | **Portuguese (pt-BR) and English**, both |
| License | Open source (choose before first release; avoid AGPL dependencies) |

## Policy constraints (DECEA / AISWEB)
- The AISWEB usage policy forbids bots/crawlers and framing. **No automated scraping, no
  background polling.** The user downloads files; the app only organises and converts them.
- The official API needs a key issued by DECEA; keys are never stored in the repository.
- Do not publish downloaded charts or generated images in any repository.

## Product
1. **Sources** - each source declares what it allows (download guidance, manual only, API).
2. **Library** - every chart has: id, source, ICAO, type, generated display name (editable),
   validity (AIRAC date), georeference status, files.
3. **Search** - by ICAO, type, runway, procedure name, aliases. Same index for the manager and the sim.
4. **Flights** - a flight holds a small set of **pinned** charts; one flight is "current".
5. **Viewer in the sim** - search box, pinned bar, chart list, and a button to switch between
   **OSM overlay** and **chart only**.

## Architecture
```
core (Go)
  store      library on disk + in-memory index, flights/pins, settings
  importer   PDF -> chart.png + chart.json (georef, effective scale)
  sources/   decea-br, manual, (more later)
  api        HTTP: search, charts, flights/pins, import, progress (SSE)
  simlink    SimConnect via DLL: position, nearest airport
  web/manager   management UI (modern, concise)
  web/sim       in-sim viewer (old-browser safe)
```

## Milestones
1. Importer in Go: georeference + `chart.json` with parity against the Python tool; library API.
2. Sources framework + DECEA guide + manual upload.
3. In-sim viewer v2: search, generated names, flights/pins, overlay/chart-only toggle.
4. Manager UI (pt-BR + en).
5. SimConnect: live position; suggest charts for the nearest airport.
6. Packaging: single exe, installer, docs, license.

## Open questions
- Final name and repository (candidates: `open-charts`). Note an unrelated "OpenCharts"
  trading-charts project exists on GitHub.
- Does the keyboard reach a text input inside the EFB iframe? If not: on-screen keyboard.
- How is the "current flight" chosen: in the manager, in the sim, or both?
- Chart-name generation rules for non-DECEA sources and manual uploads.

## Progress log
- **2026-10-12 - Milestone 1 (importer), first half done.** `core/` (Go 1.27, `CGO_ENABLED=0`):
  `internal/geo` (UTM + Lambert + affine fit), `internal/pdfgeo` (pdfcpu: /VP reader),
  `internal/importer` (PDFium-wasm render + chart.json), `cmd/chartimport` (CLI).
  - 6 sample charts imported; corners match the Python prototype within 0.004-0.017 px (< 0.5 m).
  - End-to-end check on the SBSP ADC (raster scan vs OSM runways): 17R/35L stripe 44.4 m wide,
    centre offset +3.6 m (sd 1.1); Python gives +3.7 m. Same result, so the effective scale is right.
  - Effective scale (PDFium stretches the page to whole pixels, 5.1274/5.1265 px/pt instead of
    5.125) is stored in chart.json as `scale.{x,y}` and used for the pixel mapping.
  - Control-point fit is now judged in metres; GPTS with 4 decimals limits some charts to ~3 m.
  - Import time 0.9-1.9 s per chart (REA 5.7 s: PNG BestCompression dominates; tune later).
  - Pending in M1: library store + search index + HTTP API.
- **2026-10-12 - Milestone 1 complete (library + API).** `cmd/opencharts` is the server (single
  exe, ~25 MB, no cgo): `internal/store` (library, search index, flights/pins), `internal/naming`
  (generated names, pt/en type words), `internal/server` (HTTP API, import, OSM tile cache,
  request guards), `web/sim` (current viewer pages embedded).
  - Imported the 6 sample charts via `POST /api/import`; names are generated
    ("SBSP · IAC · ILS X RWY 17R"). Search matches ICAO, type, runway words, pt/en synonyms
    ("saida"/"departure"), accent-folded text.
  - Flights/pins work (`/api/flights`, pins per flight). Guards: foreign Host/Origin -> 403,
    non-PDF -> 400, path traversal -> 404.
  - Viewer pages (overlay and chart-only) work against the Go server with Go-generated charts.
  - Known gaps: pins are returned in search order, not pin order; viewer UI is still the
    prototype (no search box / pin bar yet - milestone 3); position is fixed.

## Sources model (clarified 2026-10-12)
A **source** formalises, standardises and structures a provider of aeronautical charts (real,
verified sources are the norm; the app is for simulation). It answers two questions:
**where do the files come from** and **how is what was downloaded turned into charts the sim can
search and show**.

- A GeoPDF alone is enough: the built-in "manual" path does everything without any source.
  Sources exist to scale: naming, grouping, update tracking, guided download.
- **Origins (locators)**
  - *static*: a list (name, chart type, link). Example: the DECEA visual charts catalog.
  - *dynamic*: a link derived from something such as a date (AIRAC cycle / publication date).
    Example: DECEA full package `pacote-full-YYYYMMDD.zip`. Date-based only, for now.
  - *manual*: the user supplies the file (always available, also when a policy forbids fetching).
- **Recognition**: once the user has downloaded something, the app identifies it (which source,
  which catalog item / date) from its name, structure or content.
- **Ingestion (parsers)**: turns the artifact (zip package, single PDF) into normalised charts:
  ICAO, type, title, validity, georeference. Output is what search and the viewers use.
- **Policy** is part of the definition (no automated fetch, redistribution, licence, terms URL).
- **Provenance** is recorded per chart (source, origin, original file name).
- **Future (not now):** sources may become installable plugins / a marketplace, including paid
  sources from third parties. Keep the boundary declarative (manifest + patterns) where possible,
  so that most sources need data rather than code. No plugin mechanism is designed yet.

### Where the code stands
- `internal/sources`: DECEA info + policy, static catalog, package (zip) indexer.
- `internal/naming`: DECEA file-name rules are still global; they belong to the source definition.
- `internal/server/sources_api.go`: catalog, package index/import (tracked job), inbox scan.
- Not yet: dynamic locator (date -> package URL), recognition by content, declarative manifest.

## Progress log (cont.)
- **2026-10-12 - Milestone 2, mostly done.** Tested against the real 700 MB DECEA package:
  index in 0.07 s (1,925 charts, 251 airports); import of SBSP ADC+IAC = 13 charts as a tracked
  job; re-running skips all 13 (no duplicates). Inbox scan: new PDF imported, duplicate/fake
  handled, files moved to `inbox/done`. Catalog marks items already in the library.
  Pending in M2: dynamic locator, manifest-based source definitions, user-facing guide texts.

## Sources model - implemented (2026-10-12, clarified by Rafael)
- **`manual` is the only built-in source.** Any GeoPDF works through it (upload, inbox folder).
- **DECEA is a provider module compiled in for now** (isolated in `internal/sources/decea.go` +
  `package.go`); in the future it becomes a plugin. Other providers (FAA, ...) follow later.
- **Capabilities are declared by each source**, not global: `Cataloger` (static list),
  `Locator` (dynamic address from a date), `Packager` (reads a downloaded archive). A source may
  have only one (e.g. static-only). `Info.capabilities` is derived from the interfaces implemented.
- **Naming rules live in the source** (`Recognize`); `manual` falls back to trying every source's
  recogniser, then to a tidied file name.
- **Provenance**: chart.json now has `package` (archive the PDF came from) besides `source` and
  `originalFile`.
- **Locator (DECEA)**: offers `pacote-full-YYYYMMDD.zip` for the chosen day, the current AIRAC
  cycle date and the next one. The real package seen was named after the generation day
  (`pacote-full-20261011.zip`) while its charts are effective 2026-10-01, so the naming rule is
  not confirmed: addresses are flagged `verified:false` and the official AISWEB page is linked.
  AIRAC cycles: 28 days, anchor 2026-10-01 = 2610.
- API: `GET /api/sources/{id}/locate?date=`, `GET /api/sources/{id}/package?path=`,
  `POST /api/sources/{id}/package/import`, `GET /api/sources/{id}/catalog`.

## Milestones 3-5 (2026-10-12)
- **M3 viewer v2** (`web/sim/viewer.html|js|css`): one page, search box (finds by ICAO, type,
  words; pt/en synonyms), pin/unpin per result, pinned bar in pin order for the current flight,
  flight selector + new flight, overlay-on-OSM <-> chart-only toggle (map is rebuilt: different
  CRS), opacity step, follow/fit, on-screen keyboard (typing into the EFB iframe is untested),
  pt/en strings (`?lang=`). ES5, no animations, SVG icons. EFB app points to `viewer.html`.
- **M4 manager** (`web/manager`, served at `/` -> `/manager/`): Library (search, type filter,
  pin, rename, delete, open in either mode), Add (drop PDFs, inbox folder + import job),
  Sources (guide, locator links, package reader + per-airport import with progress, catalog with
  "open official page" and installed badge), Flights. Light/dark, pt/en. No framework, no build.
- **M5 SimConnect** (`internal/simconnect`): official `SimConnect.dll` loaded with LazyDLL
  (no cgo). Defines lat, lon, true heading, ground speed, altitude; requests every 6 sim frames;
  reconnects when the sim is closed/started; states `no-dll | waiting-sim | connected |
  receiving`. DLL lookup: `-simconnect-dll`, next to the exe, data dir, `MSFS2024_SDK`, default
  SDK folder. `-fixed` forces the fixed position. **Not yet tested against a running simulator**
  (verified: DLL loads, graceful "simulator not reachable").
