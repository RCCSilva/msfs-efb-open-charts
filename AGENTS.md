# AGENTS.md

Guidance for coding agents and contributors working on **Open Charts**, a local aeronautical chart
manager and in-sim moving map for Microsoft Flight Simulator 2024 (0.3.0 Open Beta).

> Open Charts is **for flight simulation only**. Real-world aviation use is prohibited. Keep that
> notice in every user-facing surface you add (README, About, installer, in-sim viewer).

## Project overview

One Go program (no cgo) that runs in the Windows tray:

- an HTTP server on `127.0.0.1:43210` by default (port and network access are in `settings.json`)
  serving the chart API, the manager UI and the
  in-sim viewer;
- a PDF importer (PDFium compiled to WebAssembly, run by wazero) that renders GeoPDF charts to PNG
  and extracts the georeference;
- a library of charts with search, groups and pins;
- a SimConnect client (official `SimConnect.dll`, loaded at run time) for the live position;
- an installer for the EFB app (an iframe that embeds the viewer) and an Inno Setup installer.

## Repository layout

| Path | Content |
|---|---|
| `core/cmd/opencharts` | The program: flags, tray, `setup` subcommands for the installer |
| `core/internal/geo` | Projections (Transverse Mercator, Lambert Conformal Conic), affine fit |
| `core/internal/pdfgeo` | Reads the `/VP` georeference of a GeoPDF (pdfcpu) |
| `core/internal/importer` | Renders a PDF and writes the image + `chart.json` |
| `core/internal/store` | Library on disk, search, groups and pins |
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
powershell -File installer\build.ps1 -Version 0.3.0

