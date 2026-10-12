# Open Charts — Open Beta 0.1.0

Gerenciador de cartas aeronáuticas e mapa móvel dentro do **Microsoft Flight Simulator 2024**.
Local chart manager and in-sim moving map for **Microsoft Flight Simulator 2024**.

> **Apenas para simulação aérea. É vedado o uso em aviação real.**
> **For flight simulation only. Use in real-world aviation is prohibited.**

## O que faz / What it does

- Importa cartas **GeoPDF** (PDF georreferenciado), gera imagem + georreferência e deixa tudo
  **pesquisável** por ICAO, tipo ou palavras, com nomes gerados (`SBSP · IAC · ILS X RWY 17R`).
- **Fixa cartas por voo** e mostra a carta no EFB do simulador, sobre o OpenStreetMap ou só a carta,
  com a posição da aeronave ao vivo (SimConnect).
- **Fontes** de cartas: *Meus PDFs* (embutida) e *DECEA/AISWEB* (Brasil). O app nunca baixa nada
  sozinho de um provedor: guia você até as páginas oficiais.
- Interface em **português e inglês**, ícone na bandeja, instalador para Windows.

## Instalar (usuários)

Baixe `OpenCharts-Setup-<versão>.exe` (ou o ZIP portátil) nas *Releases*, execute e siga o
assistente. Depois abra o gerenciador pelo ícone na bandeja → aba **Configuração**.

## Compilar (desenvolvedores)

Requisitos: Go 1.22+ (sem cgo), MSFS 2024 SDK (para a `SimConnect.dll`), Inno Setup 6.

```powershell
powershell -File installer\build.ps1 -Version 0.1.0
```

Gera `dist\OpenCharts-Setup-<versão>.exe` e o ZIP portátil. Para trabalhar na interface sem
recompilar:

```powershell
opencharts.exe -web core\web -data core\data2 -addr 127.0.0.1:8080
```

## Estrutura

| Pasta | Conteúdo |
|---|---|
| `core/` | Código em Go: servidor, importador, fontes, SimConnect, bandeja, interface web |
| `core/web/sim` | Visualizador que roda no EFB do simulador (ES5, Coherent GT) |
| `core/web/manager` | Gerenciador (biblioteca, fontes, voos, configuração) |
| `core/internal/setup/efbpkg` | App do EFB embutido no executável |
| `installer/` | Script do instalador e build |
| `SPEC.md` | Especificação, decisões e histórico |

## Licença

MIT (veja `core/LICENSE`). Componentes de terceiros: veja `NOTICE.md`.
Não afiliado à Microsoft, Asobo Studio, DECEA ou a qualquer provedor de cartas.
