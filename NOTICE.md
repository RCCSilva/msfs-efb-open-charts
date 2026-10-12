# Third-party notices

Open Charts is distributed under the MIT license (`LICENSE`). It uses the components below,
each under its own license.

| Component | Use | License |
|---|---|---|
| [PDFium](https://pdfium.googlesource.com/pdfium/) | PDF rendering (through WebAssembly) | BSD-3-Clause / Apache-2.0 |
| [go-pdfium](https://github.com/klippa-app/go-pdfium) | Go binding for PDFium | MIT |
| [wazero](https://github.com/tetratelabs/wazero) | WebAssembly runtime without cgo | Apache-2.0 |
| [pdfcpu](https://github.com/pdfcpu/pdfcpu) | Reading PDF structure (georeference) | Apache-2.0 |
| [fyne.io/systray](https://github.com/fyne-io/systray) | System tray icon | Apache-2.0 |
| [rsc.io/qr](https://pkg.go.dev/rsc.io/qr) | QR code of the local-network address | BSD-3-Clause |
| [golang.org/x/text](https://pkg.go.dev/golang.org/x/text) | Text normalisation (accents) | BSD-3-Clause |
| [Leaflet](https://leafletjs.com/) | Map in the viewer | BSD-2-Clause |
| [Noto Sans](https://github.com/notofonts/latin-greek-cyrillic) | Font of the in-sim viewer (`core/web/sim/fonts`) | SIL Open Font License 1.1 |
| [go-winres](https://github.com/tc-hib/go-winres) | Executable icon and version (build only) | MIT |
| [Inno Setup](https://jrsoftware.org/isinfo.php) | Installer generation (build only) | Inno Setup license (free) |

## Data and SDK

- **Maps:** © [OpenStreetMap](https://www.openstreetmap.org/copyright) contributors, ODbL. Tiles are
  cached locally and requested with an identifying user agent.
- **SimConnect:** `SimConnect.dll` is part of the Microsoft Flight Simulator SDK and is
  redistributed under the terms of the SDK.
- **Charts:** they belong to their providers (for example DECEA/AISWEB). The application does not
  redistribute them or download them automatically; follow each provider's usage policy.
