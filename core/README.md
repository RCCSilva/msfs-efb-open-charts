# Open Charts (core)

Local chart manager and in-sim moving map for Microsoft Flight Simulator 2024. One Go binary,
built without cgo: it converts georeferenced PDFs (GeoPDF) into images + georeference, makes them
searchable, pins them per flight, and serves a viewer that runs inside the MSFS EFB (iframe).

## Run

```
opencharts.exe                      # http://127.0.0.1:43210/  (manager)  /viewer.html (in-sim page)
opencharts.exe -data D:\charts      # library location (default %LOCALAPPDATA%\OpenCharts)
opencharts.exe -fixed -lat -23.62 -lon -46.65 -hdg 170   # no simulator: fixed aircraft position
opencharts.exe -simconnect-dll "C:\...\SimConnect.dll"   # explicit DLL (default: MSFS SDK folders)
```

It listens on loopback only and rejects requests whose Host/Origin is not local.

## Build

```
cd core
set CGO_ENABLED=0
go build -trimpath -ldflags "-s -w" -o ..\dist\opencharts.exe .\cmd\opencharts
```

## Layout

- `cmd/opencharts` server, `cmd/chartimport` CLI import, `cmd/mkicon` writes the application icon
- `internal/sim/msfs2024/cmd` development probes of MSFS 2024 (`fsainspect`: what is inside a package archive; `navprobe`: asks the running simulator for navigation data), not part of the product
- `internal/geo` projections (TM, LCC) + affine fit; `internal/pdfgeo` GeoPDF `/VP` reader;
  `internal/importer` render + georeference; `internal/store` library, search, groups/pins
- `internal/sources` provider modules (built-in `manual`, `decea-br`), `internal/jobs` progress
- `internal/sim` what the program needs from a simulator (interfaces + `Provider`); `internal/sim/msfs2024` what is specific to MSFS 2024, including `simconnect`: live position and navigation data via the official SimConnect.dll (no cgo)
- `web/sim` in-sim viewer (ES5, Coherent GT safe), `web/manager` manager UI (pt/en)

## Sources and policy

A source never downloads anything by itself: it lists, locates and parses what the user fetched.
DECEA/AISWEB forbids automated collection; the app only links to the official pages.
