# Avisos de terceiros / Third-party notices

Open Charts é distribuído sob a licença MIT (`core/LICENSE`). Ele usa os componentes abaixo,
cada um sob sua própria licença.

| Componente | Uso | Licença |
|---|---|---|
| [PDFium](https://pdfium.googlesource.com/pdfium/) | Renderização de PDF (via WebAssembly) | BSD-3-Clause / Apache-2.0 |
| [go-pdfium](https://github.com/klippa-app/go-pdfium) | Binding Go para o PDFium | MIT |
| [wazero](https://github.com/tetratelabs/wazero) | Execução de WebAssembly sem cgo | Apache-2.0 |
| [pdfcpu](https://github.com/pdfcpu/pdfcpu) | Leitura da estrutura do PDF (georreferência) | Apache-2.0 |
| [fyne.io/systray](https://github.com/fyne-io/systray) | Ícone na bandeja do sistema | Apache-2.0 |
| [golang.org/x/text](https://pkg.go.dev/golang.org/x/text) | Normalização de texto (acentos) | BSD-3-Clause |
| [Leaflet](https://leafletjs.com/) | Mapa no visualizador | BSD-2-Clause |
| [go-winres](https://github.com/tc-hib/go-winres) | Ícone e versão do `.exe` (só no build) | MIT |
| [Inno Setup](https://jrsoftware.org/isinfo.php) | Geração do instalador (só no build) | Licença própria (gratuita) |

## Dados e SDK

- **Mapas:** © colaboradores do [OpenStreetMap](https://www.openstreetmap.org/copyright), ODbL.
  Os tiles são usados com cache local e identificação do aplicativo.
- **SimConnect:** `SimConnect.dll` faz parte do SDK do Microsoft Flight Simulator e é redistribuída
  conforme os termos do SDK.
- **Cartas:** pertencem aos seus provedores (por exemplo DECEA/AISWEB). O aplicativo não as
  redistribui nem as baixa automaticamente; siga a política de uso de cada provedor.
