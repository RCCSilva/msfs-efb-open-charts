# AGENTS.md

Guidance for coding agents and contributors working on **Open Charts**, a local aeronautical chart
manager and in-sim moving map for Microsoft Flight Simulator 2024 (0.1.1 Open Beta).

> Open Charts is **for flight simulation only**. Real-world aviation use is prohibited. Keep that
> notice in every user-facing surface you add (README, About, installer, in-sim viewer).

## Project overview

One Go program (no cgo) that runs in the Windows tray:

- an HTTP server on `127.0.0.1:8080` (loopback only) serving the chart API, the manager UI and the
  in-sim viewer;
- a PDF importer (PDFium compiled to WebAssembly, run by wazero) that renders GeoPDF charts to PNG
  and extracts the georeference;
- a library of charts with search, flights and pins;
- a SimConnect client (official `SimConnect.dll`, loaded at run time) for the live position;
- an installer for the EFB app (an iframe that embeds the viewer) and an Inno Setup installer.

## Repository layout

| Path | Content |
|---|---|
| `core/cmd/opencharts` | The program: flags, tray, `setup` subcommands for the installer |
| `core/internal/geo` | Projections (Transverse Mercator, Lambert Conformal Conic), affine fit |
| `core/internal/pdfgeo` | Reads the `/VP` georeference of a GeoPDF (pdfcpu) |
| `core/internal/importer` | Renders a PDF and writes the image + `chart.json` |
| `core/internal/store` | Library on disk, search, flights and pins |
| `core/internal/sources` | Chart sources: built-in `manual` and `decea-br` |
| `core/internal/naming` | Name display, accent folding, type words (generic parts only) |
| `core/internal/server` | HTTP handlers, import/inspect flow, lazy renderer, setup API |
| `core/internal/simconnect` | SimConnect through `LazyDLL` (Windows only) |
| `core/internal/setup` | MSFS detection, EFB app install, `EXE.xml` autostart; embeds `efbpkg/` |
| `core/internal/tray` | Notification-area icon |
| `core/web/sim` | In-sim viewer (ES5 for the simulator's Coherent GT browser) |
| `core/web/manager` | Manager UI (plain JS, pt-BR and en) |
| `installer/` | `build.ps1` and `OpenCharts.iss` (Inno Setup) |

## Build, run and test

```powershell
# tests (run these before every commit)
cd core; go vet ./...; go test ./...

# full build: tests, exe, portable zip, installer (needs the MSFS 2024 SDK and Inno Setup 6)
powershell -File installer\build.ps1 -Version 0.1.1

# run for development, serving the UI from disk so web edits need no rebuild
build\opencharts.exe -web core\web -data core\data2 -addr 127.0.0.1:8080 -no-tray
```

Useful flags: `-fixed -lat N -lon N -hdg N` (no simulator), `-no-import`, `-no-tray`,
`-simconnect-dll PATH`, `-open`.

There is no browser test suite. To check a UI change, run the program and take a headless
screenshot (for example Edge with `--headless=new --screenshot=...`), or open the page in a
browser. Query parameters help: `/viewer.html?chart=<id>&mode=chart|overlay&lang=pt|en&at=lat,lon,hdg`.

## Conventions

- **Go:** keep `CGO_ENABLED=0`. Standard library first; a new dependency needs a clear reason and a
  permissive license (add it to `NOTICE.md`). Run `gofmt` and `go vet`. Prefer small packages and
  table-free, readable code that matches the surrounding style.
- **In-sim viewer (`core/web/sim`):** the simulator's browser is old (Chromium ~49). Write ES5
  (`var`, no arrow functions, no `fetch`, no template literals), use XHR, SVG icons only, no CSS
  animations or 3D transforms. Leaflet runs with `L_DISABLE_3D = true`. Defer layout until the
  viewport has a size (the iframe can be 0×0 at load).
- **Manager UI (`core/web/manager`):** modern plain JS, no build step, no framework. Never use
  `innerHTML` with data; build nodes with the `h()` helper and `textContent`. Every user-visible
  string goes through the pt/en table.
- **EFB app (`core/internal/setup/efbpkg/OpenCharts.js`):** the folder, the JS file name and
  `INTERNAL_NAME` must match (`OpenCharts`). Never use the name `TemplateApp`. The sim loads the JS
  only at start, so changes need a simulator restart.
- **Names and text:** repository files (docs, issue templates, commit messages, code comments) are
  in English. The product UI is bilingual (pt-BR and en).
- **Versions:** the single user-facing version is `0.1.1 Open Beta`, stored in the `VERSION` file.
  Never edit it by hand in the individual files: run
  `powershell -File installer\set-version.ps1 -Version X.Y.Z`, which syncs the manager, the EFB
  app and its package version, the executable resource (icon + version, regenerated with a pinned
  go-winres), the installer defaults and the docs. Then run `installer\build.ps1`.

## Where logic lives

- **Back-end (Go) owns the rules.** What a file is, how a chart is named, whether it is imported
  automatically or reviewed, duplicates, validation, what may be written where: all of it is decided
  in `core/internal`, in the domain packages (`sources`, `naming`, `store`, `importer`), not in
  the HTTP handlers and never in the browser. Handlers translate HTTP to domain calls.
- **One rule, one place.** If two handlers need the same decision, it belongs in a domain function
  they both call (see `sources.Identify`). Duplicated rules drift and become bugs.
- **Front-end shows and collects.** The manager and viewer handle input, layout, windows/panels and
  what to display; they send what the user entered and render what the server answers. When the UI
  needs to branch on a business decision, the API returns that decision (for example the
  `action` field of `/api/import/inspect`) instead of the UI re-deriving it from raw fields.
- Exceptions are fine when justified (for example the viewer's geometry maths, which must run in
  the browser to draw the aircraft), but say why in a comment.

## Architecture notes worth knowing

- **Georeference:** the chart image is placed with an affine map from projected coordinates to
  pixels, fitted by least squares from the GeoPDF control points. The fit error is judged in
  metres and a chart is refused above 15 m. The renderer stretches the page to whole pixels, so
  use the effective per-axis scale stored in `chart.json`, never `dpi / 72`.
- **Overlay mode** warps the chart per OpenStreetMap tile on a canvas (backward mapping with
  triangle meshes). **Chart-only mode** uses `L.CRS.Simple`; the map is rebuilt when switching.
- **Sources** declare abilities by implementing interfaces: `Cataloger` (static list), `Locator`
  (address from a date), `Packager` (reads a downloaded archive). A source may implement only some.
  File-name rules belong to the source (`Recognize`); `manual` falls back to the other sources.
- **Import flow:** files dropped in the manager are inspected first (`/api/import/inspect`); known
  provider files are imported automatically, others are reviewed by the user.
- **Memory:** the PDF renderer takes ~300 MB once loaded, so it is created on first use and
  released after 60 s idle (`internal/server/renderer.go`). Keep the resident program small.
- **Viewer state** (chart, mode, language) is kept on the server (`/api/viewer-state`) and mirrored
  in the page URL, because the EFB always loads the same URL.
- **Windows build:** the executable is a GUI-subsystem program (`-H windowsgui`), so there is no
  console; logs go to `<data>/opencharts.log`. PDFium's stdout/stderr are discarded for that reason.

## Security and policy

- The server listens on loopback only and rejects foreign `Host` and `Origin` headers. Keep that
  guard in front of every route. `/api/health` is the only endpoint with permissive CORS.
- Endpoints that write outside the data folder (`/api/setup/*`) are limited to detected simulator
  folders and need an explicit user action in the UI.
- **Never fetch from a chart provider automatically.** DECEA/AISWEB forbids automated collection
  and redistribution; the app only links to official pages and works on files the user downloaded.
  Do not embed provider API keys, scrape provider sites or add crawlers.
- **Never commit charts or provider packages** (`*.pdf`, `*.zip` are ignored on purpose), nor any
  personal data, keys or local paths.
- Do not modify the user's simulator files (`Community`, `EXE.xml`) outside the setup flow, and
  always keep other add-ons' entries intact.

## Commits and pull requests

- Small, focused commits with an imperative subject line. Describe the why in the body.
- Run `go vet ./...` and `go test ./...` first; mention manual checks for UI changes.
- Do not add generated artifacts (`build/`, `dist/`) or user data to the repository.
- Update `NOTICE.md` when adding a third-party component, and the README when behaviour that users
  see changes.