# run for development, serving the UI from disk so web edits need no rebuild
build\opencharts.exe -web core\web -data core\data2 -no-tray
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
  Measured on a real simulator (the viewer reports it; see Setup → "Viewer browsers"):
  `CoherentGT/2.0` with a Chrome 49 engine, **no WebGL and no WebGL2**, pixel ratio 1, an EFB
  viewport of about 468×696 CSS px, and touch events reported. So the map must be **raster**:
  vector tiles (MapLibre, OpenFreeMap, MapTiler vector styles) cannot be drawn in the simulator;
  they would need to be rasterised on the server. Phones and tablets do have WebGL.
  Text fields: the page runs in a frame, so it tells the EFB app (postMessage `text-input`) when the
  search field has focus, and the app triggers the game's `FOCUS_INPUT_FIELD` / `UNFOCUS_INPUT_FIELD`
  (as the SDK's own Input does). Without it the simulator treats typed keys as key bindings.
  Leaflet's CSS asks tiles for `mix-blend-mode: plus-lighter`; viewer.css turns it off because the
  simulator flickered and blackened the base map with it.
- **Manager UI (`core/web/manager`):** modern plain JS, no build step, no framework. Never use
  `innerHTML` with data; build nodes with the `h()` helper and `textContent`. Every user-visible
  string goes through the pt/en table.
- **EFB app (`core/internal/setup/efbpkg/OpenCharts.js`):** the folder, the JS file name and
  `INTERNAL_NAME` must match (`OpenCharts`). Never use the name `TemplateApp`. The sim loads the JS
  only at start, so changes need a simulator restart.
- **Build identity:** a plain `go build` shows `0.2.0-<commit>` (and `-dirty` with uncommitted
  changes), taken from the build info Go stamps into the executable (`internal/buildinfo`); only
  `installeruild.ps1` links `buildinfo.Release=1` and shows the bare version. The manager (footer, About,
  Setup) and `/api/version` show it, the EFB app is stamped with it at install time, and Setup tells
  whether the installed EFB app is the same build as the running program.
- **Names and text:** repository files (docs, issue templates, commit messages, code comments) are
  in English. The product UI is bilingual (pt-BR and en).
- **Versions:** the single user-facing version is `0.3.0 Open Beta`, stored in the `VERSION` file.
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
- **Chart picture levels:** next to `chart.png` the server makes `chart@2.png` and `chart@4.png` (half and
  quarter size, Catmull-Rom filter, on the first request; `internal/server/chartlevel.go`). Chart-only mode
  loads the level that matches the zoom and swaps to a finer one when zooming in; the overlay mode
  always warps the full picture. All coordinates stay in full-size pixels (the reduced picture is
  stretched to the same bounds).
- **Sources** declare abilities by implementing interfaces: `Cataloger` (static list), `Locator`
  (address from a date), `Packager` (reads a downloaded archive). A source may implement only some.
  File-name rules belong to the source (`Recognize`); `manual` falls back to the other sources.
- **GeoTIFF rasters (FAA VFR and IFR):** `internal/geotiff` reads the TIFF tags (pixel scale, tiepoint, GeoKeys:
  Lambert, UTM/Transverse Mercator, geographic) and decodes the picture (LZW, Deflate, PackBits, palette,
  strips or tiles). `importer.ImportGeoTIFF` shrinks it by area averaging and writes the same
  chart.json a GeoPDF gives. The raster carries margins and legends and only the map is georeferenced,
  so the map area is found in the picture (the long frame line); the FGDC .htm gives the publication
  date, not the frame. A raster has no airport: it is a "route chart" in the library.
- **Tile pyramids:** a big raster is cut at import (`importer/pyramid.go`) into 512 px PNG tiles
  (`tiles/{z}/{x}_{y}.png`, served at `/charts/{id}/tiles/{z}/{x}/{y}.png`; `chart.json` `tiles`).
  Zoom 0 is chart.png itself (the overview); higher zooms are sharper, lower ones are filtered
  levels down to one tile. The viewer's chart-only mode loads tiles (`ChartTileLayer`); overlay (warp) mode picks the
  level that gives about one picture pixel per screen pixel and assembles the needed tiles into a
  canvas (`pyramidRegion`), using chart.png itself at zoom 0 or below. PDFs get a pyramid too when the
  rendered page is bigger than the overview size (big visual charts, drawn at 150 dpi at least).
  The sim browser has no 3D transforms, so map zoom is always an integer there (Leaflet snaps to 1).
- **View-only charts:** a PDF with no georeferencing (an FAA airport diagram) is imported anyway
  (`Options.ViewOnly`): chart.json has `unreferenced: true`, the whole picture is the map frame and there is
  no projection. The viewer forces chart-only mode for it and never shows an aircraft; the manager marks it
  "View only" and offers no "On map". Charts keep `source.pdf`, which the chart page shows beside the
  edit form (`#chart/<id>`; manager pages are real `<a>` links so they open in new tabs).
- **Navigation data (what the simulator knows):** airports, procedures and fixes are asked from the running
  simulator through SimConnect's facility API (`internal/simconnect`, `FetchAirport`, `LookupFixes`) and kept in
  SQLite (`internal/navdb`, `navdata.db`, pure Go driver). Rule: a fix enters the database only by its identifier
  *as a whole* (no region: the simulator then answers with every place of that name) and the identifier is
  marked in `ident_check`. So "is in the database" means "all of them are"; an unchecked name is asked from the
  simulator once, and an empty answer from it is a real "not found". A fix that exists once is stored without a
  region (the simulator does not say it); it is given one the first time something cites it with a region.
- **Route paste:** `internal/route` reads a route as pilots write it (airport, SID, fixes, airways, STAR, airport,
  coordinates; `DCT` and level groups ignored) and resolves each name through a `Provider` (the server's one reads
  the database, then the simulator). It plans nothing: homonyms are reported with the closest one chosen, a name
  nobody knows is a problem. `POST /api/route/resolve`. The simulator's package file
  (`...\StreamedPackages\fs24-fs-base-nav\content\minimal.fsarchive`, plain, BGL inside) was studied
  (`internal/fsarchive`, `internal/bgl`, `cmd/fsainspect`): identifiers there are base-38 numbers, it holds
  about a third of the fixes only; it is not used yet.
- **Viewer flow:** the top box of the in-sim viewer stands for the route ("SBSJ > SBGR"); the route button opens a box
  to paste a route in (resolved by `POST /api/route/resolve` on Resolve, Enter, paste, or when the EFB's text dialog ends).
  Three drawers keep the screen for the map: the **route** (left; tap the box): one container per place with a three-dot
  menu, *Direct to* (drops everything before) and *Remove*, edits redraw the line and are saved (localStorage
  `oc.routepts`); the **pinned charts** (right; pin button): vertical, grouped by airport (groups exist in the back end, the viewer only uses the current one and has no group UI); the
  **search** (right; magnifier): with a route and nothing typed it lists the charts of the route's airports (those that
  match a procedure of the route first), typing searches the whole library. Tapping the map closes the drawers. Drawers are at most half of the screen; their list scrolls inside, with up/down
  buttons (the sim's screen is hard to scroll by touch). The "several charts" button (map view only) keeps the previous
  charts under the one being opened (up to 4 extra layers, stored in `oc.multi`). Adding or removing a chart in that mode never moves the view; only the eye button of a pinned row
  (`focusChart`) fits the map to that chart.
  In chart-only mode the route goes through the chart's georeference (nothing for view-only charts).
  The manager's **Map** tab (`renderMap`) is the route planner: Leaflet over the server's tiles, `GET /api/nav/area` (airports,
  fixes from zoom 7, airway segments from zoom 6, capped and reported as cut), click an airport/fix/point of the user to add it,
  right-click to make a point of one's own, save with `choices` so that the touched place is the one used. The name
  "Map" is the planner; the in-simulator screen is called EFB.
  Charts on the planner: `GET /api/charts/footprints` (corners of every georeferenced chart in the view, cached by the
  mtime of chart.json) draws the outlines (layer "Charts", off by default); a click offers to show the chart over the
  map (a compact copy of the viewer's triangle-mesh warp, `chartWarpLayer`, using `/geo.js`; opacity per chart). The
  first view is the aircraft, else the last saved route, else the first stored airport.
  **Conflicts** (a name that exists in several places): the resolver takes the closest to the previous point and lists
  every candidate on the point (`candidates`, with the distance); the user may pick another one, which is a
  `route.Choice` {token, nth occurrence, region, kind} kept with the saved route (`choices`) and sent with
  `/api/route/resolve`. The text never carries a region. The orange `!` in the manager and in the viewer opens the list.
  **Saved routes** (`saved_route` table, `/api/routes`, `server/routes_saved.go`): written, validated (`/api/route/resolve`) and
  shown on a high overview map in the manager's **Routes** tab; the viewer lists them under the route in the left drawer
  and one tap resolves and draws one. The server only keeps name + text; places are resolved each time.
  The manager has a **Navigation** tab (`renderNav`; API in `server/nav_admin.go`, queries in `navdb/admin.go`): counts, lists
  with delete/refresh for airports, fixes and airways, the user's own points (`user_point`, region `USR`, they are found by
  the route resolver like any waypoint and are never dropped by a schema change) and the **bulk load**: (1) `navseed` reads
  the identifiers (record 0x0108) from the simulator's `fs24-fs-base-nav` / `genericairports` `.fsarchive` files (read-only,
  only the names and rough positions are kept, table `seed_ident`); (2) every name is asked to the simulator as a whole
  through SimConnect (`loadIdents`), 40 names per batch, which stores region, position and airways. Resumable (pending =
  seed names without an `ident_check`); then (3) `loadAirports`: `simconnect.ListAirports` (minimal list EX1)
  gives every airport, the ones not stored (and matching an optional code prefix such as SB) are fetched with
  `FetchAirport`, one by one. Modes: files, fixes, airports, all. Step 3 was written without a reachable
  simulator: the list format (same 42-byte minimal entry as the fixes) still has to be confirmed with a flight loaded. It refuses to mark names as "unknown" until the simulator has answered something.
  The manager has a **Guide** tab (`renderGuide` in `manager.js`, data in `GUIDE`): the sim's browser shows no
  tooltips, so every button of the viewer is described there with its icon and a numbered drawing. When a button is
  added, changed or removed in the viewer, update `GUIDE` (positions are on the 468 x 696 sim screen).
- **AIRAC cycle:** one international calendar (`naming/airac.go`); a chart's cycle is derived from its date.
- **Name patterns (My PDFs):** the user may write file-name templates (`{icao}_{type}_{title}_{date:YYYYMMDD}`,
  package `naming`, `pattern.go`), kept in `settings.json` and tried first for the `manual` source (`Server.identify`).
  A file a pattern reads with ICAO and type is imported without review. Patterns never apply to a provider's files.
- **Import flow:** files dropped in the manager are inspected first (`/api/import/inspect`); known
  provider files are imported automatically, others are reviewed by the user.
- **Memory:** the PDF renderer takes ~300 MB once loaded, so it is created on first use and
  released after 60 s idle (`internal/server/renderer.go`). Keep the resident program small.
- **Viewer state** (chart, mode, language) is kept on the server (`/api/viewer-state`) and mirrored
  in the page URL, because the EFB always loads the same URL.
- **Windows build:** the executable is a GUI-subsystem program (`-H windowsgui`), so there is no
  console; logs go to `<data>/opencharts.log`. PDFium's stdout/stderr are discarded for that reason.

## Security and policy

- By default the server listens on loopback only and rejects foreign `Host` and `Origin` headers.
  Keep the guard (`internal/server/access.go`) in front of every route; its rules are tested in
  `access_test.go`. `/api/health` is the only endpoint with permissive CORS.
- Other devices of the local network are served only when the user turns that on. They must come
  from a private address, address the server by IP, and carry the key from the QR code (a cookie).
  They never reach the system pages: `/api/setup*`, `/api/settings`, `/api/lan/*` and the package
  reader. Add any new endpoint that changes the computer or reads arbitrary paths to
  `localOnlyPath`.
- The port and the network option can change while the program runs (`Server.Rebind`): never cache
  the listen address; ask `Server.port()` / `LocalURL()`. The EFB app has the address written into
  it at install time, so a port change must be followed by an EFB app update (the manager says so).
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
