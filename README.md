# Open Charts — Open Beta 0.1.0

A local aeronautical chart manager and in-sim moving map for **Microsoft Flight Simulator 2024**.

> **For flight simulation only. Use in real-world aviation is prohibited.**

Open Charts turns georeferenced chart PDFs (GeoPDF) into searchable, per-flight chart sets and
shows them inside the simulator's EFB, with your aircraft's live position on the chart.

## Features

- **Import GeoPDF charts.** Each PDF is rendered to an image together with its georeference, so the
  aircraft can be placed on it. No cgo, no external tools.
- **Searchable library.** Find charts by ICAO, type or words (`sbsp`, `ils 17`, `sao paulo`).
  Names are generated from the file and the chart (`SBSP · IAC · ILS X RWY 17R`) and can be edited.
- **Pinned charts per flight.** Keep a set of charts for each flight and switch with one tap.
- **Two viewer modes.** The chart overlaid on OpenStreetMap, or the chart alone — one toggle.
- **Live position** from the simulator through SimConnect.
- **Chart sources.** *My PDFs* (built in) and *DECEA / AISWEB* (Brazil). The app never downloads
  from a provider by itself: it guides you to the official pages and recognises what you download.
- **Manager UI** in Portuguese and English, tray icon, Windows installer.

## Install

Download `OpenCharts-Setup-<version>.exe` (or the portable ZIP) from the
[Releases](https://github.com/RCCSilva/msfs-efb-open-charts/releases) page and run it.
Then open the manager from the tray icon → **Setup**: install the app into the EFB and, optionally,
start Open Charts with the simulator.

The installer is not code-signed yet, so Windows SmartScreen may warn on first run.

## Build

Requirements: Go 1.26+ (no cgo), the MSFS 2024 SDK (for `SimConnect.dll`), Inno Setup 6.

```powershell
powershell -File installer\build.ps1 -Version 0.1.0
```

This runs the tests, builds `build\opencharts.exe`, and produces
`dist\OpenCharts-Setup-<version>-open-beta.exe` and the portable ZIP.

To work on the web UI without rebuilding:

```powershell
build\opencharts.exe -web core\web -data core\data2 -addr 127.0.0.1:8080
```

## Repository layout

| Path | Content |
|---|---|
| `core/` | Go code: server, importer, sources, SimConnect, tray, web UI |
| `core/web/sim` | The viewer that runs in the simulator's EFB (ES5, Coherent GT) |
| `core/web/manager` | The manager UI (library, sources, flights, setup) |
| `core/internal/setup/efbpkg` | The EFB app, embedded in the executable |
| `installer/` | Installer script and build |
| `AGENTS.md` | Guide for contributors and coding agents |

## Contributing and support

Bug reports and feature requests are welcome: please use the
[issue templates](https://github.com/RCCSilva/msfs-efb-open-charts/issues/new/choose).
Please include the log file: `%LOCALAPPDATA%\OpenCharts\opencharts.log`.

## Charts and providers

Charts belong to their providers and are subject to their usage policies. Open Charts does not
redistribute charts and does not download them automatically. Do not attach provider charts to
issues unless their terms allow it.

## License

MIT, see [`core/LICENSE`](core/LICENSE). Third-party components: see [`NOTICE.md`](NOTICE.md).
Not affiliated with Microsoft, Asobo Studio, DECEA or any chart provider.
