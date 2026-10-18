# Open Charts — 0.7.0 Open Beta

A local aeronautical chart manager and in-sim moving map for **Microsoft Flight Simulator 2024**.

> **For flight simulation only. Use in real-world aviation is prohibited.**

Open Charts turns georeferenced chart PDFs (GeoPDF) into searchable chart groups and
shows them inside the simulator's EFB, with your aircraft's live position on the chart.

## Features

- **Import GeoPDF charts.** Each PDF is rendered to an image together with its georeference, so the
  aircraft can be placed on it. No cgo, no external tools.
- **Searchable library.** Find charts by ICAO, type or words (`sbsp`, `ils 17`, `sao paulo`).
  Names are generated from the file and the chart (`SBSP · IAC · ILS X RWY 17R`) and can be edited.
- **Chart groups.** Pin charts into groups (one per flight, or origin / destination / route / alternate:
  your call) and switch groups with one tap. The viewer shows one group at a time.
- **Two viewer modes.** The chart overlaid on OpenStreetMap, or the chart alone — one toggle. A
  regional zoom (about 30 km around the aircraft), a clean view that hides every control, and a
  choice of aircraft symbol (plane or helicopter, in your colour).
- **FAA (United States).** Terminal procedures (d-TPP, named through the cycle metadata) and VFR/IFR
  GeoTIFF rasters (sectionals, TAC, helicopter, enroute).
- **Routes.** Write or paste a route the way pilots do (`SBSJ BOBI3A DAGUB UZ6 SBGR`); airports, fixes, airways,
  SIDs and STARs are resolved with the navigation data of your own simulator and drawn over the map or the chart.
  Routes can be saved in the manager (checked on an overview map) and loaded with one tap in the viewer.
- **Navigation data.** Read from your simulator through SimConnect and kept in a local SQLite file; a bulk load
  reads the identifiers from the simulator's own files and asks the simulator for the rest. Nothing is shipped.
  You can add your own points, or import the CSV that Little Navmap exports for user points.
- **Several charts on the map**, big pyramids of tiles for large GeoTIFFs and PDFs, view-only charts (no
  georeference), a guide in the manager with a legend of every button of the in-simulator screen.
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
powershell -File installer\build.ps1 -Version 0.7.0
```

This runs the tests, builds `build\opencharts.exe`, and produces
`dist\OpenCharts-Setup-<version>-open-beta.exe` and the portable ZIP.

To work on the web UI without rebuilding:

```powershell
build\opencharts.exe -web core\web -data core\data2
```

## Port and other devices

Open Charts listens on **port 43210** of this computer (a port other programs rarely use). You can
change it in the manager (**Setup → Port and local network**); the EFB app is then told the new
address with **Update**.

In the same place you can allow **other devices on your Wi-Fi** (a tablet, a phone): turn it on and
scan the QR code. Those devices can view and manage charts and groups, but never change this
computer's settings, and they need the key carried by the QR code.

## Repository layout

| Path | Content |
|---|---|
| `core/` | Go code: server, importer, sources, SimConnect, tray, web UI |
| `core/web/sim` | The viewer that runs in the simulator's EFB (ES5, Coherent GT) |
| `core/web/manager` | The manager UI (library, sources, groups, setup) |
| `core/internal/sim/msfs2024/efbapp/efbpkg` | The EFB app, embedded in the executable |
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

MIT, see [`LICENSE`](LICENSE). Third-party components: see [`NOTICE.md`](NOTICE.md).
Not affiliated with Microsoft, Asobo Studio, DECEA or any chart provider.
