/* Open Charts manager. Plain JS, no build step. Only textContent is used for data (no innerHTML). */
(function () {
  'use strict';

  const STR = {
    en: {
      library: 'Library', sources: 'Sources', groups: 'Groups', back: '< Sources',
      group: 'Group', newGroup: 'New group', groupName: 'Group name',
      search: 'Search by ICAO, type or name...', all: 'All', charts: 'charts',
      addToGroup: '+ Add to group', inGroup: 'In group', addSelected: 'Add selected to group',
      removeFromGroup: 'Remove from this group', confirmDel: 'Delete this chart from the library?', newName: 'New name',
      rename: 'Rename', del: 'Delete', chartOnly: 'Chart only', onMap: 'On map',
      thisGroup: 'Charts in this group', noPins: 'Nothing here yet. Use "Add to group" in the library.',
      noCharts: 'No charts yet. Open Sources to add some.', addCharts: 'Add charts (Sources)',
      current: 'Current', makeCurrent: 'Use this group', pinnedN: 'pinned',
      dropPdf: 'Drop here what you downloaded: PDFs (several at once) and/or a .zip package. Known files are named and imported automatically. Or', choose: 'choose files',
      linksTitle: 'Official links', linkHub: 'AISWEB: charts page', linkHubNote: 'click "Pacote completo" for the full package (.zip)', exVisual: 'Example of a visual chart',
      pkgFrom: 'Package', pkgChoose: 'Choose the aerodromes to import, or import everything.', pkgNone: 'Nothing of this package was recognised.',
      pkgImportAll: 'Import everything', pkgNew: 'new', pkgAlready: 'already imported', pkgPrev: 'Previous', pkgNext: 'Next', pkgOf: 'of',
      pkgAllInfo: 'Prepares {n} charts, about {t}.', pkgAllWarn: 'Importing everything prepares every chart of the package on this computer (drawing each PDF as an image). It can take {t} and the computer will be busy meanwhile. You can leave this page open and keep using the app. Continue?',
      patTitle: 'Name pattern for your PDFs', patHelp: 'Name your PDFs the same way and the app reads the airport, type, title and date from the file name, so a whole folder is imported without asking. Write the pattern with placeholders.',
      patLegend: 'Placeholders', patIcao: 'airport code (4 letters)', patType: 'chart type (IAC, SID, STAR, ADC...)', patTitle2: 'the chart\'s name',
      patDate: 'validity date; a format fixes it, for example {date:DD-MM-YYYY} (default: YYYYMMDD or YYYY-MM-DD). {cycle} reads an AIRAC cycle such as 2610', patSkip: 'any text you do not need',
      patRules: '"_", "-" and a space are interchangeable, and capitals do not matter. One pattern per line; the first that fits wins.',
      patExample: 'Example: {icao}_{type}_{title}_{date:YYYYMMDD} reads SBSP_IAC_ILS-X-RWY-17R_20260806.pdf',
      patPlaceholder: '{icao}_{type}_{title}_{date:YYYYMMDD}', patTestLabel: 'Test with a file name', patTestPlaceholder: 'SBSP_IAC_ILS-X-RWY-17R_20260806.pdf',
      patSave: 'Save patterns', patSaved: 'Patterns saved', patNoMatch: 'no pattern matches this name', patResult: 'Reads',
      libAll: 'All charts', libAirports: 'By airport', libRoutes: 'Route charts', noIcao: 'No airport (visual charts)', airportsN: 'airports',
      selN: '{n} selected', selPage: 'All {n} on this page are selected.', selAll: 'Select all {n} matching charts', selClear: 'Clear selection',
      delSelected: 'Delete selected', confirmDelN: 'Delete {n} charts from the library? This cannot be undone.', deletedN: '{n} charts deleted',
      backAirports: '< Airports', openAirport: 'Open', selectAllPage: 'Select all on this page',
      faaMetaTitle: 'd-TPP metadata (names of the files)', faaMetaNone: 'Not loaded: the FAA PDFs are named by number, and without the metadata of the cycle they cannot be told apart.',
      faaMetaOk: 'Loaded: cycle {cycle} ({from} to {to}), {charts} charts of {airports} airports.', faaMetaAdd: 'Add the metadata (.xml, or the "E" zip)',
      faaMetaDone: 'Metadata loaded', faaMetaWhy: 'Download it once per cycle (28 days): it is inside DDTPPE_<date>.zip (about 130 MB).',
      pkgEta: 'left', pkgDuration: { s: 'a few seconds', m: '{n} min', h: '{h} h {m} min' },
      inbox: 'Inbox folder', inboxHelp: 'What you add here stays in this folder, on this computer. PDFs are imported when you click the button; for a .zip, read it and choose the aerodromes.',
      importInbox: 'Import the PDFs of the inbox', noInbox: 'The inbox is empty.',
      imported: 'imported', skipped: 'skipped', failed: 'failed', done: 'Done', error: 'Error',
      placesTitle: 'Go straight to an aerodrome', placesHelp: 'Type the code or the name: the link opens the page of that aerodrome in the cycle in force, where its charts are listed.',
      placesInput: 'EGLL, heathrow, gatwick...', placesNone: 'No aerodrome matches.', placesOpen: 'Open the charts page', placesMore: 'More aerodromes match: type more of the name.',
      capPlaces: 'By aerodrome',
      srcFilter: 'Filter by name...', srcAllCountries: 'All countries', srcCountry: 'Country', srcSource: 'Source', srcWhat: 'What it gives',
      srcNone: 'No source matches.', srcAnyCountry: 'Any country', srcOpen: 'Open',
      builtin: 'Built in', noAuto: 'No automatic download', policyLink: 'Usage policy', official: 'Official page',
      howTo: 'How to get charts from this source', locate: 'Where to get the package',
      locateHelp: 'Addresses derived from the file naming; they open in your browser and you download the file yourself.',
      unverified: 'not confirmed', package: 'Downloaded package', pkgPath: 'Path to the .zip file (or pick one from the inbox)',
      readPackage: 'Read package', airports: 'airports', importAirport: 'Import',
      catalog: 'Catalog', catalogHelp: 'Open the official page, save the PDF, then use "Add file" on the same row.',
      openOfficial: '1. Open official page', addFile: '2. Add file', installed: 'Installed', snapshot: 'Snapshot of',
      tabCatalog: 'Visual charts (catalog)', tabPackage: 'Aerodromes (package)', tabFiles: 'Add files',
      stepsFiles: 'Add what you downloaded', capCatalog: 'Catalog', capPackage: 'Package', capLocate: 'By date', capUpload: 'Upload',
      efbTitle: "The screen the simulator's EFB shows, in its own tab",
      about: 'About', setup: 'Setup', efb: 'EFB', guide: 'Guide', nav: 'Navigation', routes: 'Routes', mapTab: 'Map',
      whyManualTitle: 'Why do I have to download the charts myself?',
      whyManual: 'Most chart providers do not allow redistribution, so we cannot offer the charts already processed and ready. Many also forbid any kind of bulk download or scraping script. To stay within the terms of use, we are being conservative for now: you download the files yourself and Open Charts processes them on your computer. We may look for a better way in the future.',
      guideIntro: 'The simulator screen shows no hints when you hover a button, so here is what each one does. The numbers on the drawing match the list.',
      guideTop: 'Top bar', guideLeft: 'Left column: moving the view', guideRight: 'Right column: how the chart is shown', guideLists: 'Inside the lists',
      diagTitle: 'Viewer browsers', diagNone: 'No viewer has reported yet. Open the viewer (in the simulator or in a browser) and come back.',
      diagSim: 'Simulator (EFB)', diagBrowser: 'Browser', diagYes: 'yes', diagNo: 'no', diagRatio: 'Pixel ratio',
      diagHelp: 'What each viewer reported about itself. WebGL decides whether a vector map (lighter and sharper) is possible inside the simulator.',
      baseTitle: 'Base map', baseLight: 'Light (quiet)', baseStandard: 'Standard OpenStreetMap', baseDark: 'Dark', baseNone: 'No map',
      aircraftTitle: 'Aircraft symbol', aircraftPlane: 'Airplane', aircraftHeli: 'Helicopter', aircraftColor: 'Colour',
      aircraftHelp: 'What marks the aircraft on the map in the viewer. Reload the viewer to see the change.',
      col_yellow: 'Yellow', col_white: 'White', col_orange: 'Orange', col_red: 'Red', col_magenta: 'Magenta',
      col_blue: 'Blue', col_cyan: 'Cyan', col_green: 'Green', col_black: 'Black',
      baseHelp: 'What is drawn under the chart in the viewer. Light is a quiet grey version of OpenStreetMap: lighter to load and easier on the chart. Reload the viewer to see the change.',
      detailTitle: 'Chart detail', detailNormal: 'Normal', detailHigh: 'High',
      detailHelp: 'Applies to charts imported from now on. High is sharper when you zoom in (phones, large screens) but makes bigger files and takes longer. To change charts already in the library, delete and add them again.',
      netTitle: 'Port and local network', portTitle: 'Server port', lanTitle: 'Local network (tablet and phone)', portLabel: 'Port', apply: 'Apply', portApplied: 'Port changed',
      portHelp: 'Default {n}. Change it if another program uses it. Afterwards update the EFB app and restart the simulator.',
      portFixed: 'The address was set with -addr on the command line; these options are off.',
      lanLabel: 'Other devices', lanOn: 'Allowed: tablet and phone on the same Wi-Fi can connect', lanOff: 'Only this computer',
      lanHelp: 'Scan the QR code with the tablet or phone. They can view and manage charts and groups, but never change this computer.',
      lanFirewall: 'If Windows asks about the firewall, allow Open Charts on private networks.',
      lanNoAddress: 'No network address found. Connect to a Wi-Fi or Ethernet network.',
      lanVirtual: 'virtual adapter', lanRotate: 'Generate a new access key', lanRotateConfirm: 'Devices already connected will have to scan the QR code again. Continue?',
      stEfbUrl: 'The EFB app points to another address',
      viewOnly: 'View only', viewOnlyHelp: 'No georeference: it can be looked at, but has no aircraft position and cannot go over the map.',
      preview: 'Original', pdfOriginal: 'Original PDF', chartImage: 'Chart image', backLibrary: '< Library', openPdf: 'Open the PDF', notFound: 'Chart not found',
      editChart: 'Edit properties', save: 'Save', cancel: 'Cancel', saved: 'Saved',
      editHelp: 'The display name is generated from ICAO, type and title.',
      editWarnSource: 'Source and georeference cannot be changed.',
      sUploading: 'Uploading the file', sAnalyzing: 'Reading the PDF and identifying the chart',
      sWaiting: 'Waiting for the previous file', sEngine: 'Starting the PDF engine (first import only)',
      sRender: 'Rendering and georeferencing the chart', sSave: 'Saving to the library',
      aboutProject: 'Project and support', aboutRepo: 'Source code on GitHub', aboutIssues: 'Open issues',
      aboutBug: 'Report a bug', aboutFeature: 'Suggest a feature', aboutBeta: 'This is an Open Beta: bug reports are very welcome.',
      aboutLogs: 'When reporting a bug, include the log file',
      setupTitle: 'Setup and status', stServer: 'Open Charts server', stRunning: 'Running at',
      stSim: 'Simulator link', stSimRecv: 'Receiving the aircraft position', stSimConn: 'Connected, waiting for a position (start a flight)',
      stSimWait: 'Waiting for MSFS to start', stSimFixed: 'Fixed test position (no simulator link)',
      stInstall: 'MSFS installation', stNoInstall: 'No MSFS 2024 installation found. Enter the Community folder:',
      stEfb: 'EFB app', stEfbNo: 'Not installed', stEfbOld: 'Installed, update available', stEfbOk: 'Installed',
      btnInstall: 'Install in the EFB', btnUpdate: 'Update', btnRemove: 'Remove',
      stAuto: 'Start with MSFS', stAutoHelp: 'Open Charts starts when the simulator starts (no window).', btnEnable: 'Enable', btnDisable: 'Disable',
      stEfbLegacy: 'Old version (Local Moving Map) found: installing replaces it',
      stData: 'Data folder', stRestart: 'Restart the simulator for the EFB app change to take effect.',
      edSteam: 'Steam', edStore: 'Microsoft Store',
      reviewTitle: 'Review before importing', icao: 'ICAO', ctype: 'Type', ctitle: 'Title', validity: 'AIRAC cycle or date', cname: 'Display name',
      importOne: 'Import', importAll: 'Import all', notGeo: 'This PDF has no georeferencing: it will be added for viewing only (no aircraft position).', dup: 'Already in the library as',
      detected: 'Detected from', fromFile: 'the file name', fromPdf: 'the PDF title', nameAuto: 'Leave empty to build it from ICAO, type and title',
      disclaimer: 'For flight simulation only. Not for real-world aviation: using these charts or this software for real navigation or flight planning is prohibited.',
      aboutWhat: 'Open Charts manages aeronautical chart PDFs and shows them as a moving map inside Microsoft Flight Simulator.',
      aboutNotAffil: 'Independent project. Not affiliated with or endorsed by Microsoft, Asobo Studio, DECEA or any chart provider.',
      aboutData: 'Charts stay on your computer. The app downloads nothing from chart providers by itself; you get the files from the official pages and follow the usage policy of each provider.',
      aboutCredits: 'Credits and licences', aboutMap: 'Map tiles: (c) OpenStreetMap contributors (ODbL).',
      aboutLibs: 'Built with Go, PDFium (BSD), pdfcpu (Apache-2.0), wazero (Apache-2.0), Leaflet (BSD-2). Live position through the SimConnect SDK.',
      aboutLicense: 'Open source, MIT licence.', version: 'Version'
    },
    pt: {
      library: 'Biblioteca', sources: 'Fontes', groups: 'Grupos', back: '< Fontes',
      group: 'Grupo', newGroup: 'Novo grupo', groupName: 'Nome do grupo',
      search: 'Buscar por ICAO, tipo ou nome...', all: 'Todas', charts: 'cartas',
      addToGroup: '+ Adicionar ao grupo', inGroup: 'No grupo', addSelected: 'Adicionar selecionadas ao grupo',
      removeFromGroup: 'Tirar deste grupo', confirmDel: 'Excluir esta carta da biblioteca?', newName: 'Novo nome',
      rename: 'Renomear', del: 'Excluir', chartOnly: 'Só a carta', onMap: 'No mapa',
      thisGroup: 'Cartas deste grupo', noPins: 'Nada aqui ainda. Use "Adicionar ao grupo" na biblioteca.',
      noCharts: 'Nenhuma carta ainda. Abra Fontes para adicionar.', addCharts: 'Adicionar cartas (Fontes)',
      current: 'Atual', makeCurrent: 'Usar este grupo', pinnedN: 'fixadas',
      dropPdf: 'Solte aqui o que você baixou: PDFs (vários de uma vez) e/ou um pacote .zip. Arquivos conhecidos são nomeados e importados automaticamente. Ou', choose: 'escolha os arquivos',
      linksTitle: 'Links oficiais', linkHub: 'AISWEB: página de cartas', linkHubNote: 'clique em "Pacote completo" para baixar o pacote (.zip)', exVisual: 'Exemplo de carta visual',
      pkgFrom: 'Pacote', pkgChoose: 'Escolha os aeródromos que quer importar, ou importe tudo.', pkgNone: 'Nada deste pacote foi reconhecido.',
      pkgImportAll: 'Importar tudo', pkgNew: 'novas', pkgAlready: 'já importado', pkgPrev: 'Anterior', pkgNext: 'Próxima', pkgOf: 'de',
      pkgAllInfo: 'Prepara {n} cartas, cerca de {t}.', pkgAllWarn: 'Importar tudo prepara todas as cartas do pacote neste computador (cada PDF é desenhado como imagem). Pode levar {t}, e o computador ficará ocupado nesse tempo. Você pode deixar esta página aberta e continuar usando o aplicativo. Continuar?',
      patTitle: 'Padrão de nome dos seus PDFs', patHelp: 'Nomeie seus PDFs sempre do mesmo jeito e o aplicativo lê o aeródromo, o tipo, o título e a data pelo nome do arquivo, e uma pasta inteira é importada sem perguntar. Escreva o padrão com marcadores.',
      patLegend: 'Marcadores', patIcao: 'código do aeródromo (4 letras)', patType: 'tipo da carta (IAC, SID, STAR, ADC...)', patTitle2: 'o nome da carta',
      patDate: 'data de validade; um formato a fixa, por exemplo {date:DD-MM-YYYY} (padrão: AAAAMMDD ou AAAA-MM-DD). {cycle} lê um ciclo AIRAC, como 2610', patSkip: 'qualquer texto que você não precisa',
      patRules: '"_", "-" e espaço valem o mesmo, e maiúsculas não importam. Um padrão por linha; vale o primeiro que servir.',
      patExample: 'Exemplo: {icao}_{type}_{title}_{date:YYYYMMDD} lê SBSP_IAC_ILS-X-RWY-17R_20260806.pdf',
      patPlaceholder: '{icao}_{type}_{title}_{date:YYYYMMDD}', patTestLabel: 'Teste com um nome de arquivo', patTestPlaceholder: 'SBSP_IAC_ILS-X-RWY-17R_20260806.pdf',
      patSave: 'Salvar padrões', patSaved: 'Padrões salvos', patNoMatch: 'nenhum padrão serve para este nome', patResult: 'Lê',
      libAll: 'Todas as cartas', libAirports: 'Por aeródromo', libRoutes: 'Cartas de rota', noIcao: 'Sem aeródromo (cartas visuais)', airportsN: 'aeródromos',
      selN: '{n} selecionadas', selPage: 'Todas as {n} desta página estão selecionadas.', selAll: 'Selecionar todas as {n} cartas da busca', selClear: 'Limpar seleção',
      delSelected: 'Excluir selecionadas', confirmDelN: 'Excluir {n} cartas da biblioteca? Isso não pode ser desfeito.', deletedN: '{n} cartas excluídas',
      backAirports: '< Aeródromos', openAirport: 'Abrir', selectAllPage: 'Selecionar todas desta página',
      faaMetaTitle: 'Metadados do d-TPP (nomes dos arquivos)', faaMetaNone: 'Não carregado: os PDFs da FAA têm nome numérico, e sem os metadados do ciclo não dá para distingui-los.',
      faaMetaOk: 'Carregado: ciclo {cycle} ({from} a {to}), {charts} cartas de {airports} aeroportos.', faaMetaAdd: 'Adicionar os metadados (.xml, ou o zip "E")',
      faaMetaDone: 'Metadados carregados', faaMetaWhy: 'Baixe uma vez por ciclo (28 dias): fica dentro do DDTPPE_<data>.zip (cerca de 130 MB).',
      pkgEta: 'restantes', pkgDuration: { s: 'alguns segundos', m: '{n} min', h: '{h} h {m} min' },
      inbox: 'Pasta de entrada', inboxHelp: 'O que você adiciona aqui fica nesta pasta, no seu computador. Os PDFs são importados quando você clicar no botão; de um .zip, leia o pacote e escolha os aeródromos.',
      importInbox: 'Importar os PDFs da pasta de entrada', noInbox: 'A pasta de entrada está vazia.',
      imported: 'importadas', skipped: 'ignoradas', failed: 'com erro', done: 'Concluído', error: 'Erro',
      placesTitle: 'Ir direto para um aeródromo', placesHelp: 'Digite o código ou o nome: o link abre a página daquele aeródromo no ciclo em vigor, onde as cartas estão listadas.',
      placesInput: 'EGLL, heathrow, gatwick...', placesNone: 'Nenhum aeródromo encontrado.', placesOpen: 'Abrir a página das cartas', placesMore: 'Mais aeródromos combinam: digite mais do nome.',
      capPlaces: 'Por aeródromo',
      srcFilter: 'Filtrar pelo nome...', srcAllCountries: 'Todos os países', srcCountry: 'País', srcSource: 'Fonte', srcWhat: 'O que oferece',
      srcNone: 'Nenhuma fonte encontrada.', srcAnyCountry: 'Qualquer país', srcOpen: 'Abrir',
      builtin: 'Embutida', noAuto: 'Sem download automático', policyLink: 'Política de uso', official: 'Página oficial',
      howTo: 'Como obter cartas desta fonte', locate: 'Onde obter o pacote',
      locateHelp: 'Endereços derivados do padrão de nome; abrem no seu navegador e você baixa o arquivo.',
      unverified: 'não confirmado', package: 'Pacote baixado', pkgPath: 'Caminho do arquivo .zip (ou escolha um da pasta de entrada)',
      readPackage: 'Ler pacote', airports: 'aeródromos', importAirport: 'Importar',
      catalog: 'Catálogo', catalogHelp: 'Abra a página oficial, salve o PDF e use "Adicionar arquivo" na mesma linha.',
      openOfficial: '1. Abrir página oficial', addFile: '2. Adicionar arquivo', installed: 'Instalada', snapshot: 'Retrato de',
      tabCatalog: 'Cartas visuais (catálogo)', tabPackage: 'Aeródromos (pacote)', tabFiles: 'Adicionar arquivos',
      stepsFiles: 'Adicione o que você baixou', capCatalog: 'Catálogo', capPackage: 'Pacote', capLocate: 'Por data', capUpload: 'Envio',
      efbTitle: 'A tela que o EFB do simulador mostra, em uma aba própria',
      about: 'Sobre', setup: 'Configuração', efb: 'EFB', guide: 'Guia', nav: 'Navegação', routes: 'Rotas', mapTab: 'Mapa',
      whyManualTitle: 'Por que eu tenho que baixar as cartas manualmente?',
      whyManual: 'A maior parte dos provedores não permite a redistribuição, o que impede a gente de já entregar as cartas processadas e prontas. Muitos também proíbem qualquer tipo de download em massa ou script de extração automática. Para não infringir os termos de uso, vamos ser conservadores por enquanto: você baixa os arquivos e o Open Charts os processa no seu computador. No futuro podemos avaliar uma alternativa melhor.',
      guideIntro: 'A tela do simulador não mostra dicas ao passar o mouse nos botões, então aqui está o que cada um faz. Os números do desenho batem com a lista.',
      guideTop: 'Barra de cima', guideLeft: 'Coluna da esquerda: mexer na vista', guideRight: 'Coluna da direita: como a carta aparece', guideLists: 'Dentro das listas',
      diagTitle: 'Navegadores do visualizador', diagNone: 'Nenhum visualizador se apresentou ainda. Abra o visualizador (no simulador ou num navegador) e volte aqui.',
      diagSim: 'Simulador (EFB)', diagBrowser: 'Navegador', diagYes: 'sim', diagNo: 'não', diagRatio: 'Densidade de pixels',
      diagHelp: 'O que cada visualizador informou sobre si. O WebGL decide se um mapa vetorial (mais leve e mais nítido) é possível dentro do simulador.',
      baseTitle: 'Mapa base', baseLight: 'Claro (discreto)', baseStandard: 'OpenStreetMap padrão', baseDark: 'Escuro', baseNone: 'Sem mapa',
      aircraftTitle: 'Símbolo da aeronave', aircraftPlane: 'Avião', aircraftHeli: 'Helicóptero', aircraftColor: 'Cor',
      aircraftHelp: 'O que marca a aeronave no mapa do visualizador. Recarregue o visualizador para ver a mudança.',
      col_yellow: 'Amarelo', col_white: 'Branco', col_orange: 'Laranja', col_red: 'Vermelho', col_magenta: 'Magenta',
      col_blue: 'Azul', col_cyan: 'Ciano', col_green: 'Verde', col_black: 'Preto',
      baseHelp: 'O que aparece embaixo da carta no visualizador. O Claro é uma versão discreta, em cinza, do OpenStreetMap: carrega mais leve e não disputa atenção com a carta. Recarregue o visualizador para ver a mudança.',
      detailTitle: 'Detalhe das cartas', detailNormal: 'Normal', detailHigh: 'Alto',
      detailHelp: 'Vale para as cartas importadas daqui em diante. Alto fica mais nítido ao dar zoom (celular, telas grandes), mas gera arquivos maiores e demora mais. Para mudar as cartas que já estão na biblioteca, exclua e adicione de novo.',
      netTitle: 'Porta e rede local', portTitle: 'Porta do servidor', lanTitle: 'Rede local (tablet e celular)', portLabel: 'Porta', apply: 'Aplicar', portApplied: 'Porta alterada',
      portHelp: 'Padrão {n}. Troque se outro programa usar. Depois, atualize o app do EFB e reinicie o simulador.',
      portFixed: 'O endereço foi definido com -addr na linha de comando; estas opções ficam desativadas.',
      lanLabel: 'Outros dispositivos', lanOn: 'Permitido: tablet e celular na mesma rede Wi-Fi podem conectar', lanOff: 'Somente este computador',
      lanHelp: 'Leia o QR code com o tablet ou o celular. Eles veem e gerenciam cartas e grupos, mas nunca alteram este computador.',
      lanFirewall: 'Se o Windows perguntar sobre o firewall, permita o Open Charts em redes privadas.',
      lanNoAddress: 'Nenhum endereço de rede encontrado. Conecte-se a uma rede Wi-Fi ou cabeada.',
      lanVirtual: 'adaptador virtual', lanRotate: 'Gerar nova chave de acesso', lanRotateConfirm: 'Os dispositivos já conectados precisarão ler o QR code de novo. Continuar?',
      stEfbUrl: 'O app do EFB aponta para outro endereço',
      viewOnly: 'Só visualização', viewOnlyHelp: 'Sem georreferenciamento: dá para ver a carta, mas ela não tem a posição da aeronave e não vai sobre o mapa.',
      preview: 'Original', pdfOriginal: 'PDF original', chartImage: 'Imagem da carta', backLibrary: '< Biblioteca', openPdf: 'Abrir o PDF', notFound: 'Carta não encontrada',
      editChart: 'Editar propriedades', save: 'Salvar', cancel: 'Cancelar', saved: 'Salvo',
      editHelp: 'O nome exibido é gerado a partir de ICAO, tipo e título.',
      editWarnSource: 'A fonte e a georreferência não podem ser alteradas.',
      sUploading: 'Enviando o arquivo', sAnalyzing: 'Lendo o PDF e identificando a carta',
      sWaiting: 'Aguardando o arquivo anterior', sEngine: 'Iniciando o motor de PDF (só na primeira importação)',
      sRender: 'Renderizando e georreferenciando a carta', sSave: 'Salvando na biblioteca',
      aboutProject: 'Projeto e suporte', aboutRepo: 'Código-fonte no GitHub', aboutIssues: 'Ver issues',
      aboutBug: 'Reportar um bug', aboutFeature: 'Sugerir uma funcionalidade', aboutBeta: 'Esta é uma Open Beta: relatos de bugs são muito bem-vindos.',
      aboutLogs: 'Ao reportar um bug, anexe o arquivo de log',
      setupTitle: 'Configuração e status', stServer: 'Servidor do Open Charts', stRunning: 'Rodando em',
      stSim: 'Ligação com o simulador', stSimRecv: 'Recebendo a posição da aeronave', stSimConn: 'Conectado, aguardando posição (inicie um voo)',
      stSimWait: 'Aguardando o MSFS iniciar', stSimFixed: 'Posição fixa de teste (sem ligação com o simulador)',
      stInstall: 'Instalação do MSFS', stNoInstall: 'Nenhuma instalação do MSFS 2024 encontrada. Informe a pasta Community:',
      stEfb: 'App do EFB', stEfbNo: 'Não instalado', stEfbOld: 'Instalado, há atualização', stEfbOk: 'Instalado',
      btnInstall: 'Instalar no EFB', btnUpdate: 'Atualizar', btnRemove: 'Remover',
      stAuto: 'Iniciar com o MSFS', stAutoHelp: 'O Open Charts inicia quando o simulador inicia (sem janela).', btnEnable: 'Ativar', btnDisable: 'Desativar',
      stEfbLegacy: 'Versão antiga (Local Moving Map) encontrada: instalar substitui',
      stData: 'Pasta de dados', stRestart: 'Reinicie o simulador para a mudança no app do EFB valer.',
      edSteam: 'Steam', edStore: 'Microsoft Store',
      reviewTitle: 'Revise antes de importar', icao: 'ICAO', ctype: 'Tipo', ctitle: 'Título', validity: 'Ciclo AIRAC ou data', cname: 'Nome exibido',
      importOne: 'Importar', importAll: 'Importar todas', notGeo: 'Este PDF não tem georreferenciamento: será adicionado só para visualização (sem a posição da aeronave).', dup: 'Já está na biblioteca como',
      detected: 'Detectado a partir de', fromFile: 'do nome do arquivo', fromPdf: 'do título do PDF', nameAuto: 'Deixe vazio para montar com ICAO, tipo e título',
      disclaimer: 'Apenas para simulação aérea. Proibido o uso em aviação real: é vedado utilizar estas cartas ou este software para navegação ou planejamento de voo reais.',
      aboutWhat: 'O Open Charts gerencia cartas aeronáuticas em PDF e as exibe como mapa móvel dentro do Microsoft Flight Simulator.',
      aboutNotAffil: 'Projeto independente. Sem afiliação ou endosso da Microsoft, Asobo Studio, DECEA ou de qualquer provedor de cartas.',
      aboutData: 'As cartas ficam no seu computador. O aplicativo não baixa nada dos provedores sozinho; você obtém os arquivos nas páginas oficiais e segue a política de uso de cada provedor.',
      aboutCredits: 'Créditos e licenças', aboutMap: 'Mapas: (c) colaboradores do OpenStreetMap (ODbL).',
      aboutLibs: 'Feito com Go, PDFium (BSD), pdfcpu (Apache-2.0), wazero (Apache-2.0), Leaflet (BSD-2). Posição ao vivo pelo SimConnect SDK.',
      aboutLicense: 'Código aberto, licença MIT.', version: 'Versão'
    }
  };

  let lang = 'en';
  try { lang = localStorage.getItem('oc.lang') || ((navigator.language || 'en').toLowerCase().startsWith('pt') ? 'pt' : 'en'); } catch (e) { /* ignore */ }
  const qLang = new URLSearchParams(location.search).get('lang');
  if (qLang === 'pt' || qLang === 'en') lang = qLang;
  let VERSION = '0.6.0 Open Beta';   // replaced by what the running program reports (/api/version): '0.1.1-8fa9b5b' while developing
  const REPO = 'https://github.com/RCCSilva/msfs-efb-open-charts';
  const t = (k) => STR[lang][k] || STR.en[k] || k;
  const fill = (text, vars) => text.replace(/\{(\w+)\}/g, (m, k) => (k in vars ? vars[k] : m));
  // 45 -> "a few seconds", 600 -> "10 min", 7500 -> "2 h 5 min" (an estimate: never more precise than a minute)
  function fmtDuration(sec) {
    const d = STR[lang].pkgDuration || STR.en.pkgDuration;
    if (sec < 60) return d.s;
    const min = Math.max(1, Math.round(sec / 60));
    if (min < 60) return fill(d.m, { n: min });
    return fill(d.h, { h: Math.floor(min / 60), m: min % 60 });
  }
  const tx = (o) => (o && (o[lang] || o.en)) || '';

  // ---- helpers
  function h(tag, attrs, ...kids) {
    const n = document.createElement(tag);
    if (attrs) {
      for (const k of Object.keys(attrs)) {
        const v = attrs[k];
        if (v === null || v === undefined || v === false) continue;
        if (k.startsWith('on')) n.addEventListener(k.slice(2), v);
        else if (k === 'class') n.className = v;
        else if (k === 'value') n.value = v;
        else n.setAttribute(k, v === true ? '' : v);
      }
    }
    for (const kid of kids.flat(Infinity)) {
      if (kid === null || kid === undefined || kid === false) continue;
      n.appendChild(typeof kid === 'string' || typeof kid === 'number' ? document.createTextNode(String(kid)) : kid);
    }
    return n;
  }

  async function api(method, url, body) {
    const opt = { method, headers: {} };
    if (body instanceof FormData) opt.body = body;
    else if (body !== undefined) { opt.headers['Content-Type'] = 'application/json'; opt.body = JSON.stringify(body); }
    const r = await fetch(url, opt);
    let data = null;
    try { data = await r.json(); } catch (e) { /* no body */ }
    if (!r.ok) throw new Error((data && data.error) || r.statusText);
    return data;
  }

  let toastTimer = null;
  function toast(msg) {
    const el = document.getElementById('toast');
    el.textContent = msg;
    el.hidden = false;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => { el.hidden = true; }, 3500);
  }

  async function waitJob(id, onTick) {
    for (;;) {
      const j = await api('GET', '/api/jobs/' + encodeURIComponent(id));
      onTick(j);
      if (j.state === 'done') return j;
      await new Promise((r) => setTimeout(r, 400));
    }
  }

  function progress(j) {
    const pct = j.total ? Math.round((j.done / j.total) * 100) : 0;
    return h('div', null,
      h('div', { class: 'bar' }, h('i', { style: 'width:' + pct + '%' })),
      h('div', { class: 'muted' },
        `${j.done}/${j.total}  ·  ${j.ok} ${t('imported')}  ·  ${j.skipped} ${t('skipped')}  ·  ${j.failed} ${t('failed')}` +
        (j.etaSeconds > 0 ? '  ·  ~' + fmtDuration(j.etaSeconds) + ' ' + t('pkgEta') : '') +
        (j.current ? '  ·  ' + j.current : '')),
      h('div', null, j.items.filter((i) => i.status === 'error').map((i) => h('div', { class: 'badge bad' }, i.file + ': ' + i.message))));
  }

  // ---- state and routing
  const state = { groups: { current: '', groups: [] } };
  const view = document.getElementById('view');
  const currentGroup = () => state.groups.groups.find((f) => f.id === state.groups.current) || { id: '', name: '-', pins: [] };

  async function loadGroups() { state.groups = await api('GET', '/api/groups'); }

  function parseRoute() {
    const [name, arg] = (location.hash || '#library').slice(1).split('/');
    return { name: name || 'library', arg: arg ? decodeURIComponent(arg) : '' };
  }
  const go = (hash) => { if (location.hash === hash) render(); else location.hash = hash; };

  // A page is a real link (an <a>): it can be opened in a new tab with the middle button or ctrl+click.
  // A plain click is handled in place, without reloading the page.
  function pageLink(hash, cls, ...kids) {
    return h('a', { class: cls, href: location.pathname + location.search + hash, onclick: (e) => {
      if (e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return;
      e.preventDefault();
      go(hash);
    } }, ...kids);
  }

  async function render() {
    clearTimeout(setupTimer);
    const r = parseRoute();
    // the database is looked at on every page: when it cannot be used, every page says so
    let dbst = null;
    try { dbst = await api('GET', '/api/db/status'); } catch (e) { dbst = null; }   // only the computer itself may ask
    const dbBad = !!dbst && dbst.ok === false;
    document.body.classList.toggle('fullmap', r.name === 'map' && !dbBad);
    const tab = r.name === 'source' ? 'sources' : (r.name === 'chart' ? 'library' : r.name);
    document.getElementById('brand').replaceChildren(pageLink('#library', '', 'Open Charts'));
    if (state.local === undefined) {
      try { state.local = (await api('GET', '/api/health')).local !== false; } catch (e) { state.local = true; }
    }
    document.getElementById('tabs').replaceChildren(...['library', 'sources', 'groups', 'routes', 'map', 'nav', 'guide', 'setup', 'about'].filter((id) => id !== 'setup' || state.local).map((id) =>
      pageLink('#' + id, id === tab ? 'on' : '', t(id === 'map' ? 'mapTab' : id))).concat([
        // The EFB screen (same page the simulator shows) opens in its own tab, outside the simulator too.
        h('a', { class: 'navlink', href: '/viewer.html?lang=' + lang, target: '_blank', rel: 'noopener', title: t('efbTitle') }, t('efb') + ' \u2197')]));
    try {
      await loadGroups();
      drawGroupBar();
      drawFooter();
      if (dbBad && (r.name === 'routes' || r.name === 'map' || r.name === 'nav')) view.replaceChildren(h('h2', null, t(r.name === 'map' ? 'mapTab' : r.name)), dbBanner(dbst));
      else if (r.name === 'sources') await renderSources();
      else if (r.name === 'source') await renderSource(r.arg);
      else if (r.name === 'chart') await renderChart(r.arg);
      else if (r.name === 'groups') await renderGroups();
      else if (r.name === 'setup') await renderSetup();
      else if (r.name === 'routes') await renderRoutes();
      else if (r.name === 'map') await renderMap();
      else if (r.name === 'nav') await renderNav();
      else if (r.name === 'guide') renderGuide();
      else if (r.name === 'about') renderAbout();
      else await renderLibrary();
      if (dbBad && r.name !== 'setup' && !['routes', 'map', 'nav'].includes(r.name)) view.prepend(dbBanner(dbst));
    } catch (e) {
      view.replaceChildren(h('div', { class: 'card' }, h('span', { class: 'badge bad' }, e.message)));
    }
  }

  // ---- group bar: the group everything refers to (also the one the viewer uses)
  function drawGroupBar() {
    const sel = h('select', null, state.groups.groups.map((f) => h('option', { value: f.id }, `${f.name} (${f.pins.length})`)));
    sel.value = state.groups.current;
    sel.addEventListener('change', async () => {
      await api('PUT', '/api/groups/current', { id: sel.value });
      render();
    });
    document.getElementById('groupbar').replaceChildren(
      h('span', { class: 'lbl' }, t('group') + ':'), sel,
      h('button', { class: 'btn sm', onclick: async () => {
        const n = prompt(t('groupName'), `${t('group')} ${state.groups.groups.length + 1}`);
        if (n && n.trim()) { await api('POST', '/api/groups', { name: n.trim() }); render(); }
      } }, '+ ' + t('newGroup')));
  }

  // ---- A chart: its properties on one side, the original PDF on the other. The server validates and
  // generates the name; this only collects.
  async function renderChart(id) {
    let c;
    try { c = await api('GET', '/api/charts/' + encodeURIComponent(id)); } catch (e) { c = null; }
    if (!c || !c.id) {
      view.replaceChildren(h('div', { class: 'card' }, pageLink('#library', '', t('backLibrary')), h('p', null, h('span', { class: 'badge bad' }, t('notFound')))));
      return;
    }
    const q = encodeURIComponent(c.id);
    const field = (label, val, ph) => {
      const input = h('input', { type: 'text', value: val || '', placeholder: ph || '' });
      return [h('label', { class: 'muted' }, label), input];
    };
    const [lI, icao] = field(t('icao'), c.icao, 'SBSP');
    const [lT, ctype] = field(t('ctype'), c.type, 'IAC');
    const [lTi, title] = field(t('ctitle'), c.title);
    const [lV, validity] = field(t('validity'), c.cycle || c.validity, '2610 / YYYY-MM-DD');
    const err = h('div');
    const save = h('button', { class: 'btn primary', onclick: async () => {
      save.disabled = true;
      err.replaceChildren();
      try {
        await api('PATCH', '/api/charts/' + q, { icao: icao.value, type: ctype.value, title: title.value, validity: validity.value });
        toast(t('saved'));
        render();
      } catch (e) {
        err.replaceChildren(h('span', { class: 'badge bad' }, e.message));
        save.disabled = false;
      }
    } }, t('save'));
    // the PDF as the browser shows it, or the picture the viewer uses (also the way out when the
    // browser has no PDF viewer)
    const previewBox = h('div');
    const showPreview = (kind) => {
      previewBox.replaceChildren(kind === 'pdf'
        ? h('iframe', { class: 'preview', src: `/charts/${q}/source.pdf`, title: t('pdfOriginal') })
        : h('img', { class: 'preview', src: `/charts/${q}/chart.png`, alt: t('chartImage') }));
    };
    showPreview(c.hasSource ? 'pdf' : 'img');
    const preview = previewBox;
    view.replaceChildren(
      h('div', { class: 'row', style: 'margin-bottom:10px' }, pageLink('#library', '', t('backLibrary'))),
      h('div', { class: 'chartpage' },
        h('div', { class: 'card' },
          h('h2', null, c.name),
          h('div', { class: 'muted' }, `${c.cycle ? 'AIRAC ' + c.cycle : (c.validity || '')}  ·  ${c.source}  ·  ${c.width}×${c.height}`),
          c.unreferenced ? h('div', { style: 'margin-top:8px' }, h('span', { class: 'badge warn' }, t('viewOnly')), h('span', { class: 'muted' }, t('viewOnlyHelp'))) : null,
          h('div', { class: 'formgrid' }, lI, icao, lT, ctype, lTi, title, lV, validity),
          h('div', { class: 'muted' }, t('editHelp')),
          h('div', { class: 'muted' }, t('editWarnSource')),
          err,
          h('div', { class: 'row', style: 'margin-top:12px' },
            save),
          h('div', { class: 'row', style: 'margin-top:12px' },
            h('a', { class: 'btn', href: `/viewer.html?chart=${q}&mode=chart&lang=${lang}`, target: '_blank' }, t('chartOnly')),
            c.unreferenced ? null : h('a', { class: 'btn', href: `/viewer.html?chart=${q}&mode=overlay&lang=${lang}`, target: '_blank' }, t('onMap')),
            c.hasSource ? h('a', { class: 'btn', href: `/charts/${q}/source.pdf`, target: '_blank' }, t('openPdf')) : null,
            h('span', { class: 'grow' }),
            h('button', { class: 'btn danger', onclick: async () => {
              if (!confirm(t('confirmDel'))) return;
              try {
                await api('POST', '/api/library/delete', { ids: [c.id] });
                toast(fill(t('deletedN'), { n: 1 }));
                go('#library');
              } catch (e) { toast(e.message); }
            } }, t('del')))),
        h('div', { class: 'card previewcard' },
          h('div', { class: 'row', style: 'margin-bottom:8px' }, h('h3', { class: 'grow', style: 'margin:0' }, t('preview')),
            c.hasSource ? h('button', { class: 'btn sm', onclick: () => showPreview('pdf') }, 'PDF') : null,
            h('button', { class: 'btn sm', onclick: () => showPreview('img') }, t('chartImage'))),
          preview)));
  }

  // ---- Library: all the charts, or by airport; paged, with selection and bulk actions
  async function renderLibrary() {
    let group = currentGroup();
    let pins = new Set(group.pins);
    const PAGE = 50;
    const selected = new Set();
    const names = new Map();           // id -> chart, for the group panel
    let mode = 'all';                  // 'all' | 'airports' | 'routes' (charts with no airport)
    let airport = null;                // an ICAO ('' = no ICAO) while looking inside one airport
    let page = 0;
    let total = 0;
    let items = [];
    let airports = [];
    let types = [];

    const q = h('input', { type: 'text', placeholder: t('search') });
    const type = h('select');
    const count = h('span', { class: 'muted' });
    const body = h('tbody');
    const head = h('thead');
    const pager = h('div', { class: 'row', style: 'margin-top:8px' });
    const banner = h('div', { class: 'muted', style: 'margin:6px 0' });
    const bar = h('div', { class: 'row', style: 'margin:6px 0' });
    const seg = h('div', { class: 'seg' });
    const crumb = h('div', { class: 'row' });
    const side = h('div', { class: 'card side' });

    async function setPin(id, on) {
      await api(on ? 'PUT' : 'DELETE', `/api/groups/${encodeURIComponent(group.id)}/pins/${encodeURIComponent(id)}`);
      on ? pins.add(id) : pins.delete(id);
    }

    // the group panel needs the names of the pinned charts, whichever page they are on
    async function loadPinned() {
      const list = await api('GET', `/api/charts?group=${encodeURIComponent(group.id)}&limit=1000`);
      for (const c of list) names.set(c.id, c);
    }

    function drawSide() {
      const list = group.pins.filter((id) => pins.has(id)).concat([...pins].filter((id) => !group.pins.includes(id)));
      side.replaceChildren(
        h('h3', null, `${t('thisGroup')} (${pins.size})`),
        h('div', { class: 'muted', style: 'margin-bottom:6px' }, group.name),
        ...(list.length ? list.map((id) => {
          const c = names.get(id);
          return h('div', { class: 'item' },
            h('span', { class: 't', title: c ? c.name : id }, c ? c.name : id),
            h('button', { class: 'btn sm', title: t('removeFromGroup'), onclick: async () => { await setPin(id, false); draw(); } }, '✕'));
        }) : [h('div', { class: 'muted' }, t('noPins'))]));
    }

    // ---- the charts table
    function row(c) {
      const inGroup = pins.has(c.id);
      const check = h('input', { type: 'checkbox', checked: selected.has(c.id), onchange: (e) => {
        e.target.checked ? selected.add(c.id) : selected.delete(c.id);
        drawControls();
      } });
      const open = '#chart/' + encodeURIComponent(c.id);
      return h('tr', { class: 'clickable', onclick: (e) => {
        // anything inside the row that does something of its own keeps doing only that
        if (e.target.closest('a, button, input, summary, details, select')) return;
        go(open);
      } },
        h('td', null, check),
        h('td', null, pageLink(open, 'chartlink', h('strong', null, c.name)),
          c.unreferenced ? h('span', { class: 'badge warn', title: t('viewOnlyHelp'), style: 'margin-left:6px' }, t('viewOnly')) : null,
          h('div', { class: 'muted' }, `${c.cycle ? 'AIRAC ' + c.cycle : (c.validity || '')}  ·  ${c.source}`)),
        h('td', { class: 'actions' },
          h('button', { class: 'btn sm' + (inGroup ? ' in' : ''), title: inGroup ? t('removeFromGroup') : '', onclick: async () => {
            await setPin(c.id, !inGroup);
            draw();
          } }, inGroup ? '✓ ' + t('inGroup') : t('addToGroup')), ' ',
          h('details', { class: 'more' }, h('summary', { class: 'btn sm' }, '⋯'),
            h('div', { class: 'menu' },
              h('a', { href: '/viewer.html?chart=' + encodeURIComponent(c.id) + '&mode=chart&lang=' + lang, target: '_blank' }, t('chartOnly')),
              c.unreferenced ? null : h('a', { href: '/viewer.html?chart=' + encodeURIComponent(c.id) + '&mode=overlay&lang=' + lang, target: '_blank' }, t('onMap')),
              pageLink(open, '', t('editChart')),
              h('button', { class: 'danger', onclick: async () => {
                if (confirm(t('confirmDel'))) { await deleteIds([c.id]); }
              } }, t('del'))))));
    }

    async function deleteIds(ids) {
      try {
        const r = await api('POST', '/api/library/delete', { ids });
        state.groups = r.groups;
        group = currentGroup();
        pins = new Set(group.pins);
        ids.forEach((id) => { selected.delete(id); names.delete(id); });
        drawGroupBar();
        toast(fill(t('deletedN'), { n: r.deleted }));
        await load();
      } catch (e) { toast(e.message); }
    }

    function drawControls() {
      const onPage = items.length > 0 && items.every((c) => selected.has(c.id));
      bar.replaceChildren(...(selected.size === 0 ? [] : [
        h('strong', null, fill(t('selN'), { n: selected.size })),
        h('button', { class: 'btn primary sm', onclick: async () => {
          for (const id of selected) if (!pins.has(id)) await setPin(id, true);
          selected.clear();
          drawGroupBar();
          toast(t('done'));
          draw();
        } }, t('addSelected')),
        h('button', { class: 'btn sm danger', onclick: async () => {
          if (confirm(fill(t('confirmDelN'), { n: selected.size }))) await deleteIds([...selected]);
        } }, t('delSelected')),
        h('button', { class: 'btn sm', onclick: () => { selected.clear(); draw(); } }, t('selClear'))]));
      // "everything on this page is selected" -> offer everything the search matches
      banner.replaceChildren(...(onPage && total > items.length && selected.size < total ? [
        h('span', null, fill(t('selPage'), { n: items.length }) + ' '),
        h('a', { href: '#', onclick: async (e) => { e.preventDefault(); await selectAllMatching(); } }, fill(t('selAll'), { n: total }))] : []));
    }

    async function selectAllMatching() {
      const r = await api('GET', '/api/library?ids=1&' + filters());
      r.ids.forEach((id) => selected.add(id));
      draw();
    }

    function filters() {
      const parts = ['q=' + encodeURIComponent(q.value), 'type=' + encodeURIComponent(type.value)];
      if (mode === 'routes') parts.push('icao=-');   // no airport: route charts, visual charts...
      else if (airport !== null) parts.push('icao=' + encodeURIComponent(airport));
      return parts.join('&');
    }

    function pagerFor(totalItems) {
      const pages = Math.max(1, Math.ceil(totalItems / PAGE));
      page = Math.min(page, pages - 1);
      pager.replaceChildren(
        h('button', { class: 'btn sm', disabled: page === 0, onclick: () => { page -= 1; load(); } }, '‹ ' + t('pkgPrev')),
        h('span', { class: 'muted' }, `${totalItems ? page * PAGE + 1 : 0}–${Math.min(totalItems, (page + 1) * PAGE)} ${t('pkgOf')} ${totalItems}`),
        h('button', { class: 'btn sm', disabled: page >= pages - 1, onclick: () => { page += 1; load(); } }, t('pkgNext') + ' ›'));
    }

    // ---- loading and drawing
    async function load() {
      if (mode === 'airports' && airport === null) {
        const r = await api('GET', '/api/airports?q=' + encodeURIComponent(q.value));
        airports = r.airports;
        draw();
        return;
      }
      const r = await api('GET', `/api/library?${filters()}&offset=${page * PAGE}&limit=${PAGE}`);
      items = r.items;
      total = r.total;
      types = r.types;
      for (const c of items) names.set(c.id, c);
      draw();
    }

    function drawTypeSelect() {
      const cur = type.value;
      type.replaceChildren(h('option', { value: '' }, t('all')), ...types.map((x) => h('option', { value: x }, x)));
      type.value = cur;
    }

    function draw() {
      seg.replaceChildren(
        h('button', { class: mode === 'all' ? 'on' : '', onclick: () => { mode = 'all'; airport = null; page = 0; selected.clear(); load(); } }, t('libAll')),
        h('button', { class: mode === 'airports' ? 'on' : '', onclick: () => { mode = 'airports'; airport = null; page = 0; selected.clear(); load(); } }, t('libAirports')),
        h('button', { class: mode === 'routes' ? 'on' : '', onclick: () => { mode = 'routes'; airport = null; page = 0; selected.clear(); load(); } }, t('libRoutes')));
      crumb.replaceChildren(...(mode === 'airports' && airport !== null ? [
        h('button', { class: 'btn sm', onclick: () => { airport = null; page = 0; selected.clear(); load(); } }, t('backAirports')),
        h('strong', null, airport === '' ? t('noIcao') : airport)] : []));
      drawTypeSelect();

      if (mode === 'airports' && airport === null) {
        // the aerodromes: open one to see its charts
        count.textContent = `${airports.length} ${t('airportsN')}`;
        head.replaceChildren();
        bar.replaceChildren();
        banner.replaceChildren();
        const slice = airports.slice(page * PAGE, (page + 1) * PAGE);
        body.replaceChildren(...(slice.length ? slice.map((a) => h('tr', null,
          h('td', null, h('strong', null, a.icao || t('noIcao'))),
          h('td', { class: 'muted' }, Object.keys(a.types).sort().map((k) => `${k} ${a.types[k]}`).join(' · ')),
          h('td', { class: 'actions' }, h('span', { class: 'muted' }, `${a.count} ${t('charts')}`), ' ',
            h('button', { class: 'btn sm primary', onclick: () => { airport = a.icao; page = 0; selected.clear(); load(); } }, t('openAirport'))))) :
          [h('tr', null, h('td', { colspan: 3, class: 'muted' }, t('noCharts')))]));
        pagerFor(airports.length);
      } else {
        count.textContent = `${total} ${t('charts')}`;
        const allOnPage = items.length > 0 && items.every((c) => selected.has(c.id));
        head.replaceChildren(h('tr', null,
          h('th', null, h('input', { type: 'checkbox', title: t('selectAllPage'), 'aria-label': t('selectAllPage'), checked: allOnPage, onchange: (e) => {
            items.forEach((c) => (e.target.checked ? selected.add(c.id) : selected.delete(c.id)));
            draw();
          } })),
          h('th', { colspan: 2, class: 'muted' }, t('selectAllPage'))));
        body.replaceChildren(...(items.length ? items.map(row) :
          [h('tr', null, h('td', { colspan: 3, class: 'muted' }, t('noCharts')))]));
        pagerFor(total);
        drawControls();
      }
      drawSide();
    }

    let timer = null;
    q.addEventListener('input', () => { clearTimeout(timer); timer = setTimeout(() => { page = 0; load(); }, 150); });
    type.addEventListener('change', () => { page = 0; load(); });

    view.replaceChildren(
      h('div', { class: 'row' }, h('h2', { class: 'grow' }, t('library')),
        h('a', { class: 'btn', href: '#sources' }, t('addCharts'))),
      h('div', { class: 'split' },
        h('div', { class: 'card' },
          seg,
          h('div', { class: 'row' }, h('div', { class: 'grow' }, q), type, count),
          crumb, bar, banner,
          h('table', null, head, body), pager),
        side));
    await loadPinned();
    await load();
  }

  // ---- Sources: list, then one page per source
  const sourcesView = { q: '', country: '' };

  async function renderSources() {
    const list = await api('GET', '/api/sources');
    const capName = { catalog: 'capCatalog', package: 'capPackage', locate: 'capLocate', places: 'capPlaces' };
    // the countries that have a source, by name in the language of the page
    const countries = {};
    list.forEach((x) => { if (x.country) countries[x.country] = tx(x.countryName) || x.country; });
    const codes = Object.keys(countries).sort((p, q) => countries[p].localeCompare(countries[q], lang));
    if (sourcesView.country && !countries[sourcesView.country]) sourcesView.country = '';

    const q = h('input', { type: 'text', placeholder: t('srcFilter'), value: sourcesView.q, class: 'grow' });
    const sel = h('select', null, h('option', { value: '' }, t('srcAllCountries')), codes.map((c) => h('option', { value: c }, countries[c])));
    sel.value = sourcesView.country;
    const body = h('div');

    const draw = () => {
      const text = sourcesView.q.trim().toLowerCase();
      const shown = list.filter((x) => {
        if (sourcesView.country && x.country !== sourcesView.country && !(x.builtin && !x.country)) return false;
        if (!text) return true;
        return (tx(x.name) + ' ' + tx(x.description) + ' ' + (countries[x.country] || '')).toLowerCase().includes(text);
      });
      body.replaceChildren(shown.length === 0 ? h('p', { class: 'muted' }, t('srcNone')) : h('div', { class: 'card', style: 'padding:0' },
        h('table', { class: 'srctable' },
          h('thead', null, h('tr', null, h('th', null, t('srcSource')), h('th', null, t('srcCountry')), h('th', null, t('srcWhat')), h('th'))),
          h('tbody', null, shown.map((x) => h('tr', { class: 'clickable', onclick: () => go('#source/' + encodeURIComponent(x.id)) },
            h('td', null, h('a', { href: '#source/' + encodeURIComponent(x.id), onclick: (e) => e.stopPropagation() }, h('strong', null, tx(x.name))),
              h('div', { class: 'muted', style: 'font-size:12px;max-width:520px' }, tx(x.description))),
            h('td', null, x.country ? countries[x.country] : h('span', { class: 'muted' }, t('srcAnyCountry'))),
            h('td', null,
              x.builtin ? h('span', { class: 'badge' }, t('builtin')) : null,
              (x.capabilities || []).map((c) => h('span', { class: 'badge' }, t(capName[c] || c))),
              !x.policy.automatedFetch && !x.builtin ? h('span', { class: 'badge warn' }, t('noAuto')) : null),
            h('td', { class: 'actions' }, h('a', { class: 'btn sm', href: '#source/' + encodeURIComponent(x.id), onclick: (e) => e.stopPropagation() }, t('srcOpen')))))))));
    };
    q.addEventListener('input', () => { sourcesView.q = q.value; draw(); });
    sel.addEventListener('change', () => { sourcesView.country = sel.value; draw(); });

    view.replaceChildren(h('h2', null, t('sources')),
      h('div', { class: 'warnbox', style: 'margin-bottom:16px' }, h('strong', null, t('whyManualTitle')), h('div', null, t('whyManual'))),
      h('div', { class: 'row', style: 'margin-bottom:12px' }, q, sel), body);
    draw();
  }

  async function renderSource(id) {
    const list = await api('GET', '/api/sources');
    const s = list.find((x) => x.id === id);
    if (!s) { view.replaceChildren(h('div', { class: 'card' }, 'Unknown source')); return; }
    const caps = s.capabilities || [];

    // 1. who the source is and how to get its charts (the guide is the source's own, in order)
    const head = h('div', { class: 'card' },
      h('a', { class: 'back', href: '#sources' }, t('back')),
      h('div', { class: 'row' },
        h('h2', { class: 'grow', style: 'margin:0' }, tx(s.name)),
        s.builtin ? h('span', { class: 'badge' }, t('builtin')) : null,
        !s.policy.automatedFetch && !s.builtin ? h('span', { class: 'badge warn' }, t('noAuto')) : null,
        s.policy.url ? h('a', { href: s.policy.url, target: '_blank', rel: 'noreferrer' }, t('policyLink')) : null),
      h('div', { class: 'muted' }, tx(s.description)),
      h('div', { class: 'muted' }, tx(s.policy.notes)),
      s.guide && s.guide.length ? h('div', null, h('h3', { style: 'margin-top:12px' }, t('howTo')),
        h('ol', { class: 'steps' }, s.guide.map((g) => h('li', null, tx(g))))) : null);

    // 2. official links with examples: the page of the source, one visual chart, the package of the cycle.
    // They only open the official site; the file is downloaded by the user.
    const links = h('div', { class: 'card' }, h('h3', null, t('linksTitle')));
    const linkRow = (label, href, note, unverified) => h('div', { class: 'row' },
      h('a', { href, target: '_blank', rel: 'noreferrer' }, label),
      note ? h('span', { class: 'muted' }, note) : null,
      unverified ? h('span', { class: 'badge warn' }, t('unverified')) : null);
    if (s.homepage) links.appendChild(linkRow(tx(s.homepageLabel) || t('linkHub'), s.homepage, tx(s.homepageNote) || ''));
    // each request answers with its rows; they are added in a fixed order whatever arrives first
    const none = () => [];
    const visual = (kind) => caps.includes('catalog')
      ? api('GET', '/api/sources/' + s.id + '/catalog?limit=1&type=' + kind).then((r) =>
        r.items.map((it) => linkRow(t('exVisual') + ' ' + it.type + ': ' + it.name, it.url, it.file))).catch(none)
      : Promise.resolve([]);
    const located = caps.includes('locate')
      ? api('GET', '/api/sources/' + s.id + '/locate').then((r) =>
        r.locations.filter((l) => l.verified).map((l) => linkRow(tx(l.label), l.url, l.file))).catch(none)
      : Promise.resolve([]);
    const groups = await Promise.all([located, visual('REA'), visual('WAC')]);
    const rows = groups.flat();
    rows.forEach((r) => links.appendChild(r));
    const linksShown = !!s.homepage || rows.length > 0;

    // 3. what the user downloaded goes here, PDFs and packages alike
    view.replaceChildren(head, ...(s.id === 'manual' ? [await patternCard()] : []),
      ...(caps.includes('places') ? [placesCard(s)] : []), ...(linksShown ? [links] : []),
      ...(s.id === 'faa-us' ? [await faaMetadataCard()] : []), filesPanel(s));
  }

  // A provider that keeps its charts aerodrome by aerodrome: type the code or the name and the link
  // goes straight to that aerodrome's page (the app opens nothing and downloads nothing).
  function placesCard(s) {
    const input = h('input', { type: 'text', placeholder: t('placesInput'), autocomplete: 'off', class: 'grow' });
    const out = h('div', { style: 'margin-top:10px' });
    let seq = 0;
    let timer = null;
    const run = async () => {
      const q = input.value.trim();
      const my = ++seq;
      if (q.length < 2) { out.replaceChildren(); return; }
      let r;
      try { r = await api('GET', '/api/sources/' + encodeURIComponent(s.id) + '/places?q=' + encodeURIComponent(q)); } catch (e) { return; }
      if (my !== seq) return;
      if (!r.places.length) { out.replaceChildren(h('div', { class: 'muted' }, t('placesNone'))); return; }
      const rows = r.places.map((p) => h('div', { class: 'row', style: 'padding:6px 0;border-top:1px solid var(--line)' },
        h('span', { class: 'grow' }, h('strong', null, p.icao), ' ', h('span', { class: 'muted' }, p.name)),
        h('a', { class: 'btn sm', href: p.url, target: '_blank', rel: 'noopener noreferrer' }, t('placesOpen') + ' ↗')));
      if (r.truncated) rows.push(h('div', { class: 'muted', style: 'margin-top:6px' }, t('placesMore')));
      out.replaceChildren(...rows);
    };
    input.addEventListener('input', () => { clearTimeout(timer); timer = setTimeout(run, 200); });
    return h('div', { class: 'card' }, h('h3', null, t('placesTitle')), h('p', { class: 'muted' }, t('placesHelp')),
      h('div', { class: 'row' }, input), out);
  }

  // Upload PDFs on behalf of a source (recognition and naming use that source's rules).
  // ---- per-file progress: the server reports the stage and the percentage, the UI shows them
  const STAGE_LABEL = { waiting: 'sWaiting', engine: 'sEngine', render: 'sRender', save: 'sSave' };

  // POST a FormData with upload progress. onProgress(0-100) while sending, onUploaded() once the
  // body is sent and the server is working on it.
  function xhrPost(url, fd, onProgress, onUploaded) {
    return new Promise((resolve, reject) => {
      const x = new XMLHttpRequest();
      x.open('POST', url);
      x.upload.onprogress = (e) => { if (e.lengthComputable && onProgress) onProgress(Math.round((e.loaded / e.total) * 100)); };
      x.upload.onload = () => { if (onUploaded) onUploaded(); };
      x.onload = () => {
        let data = null;
        try { data = JSON.parse(x.responseText); } catch (e) { /* no body */ }
        if (x.status >= 200 && x.status < 300) resolve(data);
        else reject(new Error((data && data.error) || x.statusText || 'error'));
      };
      x.onerror = () => reject(new Error('network error'));
      x.send(fd);
    });
  }

  // A row with the file name, what is happening, a bar and the elapsed time.
  function progressRow(name) {
    const label = h('span', { class: 'muted' });
    const clock = h('span', { class: 'muted' });
    const fill = h('i');
    const bar = h('div', { class: 'bar' }, fill);
    const el = h('div', { class: 'card prow' },
      h('div', { class: 'row' }, h('strong', { class: 'grow' }, name), label, clock), bar);
    const t0 = Date.now();
    const tick = setInterval(() => { clock.textContent = Math.round((Date.now() - t0) / 1000) + ' s'; }, 500);
    const row = {
      el,
      update(text, pct) {
        label.textContent = text;
        if (pct === null || pct === undefined) { bar.classList.add('indet'); fill.style.width = ''; } else {
          bar.classList.remove('indet'); fill.style.width = pct + '%';
        }
      },
      ok(text) {
        clearInterval(tick);
        el.replaceChildren(h('span', { class: 'badge ok' }, t('imported')), text);
      },
      warn(text) {
        clearInterval(tick);
        el.replaceChildren(h('span', { class: 'badge warn' }, t('skipped')), text);
      },
      fail(text) {
        clearInterval(tick);
        el.replaceChildren(h('span', { class: 'badge bad' }, t('error')), text);
      },
      stop() { clearInterval(tick); }
    };
    row.update(t('sUploading'), 0);
    return row;
  }

  // Start an import job on the server (async) and follow it, showing its stage in the row.
  // Resolves with the job's item; rejects with the server's message on failure.
  async function runImport(fd, row, snapshotPromise) {
    let snap = snapshotPromise ? await snapshotPromise : null;
    if (!snap) {
      fd.append('async', '1');
      snap = await api('POST', '/api/import', fd);
    }
    for (;;) {
      const label = STAGE_LABEL[snap.stage] ? t(STAGE_LABEL[snap.stage]) : t('sWaiting');
      // while rendering the server cannot say how far it is: show motion and the clock instead
      row.update(label, snap.state === 'done' ? 100 : (snap.stage === 'render' ? null : snap.percent));
      if (snap.state === 'done') break;
      await new Promise((r) => setTimeout(r, 250));
      snap = await api('GET', '/api/jobs/' + encodeURIComponent(snap.id));
    }
    const item = snap.items[0];
    if (!item) throw new Error('no result');
    if (item.status === 'ok') return item;
    const e = new Error(item.message || item.status);
    e.skipped = item.status === 'skipped';
    throw e;
  }

  // Dropped PDFs are inspected first; the user confirms or edits what was detected.
  function reviewCard(s, ins) {
    const sg = ins.suggestion;
    const field = (label, val, ph) => {
      const input = h('input', { type: 'text', value: val || '', placeholder: ph || '' });
      return [h('label', { class: 'muted' }, label), input];
    };
    const [lI, icao] = field(t('icao'), sg.icao);
    const [lT, ctype] = field(t('ctype'), sg.type);
    const [lTi, title] = field(t('ctitle'), sg.title);
    const [lV, validity] = field(t('validity'), sg.cycle || sg.validity, '2610 / YYYY-MM-DD');
    const [lN, name] = field(t('cname'), '', t('nameAuto'));
    const status = h('div');
    const btn = h('button', { class: 'btn primary', onclick: async () => {
      btn.disabled = true;
      const fd = new FormData();
      fd.append('token', ins.token);
      fd.append('source', s.id);
      fd.append('icao', icao.value.trim());
      fd.append('type', ctype.value.trim());
      fd.append('title', title.value.trim());
      fd.append('validity', validity.value.trim());
      fd.append('name', name.value.trim());
      const row = progressRow(ins.file);
      status.replaceChildren(row.el);
      try {
        const item = await runImport(fd, row);
        card.replaceChildren(h('span', { class: 'badge ok' }, t('imported')), item.name);
        card.done = true;
        row.stop();
      } catch (e) {
        row.fail(e.message);
        btn.disabled = false;
      }
    } }, t('importOne'));
    const from = ins.recognizedBy || !ins.viewport ? t('fromFile') : t('fromPdf');
    const card = h('div', { class: 'card review' },
      h('div', { class: 'row' }, h('strong', { class: 'grow' }, ins.file), h('span', { class: 'muted' }, t('detected') + ' ' + from)),
      !ins.georeferenced ? h('div', { class: 'badge warn' }, t('notGeo')) : null,
      ins.duplicate ? h('div', { class: 'badge warn' }, t('dup') + ' ' + ins.duplicate.name) : null,
      h('div', { class: 'formgrid' }, lI, icao, lT, ctype, lTi, title, lV, validity, lN, name),
      h('div', { class: 'row' }, btn, status));
    card.importNow = () => { if (!card.done && !btn.disabled) btn.click(); };
    return card;
  }

  // The FAA names its PDFs by number; the metadata of the cycle says what each one is.
  async function faaMetadataCard() {
    const card = h('div', { class: 'card' });
    const file = h('input', { type: 'file', accept: '.xml,.zip', hidden: true });
    const status = h('div', { class: 'muted' });
    const msg = h('div', { style: 'margin-top:6px' });

    async function refresh() {
      let st = { loaded: false };
      try { st = await api('GET', '/api/sources/faa-us/metadata'); } catch (e) { /* shown as not loaded */ }
      status.replaceChildren(st.loaded
        ? h('span', { class: 'badge ok' }, fill(t('faaMetaOk'), st))
        : h('span', { class: 'badge warn' }, t('faaMetaNone')));
    }

    async function add(f) {
      msg.replaceChildren(h('span', { class: 'muted' }, '...'));
      try {
        if (/\.xml$/i.test(f.name)) {
          const fd = new FormData();
          fd.append('file', f);
          await xhrPost('/api/sources/faa-us/metadata', fd);
        } else {
          const fd = new FormData();
          fd.append('file', f);
          const saved = await xhrPost('/api/packages/upload', fd);
          await api('POST', '/api/sources/faa-us/metadata', { path: saved.path });
        }
        msg.replaceChildren(h('span', { class: 'badge ok' }, t('faaMetaDone')));
        await refresh();
      } catch (e) {
        msg.replaceChildren(h('span', { class: 'badge bad' }, e.message));
      }
    }
    file.addEventListener('change', () => { if (file.files[0]) add(file.files[0]); file.value = ''; });
    await refresh();
    card.append(h('h3', null, t('faaMetaTitle')), status,
      h('div', { class: 'muted', style: 'margin-top:6px' }, t('faaMetaWhy')),
      h('div', { class: 'row', style: 'margin-top:8px' },
        h('button', { class: 'btn', onclick: () => file.click() }, t('faaMetaAdd'))),
      msg, file);
    return card;
  }

  // The file-name patterns of the user's own PDFs: written once, used for every file dropped.
  async function patternCard() {
    let cur;
    try { cur = await api('GET', '/api/naming/patterns'); } catch (e) { return h('div'); }
    const box = h('textarea', { rows: 3, placeholder: t('patPlaceholder'), style: 'width:100%;font-family:monospace' });
    box.value = cur.patterns.join('\n');
    const sample = h('input', { type: 'text', placeholder: t('patTestPlaceholder'), style: 'width:100%' });
    const result = h('div', { class: 'muted', style: 'margin-top:6px;min-height:20px' });

    // what the patterns read from the sample: the first line that fits, as the server would do
    let timer = null;
    async function test() {
      const name = sample.value.trim();
      if (!name) { result.textContent = ''; return; }
      const lines = box.value.split('\n').map((l) => l.trim()).filter(Boolean);
      for (const line of lines) {
        const r = await api('POST', '/api/naming/test', { pattern: line, sample: name });
        if (r.ok) {
          const parts = [['ICAO', r.icao], ['Type', r.type], ['Title', r.title], ['Date', r.validity]].filter((x) => x[1]);
          result.replaceChildren(h('span', { class: 'badge ok' }, t('patResult') + ': ' + parts.map((x) => x[0] + ' ' + x[1]).join(' \u00b7 ')));
          return;
        }
        if (r.error && r.error !== 'no match') { result.replaceChildren(h('span', { class: 'badge bad' }, line + ': ' + r.error)); return; }
      }
      result.replaceChildren(h('span', { class: 'badge warn' }, lines.length ? t('patNoMatch') : t('patNoMatch')));
    }
    const schedule = () => { clearTimeout(timer); timer = setTimeout(test, 250); };
    box.addEventListener('input', schedule);
    sample.addEventListener('input', schedule);

    const legend = [['{icao}', t('patIcao')], ['{type}', t('patType')], ['{title}', t('patTitle2')], ['{date}', t('patDate')], ['{skip}', t('patSkip')]];
    return h('div', { class: 'card' },
      h('h3', null, t('patTitle')),
      h('div', { class: 'muted' }, t('patHelp')),
      h('div', { style: 'margin:8px 0' },
        h('strong', null, t('patLegend') + ':'),
        h('table', null, h('tbody', null, legend.map(([k, v]) => h('tr', null, h('td', null, h('code', null, k)), h('td', { class: 'muted' }, v)))))),
      h('div', { class: 'muted' }, t('patRules')),
      h('div', { class: 'muted', style: 'margin-bottom:8px' }, t('patExample')),
      box,
      h('div', { style: 'margin-top:8px' }, h('label', { class: 'muted' }, t('patTestLabel')), sample, result),
      h('div', { class: 'row', style: 'margin-top:8px' },
        h('button', { class: 'btn primary', onclick: async () => {
          try {
            await api('PUT', '/api/naming/patterns', { patterns: box.value.split('\n').map((l) => l.trim()).filter(Boolean) });
            toast(t('patSaved'));
          } catch (e) { toast(e.message); }
        } }, t('patSave'))));
  }

  function filesPanel(s) {
    const reviews = h('div');
    const packages = h('div');
    const drop = h('div', { class: 'drop' });
    const file = h('input', { type: 'file', accept: '.pdf,.zip,.tif,.tiff,application/pdf,application/zip', multiple: true, hidden: true });
    drop.append(t('dropPdf') + ' ', h('a', { href: '#', onclick: (e) => { e.preventDefault(); file.click(); } }, t('choose')));
    const cards = [];
    const isZip = (name) => /\.zip$/i.test(name);
    const isXml = (name) => /\.xml$/i.test(name);

    // The d-TPP metafile itself (an .xml): it only names the files of the cycle.
    async function addMetadataFile(f) {
      const row = progressRow(f.name);
      reviews.appendChild(row.el);
      const fd = new FormData();
      fd.append('file', f);
      try {
        await xhrPost('/api/sources/faa-us/metadata', fd, (p) => row.update(t('sUploading'), p), () => row.update(t('sAnalyzing'), null));
        row.ok(t('faaMetaDone'));
      } catch (e) {
        row.fail(f.name + ': ' + e.message);
      }
    }

    // A package is kept by the program (importing an aerodrome reads it again) and shown right here.
    async function addPackage(f) {
      const row = progressRow(f.name);
      reviews.appendChild(row.el);
      const fd = new FormData();
      fd.append('file', f);
      try {
        const saved = await xhrPost('/api/packages/upload', fd, (p) => row.update(t('sUploading'), p), () => row.update(t('sAnalyzing'), null));
        row.stop();
        reviews.removeChild(row.el);
        packages.prepend(packageView(s, saved.path, saved.name));
      } catch (e) {
        row.fail(f.name + ': ' + e.message);
      }
    }

    async function inspectFiles(files) {
      for (const f of files) {
        if (isXml(f.name) && s.id === 'faa-us') { await addMetadataFile(f); continue; }
        if (isZip(f.name)) { await addPackage(f); continue; }
        const row = progressRow(f.name);
        reviews.appendChild(row.el);
        const fd = new FormData();
        fd.append('file', f);
        fd.append('source', s.id);
        try {
          const ins = await xhrPost('/api/import/inspect', fd,
            (p) => row.update(t('sUploading'), p), () => row.update(t('sAnalyzing'), null));
          if (ins.action === 'import') { // the server decided: import as it identified the file
            const body = new FormData();
            body.append('token', ins.token);
            body.append('source', s.id);
            try {
              const item = await runImport(body, row);
              row.ok(item.name);
            } catch (e) {
              if (e.skipped) row.warn(f.name + ': ' + e.message); else row.fail(f.name + ': ' + e.message);
            }
            continue;
          }
          const card = reviewCard(s, ins);
          cards.push(card);
          row.stop();
          reviews.replaceChild(card, row.el);
        } catch (e) {
          row.fail(f.name + ': ' + e.message);
        }
      }
      if (cards.length > 1 && !reviews.querySelector('.importall')) {
        reviews.prepend(h('div', { class: 'row importall' },
          h('button', { class: 'btn primary', onclick: () => cards.forEach((c) => c.importNow()) }, t('importAll'))));
      }
    }
    file.addEventListener('change', () => { inspectFiles([...file.files]); file.value = ''; });
    drop.addEventListener('dragover', (e) => { e.preventDefault(); drop.classList.add('over'); });
    drop.addEventListener('dragleave', () => drop.classList.remove('over'));
    drop.addEventListener('drop', (e) => { e.preventDefault(); drop.classList.remove('over'); inspectFiles([...e.dataTransfer.files]); });

    return h('div', null,
      h('div', { class: 'card' }, h('h3', null, t('stepsFiles')), drop, file),
      reviews,
      packages);
  }

  // A downloaded package: import everything, or choose aerodromes from a paged list.
  const PKG_PAGE = 20;
  function packageView(s, path, name) {
    const out = h('div', { class: 'muted' }, '...');
    const prog = h('div');
    const card = h('div', { class: 'card' }, h('h3', null, t('pkgFrom') + ': ' + name), out, prog);

    // one import at a time: the progress shows under the list, and the buttons wait for it
    let busy = false;
    async function run(body) {
      if (busy) return;
      busy = true;
      card.classList.add('busy');
      try {
        const j = await api('POST', `/api/sources/${s.id}/package/import`, Object.assign({ path }, body));
        await waitJob(j.id, (x) => prog.replaceChildren(progress(x)));
        toast(t('done'));
      } catch (e) {
        prog.replaceChildren(h('span', { class: 'badge bad' }, e.message));
      } finally {
        busy = false;
        card.classList.remove('busy');
      }
    }

    api('GET', `/api/sources/${s.id}/package?path=${encodeURIComponent(path)}`).then((p) => {
      if (p.kind === 'metadata') { // the "E" zip: not charts, the names of the cycle's files
        return api('POST', '/api/sources/faa-us/metadata', { path: p.path }).then((st) => {
          out.replaceChildren(h('span', { class: 'badge ok' }, t('faaMetaDone') + ': ' + fill(t('faaMetaOk'), st)));
        });
      }
      if (!p.airports || !p.airports.length) { out.replaceChildren(h('span', { class: 'badge warn' }, t('pkgNone'))); return; }
      const newBy = p.newByAirport || {};
      const filter = h('input', { type: 'text', placeholder: 'ICAO' });
      const body = h('tbody');
      const pager = h('div', { class: 'row' });
      let page = 0;

      const draw = () => {
        const q = filter.value.trim().toUpperCase();
        const rows = p.airports.filter((a) => !q || a.icao.startsWith(q));
        const pages = Math.max(1, Math.ceil(rows.length / PKG_PAGE));
        page = Math.min(page, pages - 1);
        body.replaceChildren(...rows.slice(page * PKG_PAGE, (page + 1) * PKG_PAGE).map((a) => {
          const fresh = newBy[a.icao] || 0;
          return h('tr', null,
            h('td', null, a.icao ? h('strong', null, a.icao) : h('strong', null, t('noIcao'))),
            h('td', { class: 'muted' }, Object.keys(a.types).sort().map((k) => `${k} ${a.types[k]}`).join(' \u00b7 ')),
            h('td', { class: 'actions' }, fresh === 0
              ? h('span', { class: 'badge ok' }, '\u2713 ' + t('pkgAlready'))
              : h('button', { class: 'btn sm primary', onclick: () => run(a.icao ? { icao: a.icao } : { noAirport: true }) },
                `${t('importAirport')} (${fresh === a.count ? a.count : fresh + ' ' + t('pkgNew')})`)));
        }));
        pager.replaceChildren(
          h('button', { class: 'btn sm', disabled: page === 0, onclick: () => { page -= 1; draw(); } }, '\u2039 ' + t('pkgPrev')),
          h('span', { class: 'muted' }, `${rows.length ? page * PKG_PAGE + 1 : 0}\u2013${Math.min(rows.length, (page + 1) * PKG_PAGE)} ${t('pkgOf')} ${rows.length}`),
          h('button', { class: 'btn sm', disabled: page >= pages - 1, onclick: () => { page += 1; draw(); } }, t('pkgNext') + ' \u203a'));
      };
      filter.addEventListener('input', () => { page = 0; draw(); });
      draw();

      // import everything that is not in the library yet, with the time it should take here
      const when = fmtDuration(p.estimateSeconds || 0);
      const all = p.new > 0
        ? h('div', { class: 'row', style: 'margin:8px 0' },
          h('button', { class: 'btn primary', onclick: () => {
            if (confirm(fill(t('pkgAllWarn'), { t: when }))) run({ all: true });
          } }, `${t('pkgImportAll')} (${p.new})`),
          h('span', { class: 'muted' }, fill(t('pkgAllInfo'), { n: p.new, t: when })))
        : h('div', { class: 'muted', style: 'margin:8px 0' }, '\u2713 ' + t('pkgAlready'));

      out.replaceChildren(
        h('div', { class: 'muted' }, `${p.charts} ${t('charts')} \u00b7 ${p.airports.length} ${t('airports')}${p.date ? ' \u00b7 ' + p.date : ''}`),
        all,
        h('div', { class: 'muted' }, t('pkgChoose')),
        h('div', { class: 'row' }, h('div', { class: 'grow' }, filter)), h('table', null, body), pager);
    }).catch((e) => out.replaceChildren(h('span', { class: 'badge bad' }, e.message)));
    return card;
  }

  // ---- Groups: all groups and what is pinned in each
  async function renderGroups() {
    const all = await api('GET', '/api/charts?limit=1000');
    const byId = new Map(all.map((c) => [c.id, c]));
    const cards = state.groups.groups.map((f) => h('div', { class: 'card' },
      h('div', { class: 'row' },
        h('h3', { class: 'grow' }, `${f.name} (${f.pins.length} ${t('pinnedN')})`),
        f.id === state.groups.current ? h('span', { class: 'badge ok' }, t('current')) :
          h('button', { class: 'btn sm', onclick: async () => { await api('PUT', '/api/groups/current', { id: f.id }); render(); } }, t('makeCurrent')),
        h('button', { class: 'btn sm', onclick: async () => {
          const n = prompt(t('newName'), f.name);
          if (n && n.trim()) { await api('PATCH', '/api/groups/' + encodeURIComponent(f.id), { name: n.trim() }); render(); }
        } }, t('rename')),
        h('button', { class: 'btn sm danger', disabled: state.groups.groups.length < 2, onclick: async () => {
          if (confirm(t('confirmDel'))) { await api('DELETE', '/api/groups/' + encodeURIComponent(f.id)); render(); }
        } }, t('del'))),
      f.pins.length ? f.pins.map((id) => {
        const c = byId.get(id);
        return h('div', { class: 'row' }, h('span', { class: 'grow' }, c ? c.name : id),
          h('button', { class: 'btn sm', onclick: async () => {
            await api('DELETE', `/api/groups/${encodeURIComponent(f.id)}/pins/${encodeURIComponent(id)}`);
            render();
          } }, '✕'));
      }) : h('div', { class: 'muted' }, t('noPins'))));
    view.replaceChildren(h('h2', null, t('groups')), ...cards);
  }

  // ---- Setup: status checklist, EFB app install, start with the simulator
  async function renderSetup() {
    clearTimeout(setupTimer);
    const st = await api('GET', '/api/setup');
    // what the simulator in use can do: without an app of its own there is nothing to install
    let simInfo = { capabilities: { inSimApp: true } };
    try { simInfo = await api('GET', '/api/sim'); } catch (e) { /* an older program: assume it can */ }
    const row = (label, value, extra) => h('div', { class: 'row setuprow' },
      h('div', { class: 'setuplbl' }, label), h('div', { class: 'grow' }, value), extra || null);
    const ok = (txt) => h('span', null, h('span', { class: 'badge ok' }, '\u2713'), ' ', txt);
    const warn = (txt) => h('span', null, h('span', { class: 'badge warn' }, '!'), ' ', txt);

    let simText = warn(t('stSimWait'));
    if (st.sim.source === 'fixed') simText = h('span', null, h('span', { class: 'badge' }, 'i'), ' ', t('stSimFixed'));
    else if (st.sim.receiving) simText = ok(t('stSimRecv'));
    else if (st.sim.connected) simText = warn(t('stSimConn'));

    const cards = [h('div', { class: 'card' },
      h('h3', null, t('setupTitle')),
      row(t('stServer'), ok(t('stRunning') + ' ' + st.serverURL)),
      row(t('stSim'), simText),
      row(t('stData'), h('span', { class: 'muted' }, st.dataDir)))];

  // ---- Local network and port. The server validates and applies; this only collects and shows.
  // Returns two sections: the server port, and the local network (with its QR codes).
  async function networkCards(st) {
    let net;
    try { net = await api('GET', '/api/settings'); } catch (e) { return []; }
    const portBody = h('div');
    const lanBody = h('div');
    const portInput = h('input', { type: 'text', value: String(net.port), style: 'width:110px' });
    const apply = h('button', { class: 'btn primary', disabled: net.fixed, onclick: async () => {
      const p = parseInt(portInput.value, 10);
      try {
        await api('PUT', '/api/settings', { port: p });
        toast(t('portApplied'));
        // the page itself is served on the old port: go to the new address
        const here = new URL(location.href);
        here.port = String(p);
        setTimeout(() => { location.href = here.toString(); }, 600);
      } catch (e) { toast(e.message); }
    } }, t('apply'));

    const lanBtn = h('button', { class: 'btn sm' + (net.lan ? '' : ' primary'), disabled: net.fixed, onclick: async () => {
      try { await api('PUT', '/api/settings', { lan: !net.lan }); toast(t('done')); } catch (e) { toast(e.message); }
      renderSetup();
    } }, net.lan ? t('btnDisable') : t('btnEnable'));

    portBody.append(
      h('div', { class: 'row setuprow' }, h('div', { class: 'setuplbl' }, t('portLabel')),
        h('div', { class: 'grow' }, h('div', { class: 'row' }, portInput, apply)),
      ),
      h('div', { class: 'muted' }, t('portHelp').replace('{n}', String(net.defaultPort))),
      ...(net.fixed ? [h('div', { class: 'badge warn' }, t('portFixed'))] : []));

    lanBody.append(
      h('div', { class: 'row setuprow' }, h('div', { class: 'setuplbl' }, t('lanLabel')),
        h('div', { class: 'grow' }, net.lan ? h('span', null, h('span', { class: 'badge ok' }, '✓'), ' ', t('lanOn')) : h('span', { class: 'muted' }, t('lanOff'))),
        lanBtn));

    if (net.lan) {
      lanBody.append(h('div', { class: 'muted' }, t('lanHelp')), h('div', { class: 'muted' }, t('lanFirewall')));
      const real = net.addresses.filter((a) => !a.virtual);
      const list = real.length ? real : net.addresses;   // virtual adapters only when nothing else exists
      if (!list.length) lanBody.append(h('div', { class: 'badge warn' }, t('lanNoAddress')));
      const wrap = h('div', { class: 'qrgrid' });
      list.forEach((a) => wrap.append(h('div', { class: 'qrcard' },
        h('img', { src: a.qr + '&t=' + Date.now(), alt: 'QR', width: 168, height: 168 }),
        h('div', null, h('strong', null, a.ip)),
        h('div', { class: 'muted' }, a.name + (a.virtual ? ' · ' + t('lanVirtual') : '')),
        h('div', { class: 'muted urlbox' }, a.url))));
      lanBody.append(wrap,
        h('div', { class: 'row', style: 'margin-top:10px' },
          h('button', { class: 'btn sm', onclick: async () => {
            if (!confirm(t('lanRotateConfirm'))) return;
            try { await api('POST', '/api/lan/token'); toast(t('done')); } catch (e) { toast(e.message); }
            renderSetup();
          } }, t('lanRotate'))));
    }
    return [
      h('div', { class: 'card' }, h('h3', null, t('portTitle')), portBody),
      h('div', { class: 'card' }, h('h3', null, t('lanTitle')), lanBody)
    ];
  }

  async function diagCard() {
    let list;
    try { list = await api('GET', '/api/client-info'); } catch (e) { return null; }
    const yes = (v) => h('span', { class: 'badge ' + (v ? 'ok' : 'warn') }, v ? t('diagYes') : t('diagNo'));
    const rows = list.map((c) => h('div', { class: 'setuprow' },
      h('div', { class: 'row' },
        h('strong', { class: 'grow' }, c.kind === 'simulator' ? t('diagSim') : t('diagBrowser')),
        h('span', { class: 'muted' }, new Date(c.at).toLocaleString())),
      h('div', { class: 'row' }, h('span', null, 'WebGL '), yes(c.webgl), h('span', null, ' WebGL2 '), yes(c.webgl2),
        h('span', { class: 'muted' }, ' · ' + t('diagRatio') + ' ' + c.dpr + ' · ' + c.width + '×' + c.height + (c.touch ? ' · touch' : ''))),
      h('div', { class: 'muted urlbox' }, (c.renderer ? c.renderer + ' · ' : '') + c.ua)));
    return h('div', { class: 'card' }, h('h3', null, t('diagTitle')),
      h('div', { class: 'muted', style: 'margin-bottom:8px' }, t('diagHelp')),
      ...(rows.length ? rows : [h('div', { class: 'muted' }, t('diagNone'))]));
  }

  async function basemapCard() {
    let net;
    try { net = await api('GET', '/api/settings'); } catch (e) { return null; }
    const pick = (value, label) => h('button', {
      class: 'btn sm' + (net.basemap === value ? ' primary' : ''),
      onclick: async () => {
        try { await api('PUT', '/api/settings', { basemap: value }); toast(t('done')); } catch (e) { toast(e.message); }
        renderSetup();
      } }, label);
    return h('div', { class: 'card' }, h('h3', null, t('baseTitle')),
      h('div', { class: 'row' }, pick('light', t('baseLight')), pick('standard', t('baseStandard')),
        pick('dark', t('baseDark')), pick('none', t('baseNone'))),
      h('div', { class: 'muted', style: 'margin-top:8px' }, t('baseHelp')));
  }

  // The aircraft symbol (same shapes as the viewer's; the colour list comes from the server).
  const SVG_NS = 'http://www.w3.org/2000/svg';
  const AIRCRAFT_PATHS = {
    plane: ['M32 4L35 10L36 24L58 40L58 45L36 38L35 50L42 57L42 61L32 58L22 61L22 57L29 50L28 38L6 45L6 40L28 24L29 10Z'],
    helicopter: ['M29.5 36h5v20h-5z', 'M23 54h18v5H23z', 'M32 8c8.5 0 13 8 13 18.5S40.5 41 32 41s-13-4-13-14.5S23.5 8 32 8z',
      'M8 13l4-3.5 44 38-4 3.5z', 'M56 13l-4-3.5-44 38 4 3.5z']
  };
  function aircraftSvg(shape, hex, size) {
    const svg = document.createElementNS(SVG_NS, 'svg');
    svg.setAttribute('viewBox', '0 0 64 64');
    svg.setAttribute('width', size);
    svg.setAttribute('height', size);
    for (const d of AIRCRAFT_PATHS[shape] || AIRCRAFT_PATHS.plane) {
      const p = document.createElementNS(SVG_NS, 'path');
      p.setAttribute('d', d);
      p.setAttribute('fill', hex);
      p.setAttribute('stroke', '#0b1f33');
      p.setAttribute('stroke-width', '3');
      p.setAttribute('stroke-linejoin', 'round');
      svg.appendChild(p);
    }
    return svg;
  }

  async function aircraftCard() {
    let net;
    try { net = await api('GET', '/api/settings'); } catch (e) { return null; }
    if (!net.aircraftColors) return null;
    const set = async (patch) => {
      try { await api('PUT', '/api/settings', patch); toast(t('done')); } catch (e) { toast(e.message); }
      renderSetup();
    };
    const shapeBtn = (value, label) => h('button', {
      class: 'btn sm' + (net.aircraft === value ? ' primary' : ''),
      onclick: () => set({ aircraft: value }) }, label);
    const current = (net.aircraftColors.find((c) => c.id === net.aircraftColor) || net.aircraftColors[0]).hex;
    const swatches = net.aircraftColors.map((c) => h('button', {
      class: 'btn sm', title: t('col_' + c.id), 'aria-label': t('col_' + c.id),
      style: 'width:30px;height:30px;padding:0;border-radius:50%;background:' + c.hex + ';border:' +
        (c.id === net.aircraftColor ? '3px solid #2369be' : '1px solid #888'),
      onclick: () => set({ aircraftColor: c.id }) }));
    return h('div', { class: 'card' }, h('h3', null, t('aircraftTitle')),
      h('div', { class: 'row', style: 'align-items:center' },
        h('span', { style: 'display:inline-block;width:56px;height:56px;background:#dfe6ea;border-radius:8px;text-align:center;margin-right:10px' },
          aircraftSvg(net.aircraft, current, 52)),
        shapeBtn('plane', t('aircraftPlane')), shapeBtn('helicopter', t('aircraftHeli'))),
      h('div', { class: 'row', style: 'margin-top:8px' }, h('span', { class: 'muted', style: 'margin-right:6px' }, t('aircraftColor') + ':'), ...swatches),
      h('div', { class: 'muted', style: 'margin-top:8px' }, t('aircraftHelp')));
  }

  async function detailCard() {
    let net;
    try { net = await api('GET', '/api/settings'); } catch (e) { return null; }
    const pick = (value, label) => h('button', {
      class: 'btn sm' + (net.detail === value ? ' primary' : ''),
      onclick: async () => {
        try { await api('PUT', '/api/settings', { detail: value }); toast(t('done')); } catch (e) { toast(e.message); }
        renderSetup();
      } }, label);
    return h('div', { class: 'card' }, h('h3', null, t('detailTitle')),
      h('div', { class: 'row' }, pick('normal', t('detailNormal')), pick('high', t('detailHigh'))),
      h('div', { class: 'muted', style: 'margin-top:8px' }, t('detailHelp')));
  }

    cards.push(...await networkCards(st));
    const bCard = await basemapCard();
    if (bCard) cards.push(bCard);
    const aCard = await aircraftCard();
    if (aCard) cards.push(aCard);
    const gCard = await diagCard();
    if (gCard) cards.push(gCard);
    const dCard = await detailCard();
    if (dCard) cards.push(dCard);

    async function act(fn, done) {
      try { await fn(); toast(done || t('done')); } catch (e) { toast(e.message); }
      renderSetup();
    }

    for (const inst of st.installs) {
      const efb = inst.efb;
      const efbText = !efb.installed ? warn(efb.legacy ? t('stEfbLegacy') : t('stEfbNo')) :
        efb.urlMismatch ? warn(t('stEfbUrl') + ': ' + efb.serverURL + ' → ' + st.serverURL) :
        efb.upToDate ? ok(t('stEfbOk') + ' (' + (efb.build || efb.version) + ')') :
          warn(t('stEfbOld') + ' (' + (efb.build || efb.version || '?') + ' \u2192 ' + ((st.program && st.program.display) || st.efbVersion) + ')');
      const efbBtns = h('span', null,
        h('button', { class: 'btn sm primary', onclick: () => act(() => api('POST', '/api/setup/efb', { community: inst.community }), t('stRestart')) },
          efb.installed ? t('btnUpdate') : t('btnInstall')), ' ',
        efb.installed ? h('button', { class: 'btn sm danger', onclick: () => act(() => api('DELETE', '/api/setup/efb?community=' + encodeURIComponent(inst.community))) }, t('btnRemove')) : null);
      cards.push(h('div', { class: 'card' },
        h('h3', null, t('stInstall') + ' \u2014 ' + (inst.edition === 'store' ? t('edStore') : t('edSteam'))),
        h('div', { class: 'muted', style: 'margin-bottom:6px' }, inst.community),
        h('div', { class: 'muted', style: 'margin-bottom:6px' }, t('version') + ': ' + ((st.program && st.program.display) || '?') + '  ·  EFB: ' + (efb.build || efb.version || '-')),
        row(t('stEfb'), efbText, efbBtns),
        row(t('stAuto'), h('span', null, inst.autoStart ? ok(t('stEfbOk')) : h('span', { class: 'muted' }, t('stAutoHelp'))),
          h('button', { class: 'btn sm' + (inst.autoStart ? '' : ' primary'),
            onclick: () => act(() => api('PUT', '/api/setup/autostart', { exeXml: inst.exeXml, enabled: !inst.autoStart })) },
            inst.autoStart ? t('btnDisable') : t('btnEnable')))));
    }

    if (!st.installs.length && simInfo.capabilities.inSimApp) {
      const path = h('input', { type: 'text', placeholder: 'C:\\...\\Packages\\Community' });
      cards.push(h('div', { class: 'card' }, h('h3', null, t('stInstall')), h('div', { class: 'muted' }, t('stNoInstall')),
        h('div', { class: 'row', style: 'margin-top:8px' }, h('div', { class: 'grow' }, path),
          h('button', { class: 'btn primary', onclick: () => act(() => api('POST', '/api/setup/efb', { community: path.value }), t('stRestart')) }, t('btnInstall')))));
    }
    cards.push(...await databaseCards());
    view.replaceChildren(h('h2', null, t('setup')), ...cards);
  }


  // ---- Setup > database: the file that holds the navigation data, its copies, starting over and
  // restoring a copy; and the loads from the simulator, which are made from here.
  const SD = {
    en: {
      title: 'Navigation database', intro: 'The file where Open Charts keeps the airports, fixes, airways, your own points and your saved routes. Copies of it are kept in the backups folder next to it.',
      file: 'File', size: 'Size', schema: 'Version of the structure', stateOk: 'Working', problemWord: 'Problem',
      corruptTitle: 'The navigation database is corrupted',
      corruptText: 'Open Charts cannot use it. It has to be set aside and started again from scratch: the airports and fixes are loaded again from the simulator. The damaged file is kept in the backups folder.',
      newerTitle: 'The navigation database is from a newer version',
      newerText: 'It was made by a newer version of Open Charts, so this one does not touch it. Update Open Charts. Starting over would set that file aside and begin a new one.',
      failedTitle: 'The navigation database could not be updated',
      failedText: 'The update to the new version failed and the file was left as it was (a copy was made before). You can start over: the file is kept in the backups folder.',
      goSetup: 'Open the Setup page', bannerNoDb: 'Until this is solved, the Navigation, Routes and Map tabs cannot be used.',
      backupNow: 'Make a backup now', backupMade: 'Backup saved', reset: 'Start over...',
      resetConfirm: 'Start the navigation database over?\n\nEverything in it is lost: airports, fixes, airways, your own points and your saved routes. A backup is saved first, in the backups folder, and you can restore it later.',
      resetConfirmBroken: 'Set the damaged database aside and start over?\n\nEverything that was in it is lost, including your own points and saved routes. The damaged file is kept in the backups folder.',
      resetDone: 'Done: a new database was created',
      backupsTitle: 'Backups', backupsEmpty: 'No backup yet.', restore: 'Restore', remove: 'Remove',
      restoreConfirm: 'Replace the current database with this backup?\n\nWhat is in the database now is lost, except for a copy that is saved first. If the backup is from an older version it is updated.',
      removeConfirm: 'Remove this backup file? It cannot be undone.', restored: 'Restored',
      importBackup: 'Restore from a file...', importHow: 'Choose a database file (.db) that you saved before. It is checked, and if it is a healthy navigation database it takes the place of the current one (a copy of the current one is saved first).',
      importConfirm: 'Replace the current database with this file?\n\nWhat is in the database now is lost, except for a copy that is saved first.',
      working: 'Working...',
      kinds: { schema: 'before an update', manual: 'made by you', reset: 'before starting over', restore: 'before restoring', corrupt: 'damaged file set aside', imported: 'a file you brought' },
      loadSection: 'Load from the simulator', loadAirportTitle: 'Load one airport', loadAirportHow: 'Asks the simulator for one airport and stores it.',
      needDb: 'The database is not in use: solve the problem above first.'
    },
    pt: {
      title: 'Banco de dados de navegação', intro: 'O arquivo onde o Open Charts guarda os aeroportos, fixos, aerovias, os seus pontos e as suas rotas salvas. As cópias dele ficam na pasta backups, ao lado.',
      file: 'Arquivo', size: 'Tamanho', schema: 'Versão da estrutura', stateOk: 'Funcionando', problemWord: 'Problema',
      corruptTitle: 'O banco de dados de navegação está corrompido',
      corruptText: 'O Open Charts não consegue usá-lo. Ele precisa ser posto de lado e recomeçado do zero: os aeroportos e fixos são carregados de novo do simulador. O arquivo danificado fica guardado na pasta backups.',
      newerTitle: 'O banco de dados de navegação é de uma versão mais nova',
      newerText: 'Ele foi feito por uma versão mais nova do Open Charts, então esta não mexe nele. Atualize o Open Charts. Recomeçar poria esse arquivo de lado e começaria um novo.',
      failedTitle: 'Não foi possível atualizar o banco de dados de navegação',
      failedText: 'A atualização para a versão nova falhou e o arquivo ficou como estava (uma cópia foi feita antes). Você pode recomeçar: o arquivo fica guardado na pasta backups.',
      goSetup: 'Abrir a página Setup', bannerNoDb: 'Enquanto isso não for resolvido, as abas Navegação, Rotas e Mapa não funcionam.',
      backupNow: 'Fazer uma cópia agora', backupMade: 'Cópia salva', reset: 'Recomeçar...',
      resetConfirm: 'Recomeçar o banco de dados de navegação?\n\nTudo o que há nele se perde: aeroportos, fixos, aerovias, os seus pontos e as suas rotas salvas. Uma cópia é salva antes, na pasta backups, e você pode restaurá-la depois.',
      resetConfirmBroken: 'Pôr o banco danificado de lado e recomeçar?\n\nTudo o que havia nele se perde, inclusive os seus pontos e rotas salvas. O arquivo danificado fica guardado na pasta backups.',
      resetDone: 'Pronto: um banco novo foi criado',
      backupsTitle: 'Cópias de segurança', backupsEmpty: 'Nenhuma cópia ainda.', restore: 'Restaurar', remove: 'Remover',
      restoreConfirm: 'Trocar o banco de dados atual por esta cópia?\n\nO que há no banco agora se perde, exceto por uma cópia que é salva antes. Se a cópia for de uma versão mais antiga, ela é atualizada.',
      removeConfirm: 'Remover este arquivo de cópia? Não dá para desfazer.', restored: 'Restaurado',
      importBackup: 'Restaurar de um arquivo...', importHow: 'Escolha um arquivo de banco de dados (.db) que você guardou antes. Ele é verificado e, se for um banco de navegação saudável, toma o lugar do atual (uma cópia do atual é salva antes).',
      importConfirm: 'Trocar o banco de dados atual por este arquivo?\n\nO que há no banco agora se perde, exceto por uma cópia que é salva antes.',
      working: 'Trabalhando...',
      kinds: { schema: 'antes de uma atualização', manual: 'feita por você', reset: 'antes de recomeçar', restore: 'antes de restaurar', corrupt: 'arquivo danificado posto de lado', imported: 'um arquivo que você trouxe' },
      loadSection: 'Carregar do simulador', loadAirportTitle: 'Carregar um aeroporto', loadAirportHow: 'Pede ao simulador um aeroporto e o guarda.',
      needDb: 'O banco não está em uso: resolva o problema acima primeiro.'
    }
  };
  const sd = (k) => (SD[lang] && SD[lang][k]) || SD.en[k] || k;
  const fmtBytes = (n) => (n >= 1 << 30 ? (n / (1 << 30)).toFixed(1) + ' GB' : n >= 1 << 20 ? (n / (1 << 20)).toFixed(1) + ' MB' : Math.max(1, Math.round(n / 1024)) + ' KB');
  const fmtWhen = (iso) => { try { return new Date(iso).toLocaleString(lang === 'pt' ? 'pt-BR' : 'en-US'); } catch (e) { return iso; } };
  let setupTimer = null;

  // what is wrong with the database, in words (the banner and the Setup card use it)
  function dbProblemText(p) {
    const kind = p && p.kind;
    if (kind === 'corrupt') return { title: sd('corruptTitle'), text: sd('corruptText') };
    if (kind === 'newer') return { title: sd('newerTitle'), text: sd('newerText') };
    return { title: sd('failedTitle'), text: sd('failedText') };
  }

  // shown at the top of every page while the database cannot be used
  function dbBanner(st) {
    const w = dbProblemText(st.problem);
    return h('div', { class: 'warnbox dbbanner', style: 'margin-bottom:12px' },
      h('strong', null, w.title), h('div', { style: 'margin:4px 0' }, w.text), h('div', { class: 'muted' }, sd('bannerNoDb')),
      state.local ? h('div', { style: 'margin-top:8px' }, h('a', { class: 'btn primary', href: '#setup' }, sd('goSetup'))) : null);
  }

  // the card with the file, and the buttons that make a copy or start over
  function dbStateCard(st) {
    const busy = h('div', { class: 'muted', style: 'margin-top:8px' });
    const run = async (fn, done) => {
      busy.textContent = sd('working');
      try { await fn(); toast(done); } catch (e) { toast(e.message); }
      render();
    };
    const w = st.ok ? null : dbProblemText(st.problem);
    return h('div', { class: 'card' },
      h('h3', null, sd('title')),
      h('p', { class: 'muted' }, sd('intro')),
      h('div', { class: 'muted', style: 'margin-bottom:6px' }, sd('file') + ': ', h('code', null, st.path || '?')),
      st.size ? h('div', { class: 'muted', style: 'margin-bottom:6px' }, sd('size') + ': ' + fmtBytes(st.size)) : null,
      st.ok ? h('div', { class: 'muted', style: 'margin-bottom:6px' }, sd('schema') + ': ' + st.schema + '  \u00b7  ', h('span', { class: 'badge ok' }, sd('stateOk')))
        : h('div', { class: 'warnbox', style: 'margin:8px 0' }, h('strong', null, w.title), h('div', { style: 'margin-top:4px' }, w.text),
          st.problem && st.problem.detail ? h('div', { class: 'muted', style: 'margin-top:4px;font-size:12px' }, st.problem.detail) : null),
      h('div', { class: 'row', style: 'margin-top:8px' },
        h('button', { class: 'btn', disabled: !st.ok || st.loading, onclick: () => run(() => api('POST', '/api/db/backup'), sd('backupMade')) }, sd('backupNow')),
        h('button', { class: 'btn danger', disabled: st.loading, onclick: async () => {
          if (!confirm(st.ok ? sd('resetConfirm') : sd('resetConfirmBroken'))) return;
          run(() => api('POST', '/api/db/reset'), sd('resetDone'));
        } }, sd('reset'))),
      busy);
  }

  // the copies in the backups folder: restore one, remove one, or bring a file
  function dbBackupsCard(st) {
    const busy = h('div', { class: 'muted', style: 'margin-top:8px' });
    const run = async (fn, done) => {
      busy.textContent = sd('working');
      try { await fn(); toast(done); } catch (e) { toast(e.message); }
      render();
    };
    const file = h('input', { type: 'file', accept: '.db,application/octet-stream', style: 'display:none' });
    file.addEventListener('change', async () => {
      if (!file.files.length) return;
      if (!confirm(sd('importConfirm'))) { file.value = ''; return; }
      const fd = new FormData();
      fd.append('file', file.files[0]);
      run(() => api('POST', '/api/db/import', fd), sd('restored'));
    });
    const list = st.backups || [];
    const baseKind = (k) => String(k || '').replace(/[0-9]+$/, '');
    return h('div', { class: 'card' },
      h('h3', null, sd('backupsTitle')),
      list.length === 0 ? h('p', { class: 'muted' }, sd('backupsEmpty')) : h('table', null,
        h('tbody', null, list.map((b) => h('tr', null,
          h('td', null, fmtWhen(b.time), h('div', { class: 'muted', style: 'font-size:12px' }, (sd('kinds')[baseKind(b.kind)] || b.kind) + '  \u00b7  ' + fmtBytes(b.size))),
          h('td', { class: 'actions' },
            h('button', { class: 'btn sm', disabled: st.loading, onclick: () => {
              if (!confirm(sd('restoreConfirm'))) return;
              run(() => api('POST', '/api/db/restore', { name: b.name }), sd('restored'));
            } }, sd('restore')), ' ',
            h('button', { class: 'btn sm danger', onclick: () => {
              if (!confirm(sd('removeConfirm'))) return;
              run(() => api('DELETE', '/api/db/backups/' + encodeURIComponent(b.name)), t('done'));
            } }, sd('remove'))))))),
      h('p', { class: 'muted', style: 'margin-top:10px' }, sd('importHow')),
      h('button', { class: 'btn', disabled: st.loading, onclick: () => file.click() }, sd('importBackup')), file, busy);
  }

  // load from the simulator: the names from its files, each name asked through SimConnect, airports.
  // The numbers follow a running load in place: the page is not redrawn (that lost the scroll
  // position and whatever was being typed).
  async function dbLoadCard() {
    const sum = await api('GET', '/api/nav/summary');
    const c = sum.counts;
    const job = sum.job;
    const pct = job.total > 0 ? Math.min(100, Math.round(job.done * 100 / job.total)) : 0;
    const barFill = h('i', { style: `width:${pct}%` });
    const phaseText = h('div', { class: 'muted' }, navPhaseText(job, pct));
    const pendingBadge = h('span', { class: 'badge' }, navPendingText(c));
    const start = async (body) => {
      try { await api('POST', '/api/nav/load', body); } catch (e) { toast(e.message); }
      renderSetup();
    };
    const addIn = h('input', { type: 'text', placeholder: nv('loadAirport'), style: 'max-width:260px' });
    const loadOne = async () => {
      const icao = addIn.value.trim().toUpperCase();
      if (!icao) return;
      try { await api('GET', '/api/nav/airport/' + encodeURIComponent(icao) + '?refresh=1'); toast(nv('loaded') + ': ' + icao); } catch (e) { toast(nv('failed') + ': ' + e.message); }
      addIn.value = '';
    };
    const tick = async () => {
      if (parseRoute().name !== 'setup') return;
      let now;
      try { now = await api('GET', '/api/nav/summary'); } catch (e) { setupTimer = setTimeout(tick, 3000); return; }
      if (parseRoute().name !== 'setup') return;
      if (!now.job.running) { renderSetup(); return; }
      const p = now.job.total > 0 ? Math.min(100, Math.round(now.job.done * 100 / now.job.total)) : 0;
      barFill.style.width = p + '%';
      phaseText.textContent = navPhaseText(now.job, p);
      pendingBadge.textContent = navPendingText(now.counts);
      setupTimer = setTimeout(tick, 1500);
    };
    if (job.running) setupTimer = setTimeout(tick, 1500);
    return h('div', { class: 'card' },
      h('h3', null, sd('loadSection')),
      h('p', { class: 'muted' }, nv('loadHow')),
      h('p', null,
        sum.files.length ? h('span', { class: 'badge ok' }, `${nv('filesFound')}: ${sum.files.length}`) : h('span', { class: 'badge warn' }, nv('filesNone')),
        !sum.simLink ? h('span', { class: 'badge bad' }, nv('noLink')) : null,
        c.seedIdents ? pendingBadge : null),
      h('div', null, h('div', { class: 'bar' }, barFill), phaseText,
        job.message ? h('div', { class: 'badge bad', style: 'margin-top:6px' }, job.message) : null),
      h('div', { class: 'row', style: 'margin-top:10px' },
        h('label', { class: 'fld' }, h('span', { class: 'muted' }, nv('airportPrefix') + ' (' + nv('airportPrefixHint') + ')'),
          h('input', { type: 'text', value: navState.prefix, maxlength: 4, style: 'width:90px', oninput: (e) => { navState.prefix = e.target.value.toUpperCase(); } }))),
      h('div', { class: 'row', style: 'margin-top:10px' },
        job.running
          ? h('button', { class: 'btn danger', onclick: async () => { await api('DELETE', '/api/nav/load'); renderSetup(); } }, nv('stop'))
          : [
            h('button', { class: 'btn primary', disabled: !sum.simLink, onclick: () => start({ mode: 'all', prefix: navState.prefix }) }, nv('loadAll')),
            h('button', { class: 'btn', disabled: !sum.simLink, onclick: () => start({ mode: 'airports', prefix: navState.prefix }) }, nv('loadAirportsOnly')),
            h('button', { class: 'btn', disabled: !sum.files.length, onclick: () => start({ mode: 'files' }) }, nv('loadFiles'))]),
      h('h4', { style: 'margin:16px 0 4px' }, sd('loadAirportTitle')),
      h('div', { class: 'muted', style: 'margin-bottom:6px' }, sd('loadAirportHow')),
      h('div', { class: 'row' }, addIn, h('button', { class: 'btn', disabled: !sum.simLink, onclick: loadOne }, nv('loadAirportBtn'))));
  }

  // import the user points that Little Navmap exports
  function dbPointsImportCard() {
    const file = h('input', { type: 'file', accept: '.csv,text/csv,text/plain', style: 'display:none' });
    file.addEventListener('change', async () => {
      if (!file.files.length) return;
      const fd = new FormData();
      fd.append('file', file.files[0]);
      try {
        const res = await api('POST', '/api/nav/points/import', fd);
        toast(fill(nv('imported'), { a: res.added, d: res.duplicates, i: res.ignored }));
      } catch (err) { toast(nv('failed') + ': ' + err.message); }
      file.value = '';
    });
    return h('div', { class: 'card' },
      h('h3', null, nv('importTitle')),
      h('p', { class: 'muted' }, nv('importHow')),
      h('button', { class: 'btn', onclick: () => file.click() }, nv('importBtn')), file);
  }

  // every card of the database section of Setup
  async function databaseCards() {
    let st;
    try { st = await api('GET', '/api/db/status'); } catch (e) { return []; }
    const cards = [dbStateCard(st), dbBackupsCard(st)];
    if (st.ok) {
      try { cards.push(await dbLoadCard()); } catch (e) { /* the card is an extra */ }
      cards.push(dbPointsImportCard());
    }
    return cards;
  }

  // ---- Navigation: the data the route uses (airports, fixes, airways), the user's own points, and the
  // bulk load. Everything here comes from the simulator; nothing is shipped with Open Charts.
  const NV = {
    en: {
      title: 'Navigation data',
      intro: 'Airports, waypoints, VORs, NDBs and airways that Open Charts uses to read a route, as they are stored. They are loaded from your own simulator (SimConnect), as routes ask for them. To load everything at once, import points, or look after the database (copies, starting over), use the Setup page.',
      stats: 'Stored now', airports: 'Airports', runways: 'Runways', procedures: 'Procedures', waypoints: 'Waypoints', vors: 'VORs', ndbs: 'NDBs',
      airways: 'Airways', segments: 'Segments', idents: 'Names asked', myPoints: 'My points',
      loadTitle: 'Load everything',
      loadHow: 'Two steps, always in this order: (1) the names of every waypoint, VOR and NDB are read from the navigation files the simulator keeps on this computer; (2) each name is asked to the simulator through SimConnect, which brings its region, position and airways. The simulator must be running with a flight loaded. You can stop and continue later.',
      filesFound: 'Simulator files found', filesNone: 'No simulator navigation files found: only step 2 applies, for the names a route cites.',
      noLink: 'SimConnect.dll was not found: the simulator cannot be asked.',
      pending: 'names still to ask', loadAll: 'Load everything', loadFiles: 'Only read the names from the files', stop: 'Stop',
      loadAirportsOnly: 'Only airports', airportPrefix: 'Airports: only codes starting with', airportPrefixHint: 'e.g. SB (Brazil); empty = every airport of the simulator, which takes a long time',
      phase: { listing: 'Asking the simulator for its list of airports', airports: 'Loading airports (with their procedures)', interrupted: 'Interrupted: the program was restarted while loading. Press Load to continue.', idle: 'Idle', reading: 'Reading the simulator files', asking: 'Asking the simulator', done: 'Finished', stopped: 'Stopped', error: 'Stopped by an error' },
      tabAirports: 'Airports', tabFixes: 'Fixes', tabAirways: 'Airways', tabPoints: 'My points',
      search: 'Search by name...', all: 'All', kindW: 'Waypoint', kindV: 'VOR', kindN: 'NDB',
      ident: 'Name', region: 'Region', kind: 'Type', pos: 'Position', name: 'Description', lat: 'Latitude', lon: 'Longitude', note: 'Note',
      fetched: 'Stored on', refresh: 'Ask again', del: 'Delete', edit: 'Edit', save: 'Save', cancel: 'Cancel', add: 'Add', next: 'Next', prev: 'Previous',
      loadAirport: 'ICAO of an airport to load', loadAirportBtn: 'Load from the simulator',
      noRows: 'Nothing here yet.', confirmDel: 'Delete this? It can be loaded again from the simulator.',
      pointsIntro: 'Places you make yourself (a private waypoint, a visual reference). They can be written in a pasted route like any waypoint.',
      newPoint: 'New point', editPoint: 'Edit point', asked: 'Names asked', airwaySegs: 'segments', hideSegs: 'Hide', showSegs: 'Show',
      total: 'total', loaded: 'Loaded', failed: 'Could not do that',
      importTitle: 'Import from Little Navmap',
      importHow: 'Choose the CSV that Little Navmap exports from "My Data" (user points). Points without an identifier get one made from the name. Importing the same file twice adds nothing.',
      importBtn: 'Choose a CSV file...', imported: '{a} added, {d} already there, {i} rows ignored'
    },
    pt: {
      title: 'Dados de navegação',
      intro: 'Aeroportos, waypoints, VORs, NDBs e aerovias que o Open Charts usa para ler uma rota, como estão guardados. Eles vêm do seu próprio simulador (SimConnect), conforme as rotas pedem. Para carregar tudo de uma vez, importar pontos ou cuidar do banco (cópias, recomeçar), use a página Setup.',
      stats: 'Guardado agora', airports: 'Aeroportos', runways: 'Pistas', procedures: 'Procedimentos', waypoints: 'Waypoints', vors: 'VORs', ndbs: 'NDBs',
      airways: 'Aerovias', segments: 'Trechos', idents: 'Nomes consultados', myPoints: 'Meus pontos',
      loadTitle: 'Carregar tudo',
      loadHow: 'Dois passos, sempre nesta ordem: (1) os nomes de todos os waypoints, VORs e NDBs são lidos dos arquivos de navegação que o simulador guarda neste computador; (2) cada nome é perguntado ao simulador pelo SimConnect, que traz a região, a posição e as aerovias. O simulador precisa estar aberto com um voo carregado. Dá para parar e continuar depois.',
      filesFound: 'Arquivos do simulador encontrados', filesNone: 'Nenhum arquivo de navegação do simulador encontrado: vale só o passo 2, para os nomes que uma rota cita.',
      noLink: 'O SimConnect.dll não foi encontrado: não dá para perguntar ao simulador.',
      pending: 'nomes ainda para perguntar', loadAll: 'Carregar tudo', loadFiles: 'Só ler os nomes dos arquivos', stop: 'Parar',
      loadAirportsOnly: 'Só aeroportos', airportPrefix: 'Aeroportos: só os códigos que começam com', airportPrefixHint: 'ex.: SB (Brasil); vazio = todos os aeroportos do simulador, o que demora bastante',
      phase: { listing: 'Pedindo ao simulador a lista de aeroportos', airports: 'Carregando aeroportos (com os procedimentos)', interrupted: 'Interrompido: o programa foi reiniciado durante a carga. Aperte Carregar tudo para continuar.', idle: 'Parado', reading: 'Lendo os arquivos do simulador', asking: 'Perguntando ao simulador', done: 'Concluído', stopped: 'Interrompido', error: 'Interrompido por um erro' },
      tabAirports: 'Aeroportos', tabFixes: 'Fixos', tabAirways: 'Aerovias', tabPoints: 'Meus pontos',
      search: 'Buscar pelo nome...', all: 'Todos', kindW: 'Waypoint', kindV: 'VOR', kindN: 'NDB',
      ident: 'Nome', region: 'Região', kind: 'Tipo', pos: 'Posição', name: 'Descrição', lat: 'Latitude', lon: 'Longitude', note: 'Nota',
      fetched: 'Guardado em', refresh: 'Perguntar de novo', del: 'Excluir', edit: 'Editar', save: 'Salvar', cancel: 'Cancelar', add: 'Adicionar', next: 'Próxima', prev: 'Anterior',
      loadAirport: 'ICAO de um aeroporto para carregar', loadAirportBtn: 'Carregar do simulador',
      noRows: 'Nada por aqui ainda.', confirmDel: 'Excluir? Dá para carregar de novo do simulador.',
      pointsIntro: 'Lugares que você mesmo cria (um waypoint particular, uma referência visual). Podem ser escritos numa rota colada como qualquer waypoint.',
      newPoint: 'Novo ponto', editPoint: 'Editar ponto', asked: 'Nomes consultados', airwaySegs: 'trechos', hideSegs: 'Ocultar', showSegs: 'Ver',
      total: 'no total', loaded: 'Carregado', failed: 'Não foi possível',
      importTitle: 'Importar do Little Navmap',
      importHow: 'Escolha o CSV que o Little Navmap exporta de "Meus dados" (pontos do usuário). Pontos sem identificador ganham um feito a partir do nome. Importar o mesmo arquivo duas vezes não duplica nada.',
      importBtn: 'Escolher um arquivo CSV...', imported: '{a} adicionados, {d} já existiam, {i} linhas ignoradas'
    }
  };
  const nv = (k) => (NV[lang] && NV[lang][k]) || NV.en[k] || k;
  const navState = { prefix: '', tab: 'airports', fixQ: '', fixKind: '', fixOffset: 0, airportOffset: 0, airportQ: '', airwayQ: '', edit: null, open: '' };
  let navTimer = null;

  const fmtNum = (n) => Number(n || 0).toLocaleString(lang === 'pt' ? 'pt-BR' : 'en-US');
  const fmtPos = (lat, lon) => `${Number(lat).toFixed(4)}, ${Number(lon).toFixed(4)}`;

  // The Navigation page is drawn once (with the counts, which take a moment to count); changing the
  // tab, paging and searching only redraw the body, without asking for the counts again.
  let navUI = null;
  const NAV_TABS = [['airports', 'tabAirports'], ['fixes', 'tabFixes'], ['airways', 'tabAirways'], ['points', 'tabPoints']];
  function navTabButtons() {
    return NAV_TABS.map(([id, key]) =>
      h('button', { class: 'btn' + (navState.tab === id ? ' primary' : ''), onclick: () => { navState.tab = id; navState.edit = null; navRefresh(); } }, nv(key)));
  }
  async function navTabBody(body) {
    if (navState.tab === 'airports') await navAirports(body);
    else if (navState.tab === 'fixes') await navFixes(body);
    else if (navState.tab === 'airways') await navAirways(body);
    else await navPoints(body);
  }
  // changed: something was deleted or saved, so the counts are asked again, in the background
  async function navRefresh(changed) {
    if (!navUI || !navUI.body.isConnected) { await renderNav(); return; }
    navUI.tabs.replaceChildren(...navTabButtons());
    await navTabBody(navUI.body);
    if (changed) navStats();
  }
  async function navStats() {
    try {
      const sum = await api('GET', '/api/nav/summary');
      if (!navUI) return;
      for (const k of Object.keys(sum.counts)) if (navUI.live[k]) navUI.live[k].textContent = fmtNum(sum.counts[k]);
    } catch (e) { /* the numbers stay as they were */ }
  }

  async function renderNav() {
    clearTimeout(navTimer);
    const sum = await api('GET', '/api/nav/summary');
    const c = sum.counts;
    const live = {};
    const stat = (label, n, key) => {
      const num = h('div', { class: 'statn' }, fmtNum(n));
      if (key) live[key] = num;
      return h('div', { class: 'stat' }, num, h('div', { class: 'muted' }, label));
    };

    const tabs = h('div', { class: 'row', style: 'margin-bottom:12px' }, navTabButtons());

    const body = h('div', { class: 'card' });
    view.replaceChildren(h('h2', null, nv('title')), h('p', { class: 'muted' }, nv('intro')),
      h('div', { class: 'card' }, h('h3', null, nv('stats')),
        h('div', { class: 'stats' }, stat(nv('airports'), c.airports, 'airports'), stat(nv('runways'), c.runways, 'runways'),
          stat(nv('procedures'), c.procedures, 'procedures'), stat(nv('waypoints'), c.waypoints, 'waypoints'), stat(nv('vors'), c.vors, 'vors'),
          stat(nv('ndbs'), c.ndbs, 'ndbs'), stat(nv('airways'), c.airways, 'airways'), stat(nv('segments'), c.segments, 'segments'),
          stat(nv('idents'), c.identsChecked, 'identsChecked'), stat(nv('myPoints'), c.userPoints, 'userPoints'))),
      tabs, body);

    navUI = { body, tabs, live };
    await navTabBody(body);
  }

  function navPhaseText(job, pct) {
    return `${nv('phase')[job.phase] || job.phase}${job.total ? ` - ${fmtNum(job.done)} / ${fmtNum(job.total)} (${pct}%)` : ''}`;
  }
  const navPendingText = (c) => `${fmtNum(c.seedPending)} ${nv('pending')} (${fmtNum(c.seedIdents)} ${nv('total')})`;

  function navSearch(value, placeholder, onGo) {
    const input = h('input', { type: 'text', value, placeholder, class: 'grow' });
    input.addEventListener('keydown', (e) => { if (e.key === 'Enter') onGo(input.value); });
    return [input, h('button', { class: 'btn', onclick: () => onGo(input.value) }, '⌕')];
  }

  async function navAirports(body) {
    const limit = 100;
    const r = await api('GET', `/api/nav/airports?q=${encodeURIComponent(navState.airportQ)}&limit=${limit}&offset=${navState.airportOffset}`);
    const last = navState.airportOffset + r.airports.length;
    body.replaceChildren(
      h('div', { class: 'row' }, ...navSearch(navState.airportQ, nv('search'), (v) => { navState.airportQ = v.trim(); navState.airportOffset = 0; navRefresh(); })),
      h('div', { style: 'height:10px' }),
      r.airports.length === 0 ? h('p', { class: 'muted' }, nv('noRows')) :
        h('table', null,
          h('thead', null, h('tr', null, h('th', null, 'ICAO'), h('th', null, nv('name')), h('th', null, nv('fetched')), h('th'))),
          h('tbody', null, r.airports.map((a) => h('tr', null,
            h('td', null, h('strong', null, a.icao)), h('td', null, a.name), h('td', { class: 'muted' }, (a.fetchedAt || '').slice(0, 10)),
            h('td', { class: 'actions' },
              h('button', { class: 'btn sm danger', onclick: async () => {
                if (!confirm(nv('confirmDel'))) return;
                await api('DELETE', '/api/nav/airport/' + encodeURIComponent(a.icao)); navRefresh(true);
              } }, nv('del'))))))),
      h('div', { class: 'row', style: 'margin-top:10px' },
        h('button', { class: 'btn sm', disabled: navState.airportOffset === 0, onclick: () => { navState.airportOffset = Math.max(0, navState.airportOffset - limit); navRefresh(); } }, nv('prev')),
        h('span', { class: 'muted' }, r.total ? `${fmtNum(navState.airportOffset + 1)}-${fmtNum(last)} / ${fmtNum(r.total)}` : ''),
        h('button', { class: 'btn sm', disabled: last >= r.total, onclick: () => { navState.airportOffset += limit; navRefresh(); } }, nv('next'))));
  }

  async function navFixes(body) {
    const limit = 100;
    const r = await api('GET', `/api/nav/fixes?q=${encodeURIComponent(navState.fixQ)}&kind=${navState.fixKind}&limit=${limit}&offset=${navState.fixOffset}`);
    const kindSel = h('select', { onchange: (e) => { navState.fixKind = e.target.value; navState.fixOffset = 0; navRefresh(); } },
      h('option', { value: '' }, nv('all')), h('option', { value: 'W' }, nv('kindW')), h('option', { value: 'V' }, nv('kindV')), h('option', { value: 'N' }, nv('kindN')));
    kindSel.value = navState.fixKind;
    const kindName = { W: nv('kindW'), V: nv('kindV'), N: nv('kindN') };
    const last = navState.fixOffset + r.fixes.length;
    body.replaceChildren(
      h('div', { class: 'row' }, ...navSearch(navState.fixQ, nv('search'), (v) => { navState.fixQ = v.trim(); navState.fixOffset = 0; navRefresh(); }), kindSel),
      r.fixes.length === 0 ? h('p', { class: 'muted' }, nv('noRows')) : h('table', { style: 'margin-top:10px' },
        h('thead', null, h('tr', null, h('th', null, nv('ident')), h('th', null, nv('kind')), h('th', null, nv('region')), h('th', null, nv('name')),
          h('th', null, nv('pos')), h('th', null, nv('airways')), h('th'))),
        h('tbody', null, r.fixes.map((f) => h('tr', null,
          h('td', null, h('strong', null, f.ident)), h('td', null, kindName[f.kind] || f.kind), h('td', null, f.region || '-'),
          h('td', null, f.name), h('td', { class: 'muted' }, fmtPos(f.lat, f.lon)), h('td', null, f.routes || ''),
          h('td', { class: 'actions' }, h('button', { class: 'btn sm danger', onclick: async () => {
            if (!confirm(nv('confirmDel'))) return;
            await api('DELETE', `/api/nav/fixes?ident=${encodeURIComponent(f.ident)}&region=${encodeURIComponent(f.region)}&kind=${f.kind}`);
            navRefresh(true);
          } }, nv('del'))))))),
      h('div', { class: 'row', style: 'margin-top:10px' },
        h('button', { class: 'btn sm', disabled: navState.fixOffset === 0, onclick: () => { navState.fixOffset = Math.max(0, navState.fixOffset - limit); navRefresh(); } }, nv('prev')),
        h('span', { class: 'muted' }, r.total ? `${fmtNum(navState.fixOffset + 1)}-${fmtNum(last)} / ${fmtNum(r.total)}` : ''),
        h('button', { class: 'btn sm', disabled: last >= r.total, onclick: () => { navState.fixOffset += limit; navRefresh(); } }, nv('next'))));
  }

  async function navAirways(body) {
    const r = await api('GET', '/api/nav/airways?q=' + encodeURIComponent(navState.airwayQ));
    let segs = null;
    if (navState.open) segs = await api('GET', '/api/nav/airway/' + encodeURIComponent(navState.open));
    body.replaceChildren(
      h('div', { class: 'row' }, ...navSearch(navState.airwayQ, nv('search'), (v) => { navState.airwayQ = v.trim(); navState.open = ''; navRefresh(); })),
      r.airways.length === 0 ? h('p', { class: 'muted' }, nv('noRows')) : h('table', { style: 'margin-top:10px' },
        h('tbody', null, r.airways.map((a) => [
          h('tr', { class: 'clickable', onclick: () => { navState.open = navState.open === a.name ? '' : a.name; navRefresh(); } },
            h('td', null, h('strong', null, a.name)), h('td', { class: 'muted' }, `${a.segments} ${nv('airwaySegs')}`),
            h('td', { class: 'actions' }, navState.open === a.name ? nv('hideSegs') : nv('showSegs'))),
          navState.open === a.name && segs ? h('tr', null, h('td', { colspan: 3, class: 'muted' },
            segs.segments.map((s) => `${s.from} > ${s.to}`).join('   '))) : null]))));
  }

  async function navPoints(body) {
    const r = await api('GET', '/api/nav/points');
    const e = navState.edit || { id: 0, ident: '', name: '', lat: '', lon: '', note: '' };
    const field = (key, label, extra) => h('label', { class: 'fld' }, h('span', { class: 'muted' }, label),
      h('input', Object.assign({ type: 'text', value: e[key] === undefined ? '' : String(e[key]), 'data-k': key }, extra || {})));
    const form = h('div', { class: 'card', style: 'margin-bottom:12px' },
      h('h3', null, e.id ? nv('editPoint') : nv('newPoint')),
      h('div', { class: 'row' }, field('ident', nv('ident'), { maxlength: 12, style: 'width:120px' }), field('name', nv('name'), { style: 'width:200px' }),
        field('lat', nv('lat'), { style: 'width:120px' }), field('lon', nv('lon'), { style: 'width:120px' }), field('note', nv('note'), { style: 'width:200px' })),
      h('div', { class: 'row', style: 'margin-top:10px' },
        h('button', { class: 'btn primary', onclick: async () => {
          const v = {};
          form.querySelectorAll('input[data-k]').forEach((i) => { v[i.getAttribute('data-k')] = i.value.trim(); });
          const payload = { ident: v.ident, name: v.name, note: v.note, type: e.type || '', tags: e.tags || '', altFt: e.altFt || 0, lat: Number(v.lat.replace(',', '.')), lon: Number(v.lon.replace(',', '.')) };
          if (v.lat === '' || v.lon === '' || !isFinite(payload.lat) || !isFinite(payload.lon)) { toast(nv('failed') + ': ' + nv('lat') + ' / ' + nv('lon')); return; }
          try {
            if (e.id) await api('PUT', '/api/nav/points/' + e.id, payload); else await api('POST', '/api/nav/points', payload);
            navState.edit = null;
            navRefresh(true);
          } catch (err) { toast(nv('failed') + ': ' + err.message); }
        } }, e.id ? nv('save') : nv('add')),
        e.id ? h('button', { class: 'btn', onclick: () => { navState.edit = null; navRefresh(); } }, nv('cancel')) : null));
    body.replaceChildren(h('p', { class: 'muted' }, nv('pointsIntro')), form,
      r.points.length === 0 ? h('p', { class: 'muted' }, nv('noRows')) : h('table', null,
        h('thead', null, h('tr', null, h('th', null, nv('ident')), h('th', null, nv('name')), h('th', null, nv('kind')), h('th', null, nv('pos')), h('th', null, nv('note')), h('th'))),
        h('tbody', null, r.points.map((p) => h('tr', null,
          h('td', null, h('strong', null, p.ident)), h('td', null, p.name), h('td', { class: 'muted' }, p.type), h('td', { class: 'muted' }, fmtPos(p.lat, p.lon)), h('td', { class: 'muted' }, p.note),
          h('td', { class: 'actions' },
            h('button', { class: 'btn sm', onclick: () => { navState.edit = p; navRefresh(); } }, nv('edit')), ' ',
            h('button', { class: 'btn sm danger', onclick: async () => {
              if (!confirm(nv('confirmDel'))) return;
              await api('DELETE', '/api/nav/points/' + p.id); navRefresh(true);
            } }, nv('del'))))))));
  }

  // ---- Routes: written here, validated here, and one tap in the viewer. The map below is only a
  // high overview to check the route by eye; it is not the map of the simulator's tablet.
  const RT = {
    en: {
      title: 'Routes', intro: 'Write routes the way pilots do (SBSJ BOBI3A DAGUB UZ6 SBGR), check them and keep them. In the simulator\'s viewer, the route button lists them: one tap and the route is loaded.',
      mine: 'Saved routes', newRoute: 'New route', none: 'No saved route yet.', name: 'Name', text: 'Route',
      textHint: 'SBSJ BOBI3A DAGUB UZ6 ... SBGR', validate: 'Validate', save: 'Save', del: 'Delete', confirmDel: 'Delete this route?',
      working: 'Reading the route (the simulator may be asked for names it does not know yet)...', places: 'places', problems: 'to check',
      ok: 'All names were found.', saved: 'Saved', failed: 'Could not do that', needName: 'Give the route a name.',
      sendTo: 'Send the plan', ivao: 'Open in IVAO', savePln: 'Save the plan (.pln)',
      sendHint: 'Only the origin, the destination and the route are sent. IVAO opens its form with them filled in: check it and file it there. The .pln is the simulator\'s flight plan file.',
      needText: 'Write the route first.',
      nameCol: 'Name', kind: 'Type', pos: 'Position', via: 'Via', empty: 'Write a route and press Validate.',
      kinds: { airport: 'Airport', waypoint: 'Waypoint', vor: 'VOR', ndb: 'NDB', coordinate: 'Coordinate', procedure: 'Procedure' },
      mapFail: 'The map could not be loaded.',
      conflict: 'More than one place has this name. The closest to the previous point is used; tap to choose another.',
      chosen: 'You chose this one. Tap to change.', usingClosest: 'In use (closest)', usingChosen: 'In use (your choice)', useThis: 'Use this one',
      backToClosest: 'Go back to the closest', km: 'km', conflicts: 'with doubt'
    },
    pt: {
      title: 'Rotas', intro: 'Escreva rotas do jeito que o piloto escreve (SBSJ BOBI3A DAGUB UZ6 SBGR), confira e guarde. No viewer do simulador, o botão da rota lista todas elas: um toque e a rota é carregada.',
      mine: 'Rotas salvas', newRoute: 'Nova rota', none: 'Nenhuma rota salva ainda.', name: 'Nome', text: 'Rota',
      textHint: 'SBSJ BOBI3A DAGUB UZ6 ... SBGR', validate: 'Validar', save: 'Salvar', del: 'Excluir', confirmDel: 'Excluir esta rota?',
      working: 'Lendo a rota (o simulador pode ser consultado por nomes que ainda não conhece)...', places: 'pontos', problems: 'a conferir',
      ok: 'Todos os nomes foram encontrados.', saved: 'Salva', failed: 'Não foi possível', needName: 'Dê um nome para a rota.',
      sendTo: 'Enviar o plano', ivao: 'Abrir na IVAO', savePln: 'Salvar o plano (.pln)',
      sendHint: 'Só a origem, o destino e a rota são enviados. A IVAO abre o formulário com eles preenchidos: confira e registre lá. O .pln é o arquivo de plano de voo do simulador.',
      needText: 'Escreva a rota primeiro.',
      nameCol: 'Nome', kind: 'Tipo', pos: 'Posição', via: 'Via', empty: 'Escreva uma rota e aperte Validar.',
      kinds: { airport: 'Aeroporto', waypoint: 'Waypoint', vor: 'VOR', ndb: 'NDB', coordinate: 'Coordenada', procedure: 'Procedimento' },
      mapFail: 'Não foi possível carregar o mapa.',
      conflict: 'Mais de um lugar tem este nome. Está sendo usado o mais próximo do ponto anterior; toque para escolher outro.',
      chosen: 'Você escolheu este. Toque para mudar.', usingClosest: 'Em uso (o mais próximo)', usingChosen: 'Em uso (sua escolha)', useThis: 'Usar este',
      backToClosest: 'Voltar para o mais próximo', km: 'km', conflicts: 'com dúvida'
    }
  };
  const rt = (k) => (RT[lang] && RT[lang][k]) || RT.en[k] || k;
  const routesState = { id: 0, name: '', text: '', choices: [], result: null, working: false };
  let overviewMap = null;

  function loadLeaflet() {
    if (window.L) return Promise.resolve();
    return new Promise((resolve, reject) => {
      const css = document.createElement('link');
      css.rel = 'stylesheet';
      css.href = '/vendor/leaflet/leaflet.css';
      document.head.appendChild(css);
      const js = document.createElement('script');
      js.src = '/vendor/leaflet/leaflet.js';
      js.onload = () => resolve();
      js.onerror = () => reject(new Error('leaflet'));
      document.head.appendChild(js);
    });
  }

  async function drawOverview(host, res) {
    if (overviewMap) { overviewMap.remove(); overviewMap = null; }
    host.replaceChildren();
    if (!res || !res.points || !res.points.length) return;
    try { await loadLeaflet(); } catch (e) { host.textContent = rt('mapFail'); return; }
    const m = window.L.map(host, { zoomControl: true, attributionControl: true, scrollWheelZoom: false });
    window.L.tileLayer('/tiles/{z}/{x}/{y}.png', { maxZoom: 12, attribution: '&copy; OpenStreetMap contributors' }).addTo(m);
    const pts = res.points.map((p) => [p.lat, p.lon]);
    window.L.polyline(pts, { color: '#c2185b', weight: 4, opacity: 0.9 }).addTo(m);
    let last = '';
    res.points.forEach((p) => {
      const big = p.kind === 'airport';
      const dot = window.L.circleMarker([p.lat, p.lon], { radius: big ? 7 : 4, color: '#c2185b', weight: 2, fillColor: big ? '#c2185b' : '#fff', fillOpacity: 1 }).addTo(m);
      if (p.ident !== last) dot.bindTooltip(p.ident, { permanent: big, direction: 'right', offset: [6, 0] });
      last = p.ident;
    });
    m.fitBounds(window.L.latLngBounds(pts), { padding: [30, 30] });
    overviewMap = m;
    setTimeout(() => m.invalidateSize(), 50);
  }

  async function renderRoutes() {
    if (overviewMap) { overviewMap.remove(); overviewMap = null; }
    const data = await api('GET', '/api/routes');
    const list = data.routes;
    const sel = list.find((r) => r.id === routesState.id);
    if (routesState.id && !sel) { routesState.id = 0; routesState.name = ''; routesState.text = ''; routesState.result = null; }

    const nameIn = h('input', { type: 'text', value: routesState.name, placeholder: rt('name') });
    const textIn = h('textarea', { rows: 4, placeholder: rt('textHint'), class: 'routetext' });
    textIn.value = routesState.text;
    // The name follows the route while the user has not chosen one: first place > last place.
    let autoName = routesState.id === 0 && (routesState.name === '' || routesState.name === routesState.lastAuto);
    let suggestTimer = null;
    const suggest = () => {
      clearTimeout(suggestTimer);
      suggestTimer = setTimeout(async () => {
        if (!autoName) return;
        try {
          const r = await api('GET', '/api/routes/name?text=' + encodeURIComponent(textIn.value));
          if (!autoName) return;
          nameIn.value = r.name;
          routesState.lastAuto = r.name;
        } catch (e) { /* the name stays as it was */ }
      }, 250);
    };
    textIn.addEventListener('input', suggest);
    nameIn.addEventListener('input', () => { autoName = nameIn.value === '' || nameIn.value === routesState.lastAuto; });
    const status = h('div', { class: 'muted', style: 'margin:8px 0' });
    const resultHost = h('div');
    const mapHost = h('div', { class: 'overview' });

    const showResult = (res) => {
      routesState.result = res;
      const head = res
        ? h('div', { style: 'margin:8px 0' },
          h('span', { class: 'badge' }, `${res.points.length} ${rt('places')}`),
          res.problems.length ? h('span', { class: 'badge warn' }, `${res.problems.length} ${rt('problems')}`) : h('span', { class: 'badge ok' }, rt('ok')),
          res.totalNm ? h('span', { class: 'badge' }, `${fmtNum(res.totalNm)} NM`) : null,
          res.points.some((p) => p.candidates && p.candidates.length > 1) ? h('span', { class: 'badge warn' }, '! ' + rt('conflicts')) : null)
        : null;
      const problems = res && res.problems.length
        ? h('div', { class: 'warnbox', style: 'margin-bottom:8px' }, res.problems.map((q) => h('div', null,
          h('strong', null, q.token || ''), ' ', q.message,
          q.candidates && q.candidates.length ? '  (' + q.candidates.map((c) => c.ident + '/' + c.region).join(', ') + ')' : '')))
        : null;
      const choose = (p, c) => {
        routesState.choices = routesState.choices.filter((x) => !(x.token === p.token && x.nth === p.nth));
        if (c) routesState.choices.push({ token: p.token, nth: p.nth, region: c.region, kind: c.kind });
        validate();
      };
      const kinds = RT[lang].kinds || {};
      const rows = [];
      (res ? res.points : []).forEach((p) => {
        const doubt = p.candidates && p.candidates.length > 1;
        const detail = h('tr', { hidden: true }, h('td', { colspan: 4 }, doubt ? h('div', { class: 'doubt' },
          p.candidates.map((c) => {
            const inUse = c.region === p.region && c.kind === p.kind;
            return h('div', { class: 'doubtrow' + (inUse ? ' on' : '') },
              h('span', null, h('strong', null, c.ident), ' ', c.region || '-', ' \u00b7 ', kinds[c.kind] || c.kind,
                c.name ? ' \u00b7 ' + c.name : '', ' \u00b7 ', fmtPos(c.lat, c.lon), c.distKm ? ' \u00b7 ' + c.distKm + ' ' + rt('km') : ''),
              inUse ? h('span', { class: 'badge' }, p.chosen ? rt('usingChosen') : rt('usingClosest'))
                : h('button', { class: 'btn sm', onclick: () => choose(p, c) }, rt('useThis')));
          }),
          p.chosen ? h('div', { style: 'margin-top:6px' }, h('button', { class: 'btn sm', onclick: () => choose(p, null) }, rt('backToClosest'))) : null) : ''));
        const bang = doubt ? h('button', {
          class: 'bang' + (p.chosen ? ' done' : ''), title: p.chosen ? rt('chosen') : rt('conflict'),
          onclick: () => { detail.hidden = !detail.hidden; }
        }, '!') : null;
        rows.push(h('tr', null,
          h('td', null, h('strong', null, p.ident), ' ', bang), h('td', { class: 'muted' }, kinds[p.kind] || p.kind),
          h('td', { class: 'muted' }, p.via || ''), h('td', { class: 'muted' }, fmtPos(p.lat, p.lon))));
        rows.push(detail);
      });
      const table = res && res.points.length
        ? h('table', null,
          h('thead', null, h('tr', null, h('th', null, rt('nameCol')), h('th', null, rt('kind')), h('th', null, rt('via')), h('th', null, rt('pos')))),
          h('tbody', null, rows))
        : h('p', { class: 'muted' }, rt('empty'));
      resultHost.replaceChildren(...[head, problems, table].filter(Boolean));
      drawOverview(mapHost, res);
    };

    const validate = async () => {
      const text = textIn.value.trim();
      routesState.text = text;
      routesState.name = nameIn.value;
      if (!text) { showResult(null); return; }
      status.textContent = rt('working');
      try { showResult(await api('POST', '/api/route/resolve', { route: text, choices: routesState.choices })); status.textContent = ''; }
      catch (e) { status.textContent = rt('failed') + ': ' + e.message; }
    };

    const save = async () => {
      routesState.text = textIn.value;
      routesState.name = nameIn.value;
      try {
        const body = { name: nameIn.value, text: textIn.value, choices: routesState.choices };
        const r = routesState.id ? await api('PUT', '/api/routes/' + routesState.id, body) : await api('POST', '/api/routes', body);
        routesState.id = r.id;
        routesState.name = r.name;
        routesState.text = r.text;
        routesState.result = r.result || null;
        toast(rt('saved'));
        renderRoutes();
      } catch (e) { toast(rt('failed') + ': ' + e.message); }
    };

    // Sending the plan: the program builds the files and the link (the route is read again there)
    const sendBody = () => (routesState.id && textIn.value === routesState.text
      ? { id: routesState.id } : { route: textIn.value, choices: routesState.choices });
    const sendIvao = async () => {
      if (!textIn.value.trim()) { toast(rt('needText')); return; }
      try {
        status.textContent = '...';
        const r = await api('POST', '/api/route/ivao', sendBody());
        window.open(r.url, '_blank', 'noopener');
      } catch (e) { toast(rt('failed') + ': ' + e.message); }
      status.textContent = '';
    };
    const savePln = async () => {
      if (!textIn.value.trim()) { toast(rt('needText')); return; }
      try {
        const res = await fetch('/api/route/pln', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(sendBody()) });
        if (!res.ok) { throw new Error(((await res.json().catch(() => ({}))).error) || res.statusText); }
        const m = /filename="([^"]+)"/.exec(res.headers.get('Content-Disposition') || '');
        const a = h('a', { href: URL.createObjectURL(await res.blob()), download: m ? m[1] : 'route.pln' });
        document.body.appendChild(a);
        a.click();
        a.remove();
        setTimeout(() => URL.revokeObjectURL(a.href), 2000);
      } catch (e) { toast(rt('failed') + ': ' + e.message); }
    };
    const listCard = h('div', { class: 'card' },
      h('div', { class: 'row', style: 'margin-bottom:8px' },
        h('h3', { style: 'margin:0', class: 'grow' }, rt('mine')),
        h('button', { class: 'btn sm primary', onclick: () => { Object.assign(routesState, { id: 0, name: '', text: '', choices: [], result: null, lastAuto: '' }); renderRoutes(); } }, '+ ' + rt('newRoute'))),
      list.length === 0 ? h('p', { class: 'muted' }, rt('none')) : list.map((r) => h('div', {
        class: 'routeitem' + (r.id === routesState.id ? ' on' : ''),
        onclick: () => { Object.assign(routesState, { id: r.id, name: r.name, text: r.text, choices: r.choices || [], result: r.result || null }); renderRoutes(); }
      }, h('strong', null, r.name), h('div', { class: 'muted' }, r.summary || r.text))));

    const editor = h('div', { class: 'card' },
      h('label', { class: 'fld' }, h('span', { class: 'muted' }, rt('name')), nameIn),
      h('label', { class: 'fld', style: 'margin-top:10px' }, h('span', { class: 'muted' }, rt('text')), textIn),
      h('div', { class: 'row', style: 'margin-top:10px' },
        h('button', { class: 'btn', onclick: validate }, rt('validate')),
        h('button', { class: 'btn primary', onclick: save }, rt('save')),
        routesState.id ? h('button', { class: 'btn danger', onclick: async () => {
          if (!confirm(rt('confirmDel'))) return;
          await api('DELETE', '/api/routes/' + routesState.id);
          Object.assign(routesState, { id: 0, name: '', text: '', choices: [], result: null });
          renderRoutes();
        } }, rt('del')) : null),
      h('div', { class: 'row sendrow', style: 'margin-top:10px' },
        h('span', { class: 'muted' }, rt('sendTo') + ':'),
        h('button', { class: 'btn', onclick: sendIvao }, rt('ivao')),
        h('button', { class: 'btn', onclick: savePln }, rt('savePln'))),
      h('p', { class: 'muted', style: 'margin:6px 0 0; font-size:12px' }, rt('sendHint')),
      status, mapHost, resultHost);

    view.replaceChildren(h('h2', null, rt('title')), h('p', { class: 'muted' }, rt('intro')),
      h('div', { class: 'routespage' }, listCard, editor));

    if (routesState.result) showResult(routesState.result);
    else if (routesState.id && routesState.text) validate();
    else showResult(null);
  }

  // ---- Map: the planner. A technical map to build a route by touching airports and fixes, with the
  // airways around. The EFB screen is for flying; planning is done here, before the flight.
  const MP = {
    en: {
      title: 'Map', hint: 'Click an airport or a fix to add it to the route. Click the origin, then the destination: what you add after that goes between them. Right-click an empty place to make a point of your own there.',
      route: 'Route', empty: 'The route is empty. Click airports and fixes on the map.', name: 'Name', save: 'Save', clear: 'Clear',
      validate: 'Validate', saved: 'Saved', failed: 'Could not do that', open: 'Open a saved route...', up: 'Move up', down: 'Move down',
      remove: 'Remove', zoomIn: 'Zoom in to see fixes and airways.', cut: 'Too many here: zoom in to see them all.',
      airports: 'Airports', fixesLayer: 'Fixes', airwaysLayer: 'Airways', pointsLayer: 'My points', ok: 'All names were found.',
      problems: 'to check', newPoint: 'Identifier of the new point (letters and digits):', pointFailed: 'Could not make the point',
      noAirports: 'No airports are stored in this area yet. Load them in the Navigation tab.', places: 'places',
      search: 'Search a place: identifier or name', noResults: 'Nothing found.', goTo: 'Show on the map', addTo: 'Add to the route',
      distance: 'Distance', nm: 'NM', typeAirport: 'Airport', typePoint: 'My point',
      kindsShort: { W: 'Waypoint', V: 'VOR', N: 'NDB' },
      chartsLayer: 'Charts (outline)', showChart: 'Show the chart on the map', hideChart: 'Remove from the map', openChart: 'Open the chart page',
      chartsOnMap: 'Charts on the map', opacity: 'Opacity', loadingChart: 'Loading the chart...', chartFail: 'Could not load the chart',
      chartsCut: 'More charts here than are drawn: zoom in.',
      chartSearch: 'Put a chart on the map: search by name or airport', outlines: 'Show the outline of every chart in view',
      chartNone: 'No chart found.', chartNoGeo: 'This chart has no position: it cannot go on the map.', chartsHead: 'Charts',
      secSearch: 'Find a place', secRoute: 'Route', secCharts: 'Charts', secLayers: 'What the map shows', secMine: 'My points',
      noMine: 'You have no points of your own yet.', ident: 'Identifier', lat: 'Latitude', lon: 'Longitude', createPoint: 'Create the point',
      useCenter: 'Use the centre of the map', deletePoint: 'Delete this point', confirmDelPoint: 'Delete this point?', created: 'Point created',
      chartsOnMapEmpty: 'No chart on the map yet.',
      textBox: 'Click to write or paste the route as text', textTitle: 'Route as text',
      textHelp: 'Write the route the way pilots do, for example SBSJ BOBI3A DAGUB UZ6 SBGR. Airways, SIDs and STARs are expanded into their places.',
      apply: 'Use this route', cancel: 'Cancel', textEmpty: 'Write a route first.', textNone: 'No place of this route was found.',
      nearTitle: 'Several places here', nearHelp: 'Pick the one to add to the route.', airwaysHere: 'Airways here', nearMore: 'more here: zoom in to tell them apart'
    },
    pt: {
      title: 'Mapa', hint: 'Clique num aeroporto ou fixo para acrescentar à rota. Clique na origem e depois no destino: o que você acrescentar depois entra entre os dois. Clique com o botão direito num lugar vazio para criar um ponto seu ali.',
      route: 'Rota', empty: 'A rota está vazia. Clique em aeroportos e fixos no mapa.', name: 'Nome', save: 'Salvar', clear: 'Limpar',
      validate: 'Validar', saved: 'Salva', failed: 'Não foi possível', open: 'Abrir uma rota salva...', up: 'Subir', down: 'Descer',
      remove: 'Remover', zoomIn: 'Aproxime para ver os fixos e as aerovias.', cut: 'Tem coisa demais aqui: aproxime para ver tudo.',
      airports: 'Aeroportos', fixesLayer: 'Fixos', airwaysLayer: 'Aerovias', pointsLayer: 'Meus pontos', ok: 'Todos os nomes foram encontrados.',
      problems: 'a conferir', newPoint: 'Identificador do novo ponto (letras e dígitos):', pointFailed: 'Não foi possível criar o ponto',
      noAirports: 'Ainda não há aeroportos guardados nesta área. Carregue-os na aba Navegação.', places: 'pontos',
      search: 'Buscar um ponto: identificador ou nome', noResults: 'Nada encontrado.', goTo: 'Mostrar no mapa', addTo: 'Acrescentar à rota',
      distance: 'Distância', nm: 'NM', typeAirport: 'Aeroporto', typePoint: 'Meu ponto',
      kindsShort: { W: 'Waypoint', V: 'VOR', N: 'NDB' },
      chartsLayer: 'Cartas (contorno)', showChart: 'Mostrar a carta no mapa', hideChart: 'Tirar do mapa', openChart: 'Abrir a página da carta',
      chartsOnMap: 'Cartas no mapa', opacity: 'Opacidade', loadingChart: 'Carregando a carta...', chartFail: 'Não foi possível carregar a carta',
      chartsCut: 'Há mais cartas aqui do que as desenhadas: aproxime.',
      chartSearch: 'Colocar uma carta no mapa: busque pelo nome ou aeroporto', outlines: 'Mostrar o contorno de todas as cartas da vista',
      chartNone: 'Nenhuma carta encontrada.', chartNoGeo: 'Esta carta não tem posição: não dá para colocá-la no mapa.', chartsHead: 'Cartas',
      secSearch: 'Procurar um lugar', secRoute: 'Rota', secCharts: 'Cartas', secLayers: 'O que o mapa mostra', secMine: 'Meus pontos',
      noMine: 'Você ainda não tem pontos seus.', ident: 'Identificador', lat: 'Latitude', lon: 'Longitude', createPoint: 'Criar o ponto',
      useCenter: 'Usar o centro do mapa', deletePoint: 'Excluir este ponto', confirmDelPoint: 'Excluir este ponto?', created: 'Ponto criado',
      chartsOnMapEmpty: 'Nenhuma carta no mapa ainda.',
      textBox: 'Clique para escrever ou colar a rota em texto', textTitle: 'Rota em texto',
      textHelp: 'Escreva a rota como os pilotos, por exemplo SBSJ BOBI3A DAGUB UZ6 SBGR. Aerovias, SIDs e STARs são abertas nos seus pontos.',
      apply: 'Usar esta rota', cancel: 'Cancelar', textEmpty: 'Escreva uma rota primeiro.', textNone: 'Nenhum ponto desta rota foi encontrado.',
      nearTitle: 'Vários pontos aqui', nearHelp: 'Escolha o que entra na rota.', airwaysHere: 'Aerovias aqui', nearMore: 'a mais aqui: aproxime para separá-los'
    }
  };
  const mp = (k) => (MP[lang] && MP[lang][k]) || MP.en[k] || k;
  const KIND_WORD = { W: 'waypoint', V: 'vor', N: 'ndb' };
  const plan = { id: 0, name: '', auto: true, points: [], choices: [], map: null, result: null };

  // The route as written: an airway or a procedure appears once, followed by the place it ends at
  // (the planner keeps every place the airway passes, the text does not need them).
  // Each token is { text, point }: point is the place a fix token stands for (null for a name).
  function planTokens() {
    const pts = plan.points;
    const out = [];
    for (let i = 0; i < pts.length; i++) {
      const p = pts[i];
      if (p.kind === 'procedure') {
        out.push({ text: p.ident, point: null });
        let j = i;
        while (j + 1 < pts.length && pts[j + 1].via === p.ident) j++;
        if (j > i) out.push({ text: pts[j].ident, point: pts[j] });
        i = j;
      } else if (p.via && i > 0) {
        let j = i;
        while (j + 1 < pts.length && pts[j + 1].via === p.via) j++;
        out.push({ text: p.via, point: null }, { text: pts[j].ident, point: pts[j] });
        i = j;
      } else out.push({ text: p.ident, point: p });
    }
    return out;
  }
  function planText() { return planTokens().map((t) => t.text).join(' '); }

  function planChoices() {
    // each fix is the place that was touched, even when the name exists elsewhere
    const seen = {};
    const out = [];
    planTokens().forEach((t) => {
      if (!t.point) return;
      const p = t.point;
      const n = seen[t.text] || 0;
      seen[t.text] = n + 1;
      if (p.kind !== 'airport' && p.kind !== 'coordinate' && p.kind !== 'procedure' && p.region) out.push({ token: t.text, nth: n, region: p.region, kind: p.kind });
    });
    return out;
  }
  const toPlanPoint = (p) => ({ ident: p.ident, region: p.region || '', kind: p.kind, lat: p.lat, lon: p.lon, via: p.via || '' });

  // A chart over the map: every tile is drawn by cutting the picture into small triangles and
  // placing each where the chart's own georeference says it belongs (same method as the viewer).
  let geoReady = null;
  function loadGeo() {
    if (window.LMMGeo) return Promise.resolve();
    if (!geoReady) {
      geoReady = new Promise((resolve, reject) => {
        const js = document.createElement('script');
        js.src = '/geo.js';
        js.onload = () => resolve();
        js.onerror = () => reject(new Error('geo.js'));
        document.head.appendChild(js);
      });
    }
    return geoReady;
  }

  function drawTri(ctx, img, s0, s1, s2, d0, d1, d2, fr) {
    const det = (s1.x - s0.x) * (s2.y - s0.y) - (s2.x - s0.x) * (s1.y - s0.y);
    if (Math.abs(det) < 1e-9) return;
    const a = ((d1.x - d0.x) * (s2.y - s0.y) - (d2.x - d0.x) * (s1.y - s0.y)) / det;
    const c = ((d2.x - d0.x) * (s1.x - s0.x) - (d1.x - d0.x) * (s2.x - s0.x)) / det;
    const b = ((d1.y - d0.y) * (s2.y - s0.y) - (d2.y - d0.y) * (s1.y - s0.y)) / det;
    const d = ((d2.y - d0.y) * (s1.x - s0.x) - (d1.y - d0.y) * (s2.x - s0.x)) / det;
    const e = d0.x - a * s0.x - c * s0.y;
    const f = d0.y - b * s0.x - d * s0.y;
    const cx = (d0.x + d1.x + d2.x) / 3;
    const cy = (d0.y + d1.y + d2.y) / 3;
    const grow = (pt) => {
      const dx = pt.x - cx;
      const dy = pt.y - cy;
      const len = Math.sqrt(dx * dx + dy * dy) || 1;
      return { x: pt.x + dx / len * 0.6, y: pt.y + dy / len * 0.6 };
    };
    const g0 = grow(d0), g1 = grow(d1), g2 = grow(d2);
    const sx0 = Math.max(fr.left, Math.min(s0.x, s1.x, s2.x) - 1);
    const sy0 = Math.max(fr.top, Math.min(s0.y, s1.y, s2.y) - 1);
    const sx1 = Math.min(fr.right, Math.max(s0.x, s1.x, s2.x) + 1);
    const sy1 = Math.min(fr.bottom, Math.max(s0.y, s1.y, s2.y) + 1);
    if (sx1 <= sx0 || sy1 <= sy0) return;
    ctx.save();
    ctx.beginPath();
    ctx.moveTo(g0.x, g0.y); ctx.lineTo(g1.x, g1.y); ctx.lineTo(g2.x, g2.y);
    ctx.closePath();
    ctx.clip();
    ctx.setTransform(a, b, c, d, e, f);
    ctx.drawImage(img, sx0, sy0, sx1 - sx0, sy1 - sy0, sx0, sy0, sx1 - sx0, sy1 - sy0);
    ctx.restore();
  }

  function chartWarpLayer(L, meta, img, opacity) {
    const MESH = 8;
    const Layer = L.GridLayer.extend({
      createTile(coords, done) {
        const T = 256;
        const tile = document.createElement('canvas');
        tile.width = T; tile.height = T;
        const fr = meta.mapFramePx;
        const n = Math.pow(2, coords.z);
        const ll = (x, y) => {
          const lon = x / n * 360 - 180;
          const m = Math.PI * (1 - 2 * y / n);
          return { lat: Math.atan((Math.exp(m) - Math.exp(-m)) / 2) * 180 / Math.PI, lon };
        };
        const k = img.naturalWidth / meta.image.width;   // the picture may be a reduced copy
        const src = [];
        let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
        for (let j = 0; j <= MESH; j++) {
          src[j] = [];
          for (let i = 0; i <= MESH; i++) {
            const p = ll(coords.x + i / MESH, coords.y + j / MESH);
            const px = window.LMMGeo.toPixel(meta, p.lat, p.lon);
            src[j][i] = { x: px.px * k, y: px.py * k };
            minX = Math.min(minX, px.px); maxX = Math.max(maxX, px.px);
            minY = Math.min(minY, px.py); maxY = Math.max(maxY, px.py);
          }
        }
        const inside = !(maxX < fr.left || minX > fr.right || maxY < fr.top || minY > fr.bottom);
        if (inside) {
          const ctx = tile.getContext('2d');
          ctx.imageSmoothingEnabled = true;
          const step = T / MESH;
          const frame = { left: fr.left * k, top: fr.top * k, right: fr.right * k, bottom: fr.bottom * k };
          for (let j = 0; j < MESH; j++) {
            for (let i = 0; i < MESH; i++) {
              const p00 = { x: i * step, y: j * step }, p10 = { x: (i + 1) * step, y: j * step };
              const p11 = { x: (i + 1) * step, y: (j + 1) * step }, p01 = { x: i * step, y: (j + 1) * step };
              drawTri(ctx, img, src[j][i], src[j][i + 1], src[j + 1][i + 1], p00, p10, p11, frame);
              drawTri(ctx, img, src[j][i], src[j + 1][i + 1], src[j + 1][i], p00, p11, p01, frame);
            }
          }
        }
        setTimeout(() => done(null, tile), 0);
        return tile;
      }
    });
    return new Layer({ opacity, minZoom: 3, maxZoom: 14, updateWhenIdle: true, keepBuffer: 1 });
  }

  async function renderMap() {
    if (plan.map) { plan.map.remove(); plan.map = null; }
    await loadLeaflet().catch(() => null);
    if (!window.L) { view.replaceChildren(h('div', { class: 'card' }, mp('title'), ': Leaflet')); return; }
    const L = window.L;
    const saved = (await api('GET', '/api/routes')).routes;

    const nameIn = h('input', { type: 'text', placeholder: mp('name'), value: plan.name });
    nameIn.addEventListener('input', () => { plan.name = nameIn.value; plan.auto = nameIn.value === ''; });
    const status = h('div', { class: 'muted', style: 'margin:6px 0' });
    const distHost = h('div', { class: 'plandist' });
    const chartsHost = h('div', { class: 'planchartlist' });
    const chartIn = h('input', { type: 'text', placeholder: mp('chartSearch'), autocomplete: 'off' });
    const chartRes = h('div', { class: 'searchres' });
    const outlinesCb = h('input', { type: 'checkbox' });
    const outlinesRow = h('label', { class: 'row', style: 'margin:6px 0; gap:6px; font-size:13px' }, outlinesCb, mp('outlines'));
    const searchIn = h('input', { type: 'text', placeholder: mp('search'), autocomplete: 'off' });
    const searchHost = h('div', { class: 'searchres' });
    const listHost = h('div');
    const textBox = h('div', { class: 'routebox', tabindex: '0', role: 'button', title: mp('textTitle') });
    textBox.addEventListener('click', () => openTextDialog());
    textBox.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); openTextDialog(); } });
    const resultHost = h('div');
    const hintHost = h('div', { class: 'muted planhint' });
    const openSel = h('select', { style: 'width:100%' }, h('option', { value: '' }, mp('open')),
      saved.map((r) => h('option', { value: String(r.id) }, `${r.name} (${r.summary || ''})`)));

    // everything is in the panel on the right, in sections that fold: the map itself stays clean
    const section = (id, title, open, ...body) => {
      let on = open;
      try { const v = localStorage.getItem('oc.mapsec.' + id); if (v !== null) on = v === '1'; } catch (e) { /* ignore */ }
      const d = h('details', { class: 'plansec' }, h('summary', null, title), h('div', { class: 'plansecbody' }, ...body));
      d.open = on;
      d.addEventListener('toggle', () => { try { localStorage.setItem('oc.mapsec.' + id, d.open ? '1' : '0'); } catch (e) { /* ignore */ } });
      return d;
    };
    const layerCb = {};
    const layerRow = (key, label) => {
      const cb = h('input', { type: 'checkbox' });
      cb.checked = true;
      layerCb[key] = cb;
      cb.addEventListener('change', () => {
        const lg = layers[key];
        if (cb.checked) lg.addTo(map); else map.removeLayer(lg);
        later();
      });
      return h('label', { class: 'layerrow' }, cb, label);
    };
    const mapDiv = h('div', { class: 'planmap' });
    const panel = h('div', { class: 'planpanel' },
      h('p', { class: 'muted planintro' }, mp('hint')),
      section('search', mp('secSearch'), true, searchIn, searchHost),
      section('route', mp('secRoute'), true,
        openSel,
        h('label', { class: 'fld', style: 'margin-top:8px' }, h('span', { class: 'muted' }, mp('name')), nameIn),
        textBox, listHost, distHost,
        h('div', { class: 'row', style: 'margin-top:8px' },
          h('button', { class: 'btn', onclick: () => validatePlan() }, mp('validate')),
          h('button', { class: 'btn primary', onclick: () => savePlan() }, mp('save')),
          h('button', { class: 'btn', onclick: () => { plan.points = []; plan.id = 0; plan.name = ''; plan.auto = true; nameIn.value = ''; plan.result = null; redrawPlan(); } }, mp('clear'))),
        status, resultHost),
      section('charts', mp('secCharts'), true, chartIn, chartRes, outlinesRow, chartsHost),
      section('layers', mp('secLayers'), false,
        layerRow('airports', mp('airports')), layerRow('fixes', mp('fixesLayer')), layerRow('segs', mp('airwaysLayer')), layerRow('mine', mp('pointsLayer'))),
      hintHost);
    view.replaceChildren(h('div', { class: 'plan' }, mapDiv, panel));

    const map = L.map(mapDiv, { zoomControl: true, attributionControl: true, doubleClickZoom: false });
    plan.map = map;
    L.tileLayer('/tiles/{z}/{x}/{y}.png', { maxZoom: 17, maxNativeZoom: 16, attribution: '&copy; OpenStreetMap contributors' }).addTo(map);
    const layers = {
      segs: L.layerGroup().addTo(map), fixes: L.layerGroup().addTo(map), airports: L.layerGroup().addTo(map),
      mine: L.layerGroup().addTo(map), route: L.layerGroup().addTo(map),
      charts: L.layerGroup()   // off until the user turns it on
    };
    const overlays = {};   // id -> { layer, meta, name }
    const known = { airports: [], fixes: [], segs: [], mine: [] };   // what the map shows now, for the clicks

    // With an airport at each end the route has its origin and destination: what is added after
    // goes between them. Until then, each place is added at the end.
    const add = (p) => {
      const n = plan.points.length;
      const ends = n >= 2 && plan.points[0].kind === 'airport' && plan.points[n - 1].kind === 'airport';
      if (ends) plan.points.splice(n - 1, 0, p); else plan.points.push(p);
      plan.result = null; autoName(); redrawPlan();
    };
    const colorOf = { W: '#6b7280', V: '#15803d', N: '#92400e' };

    function autoName() {
      if (!plan.auto || plan.id) return;
      const n = plan.points.length;
      plan.name = n > 1 ? `${plan.points[0].ident} > ${plan.points[n - 1].ident}` : (n === 1 ? plan.points[0].ident : '');
      nameIn.value = plan.name;
    }

    function redrawPlan() {
      layers.route.clearLayers();
      const pts = plan.points.map((p) => [p.lat, p.lon]);
      if (pts.length > 1) L.polyline(pts, { color: '#c2185b', weight: 4, opacity: 0.9, interactive: false }).addTo(layers.route);
      plan.points.forEach((p, i) => {
        L.circleMarker([p.lat, p.lon], { radius: 6, color: '#c2185b', weight: 2, fillColor: '#fff', fillOpacity: 1, interactive: false })
          .bindTooltip(String(i + 1), { permanent: true, direction: 'top', offset: [0, -4], className: 'plannum' }).addTo(layers.route);
      });
      listHost.replaceChildren(...(plan.points.length === 0 ? [h('p', { class: 'muted' }, mp('empty'))] : [
        h('div', { class: 'planlist' }, plan.points.map((p, i) => h('div', { class: 'planrow' },
          h('span', { class: 'plannumb' }, String(i + 1)),
          h('span', { class: 'grow' }, h('strong', null, p.ident), h('span', { class: 'muted' }, ' ' + (p.region || '') + ' ' + (p.kind === 'airport' ? 'AP' : (p.kind || '').toUpperCase())),
            h('span', { class: 'planleg muted', 'data-i': String(i) })),
          h('button', { class: 'btn sm', title: mp('up'), disabled: i === 0, onclick: () => { swap(i, i - 1); } }, '↑'),
          h('button', { class: 'btn sm', title: mp('down'), disabled: i === plan.points.length - 1, onclick: () => { swap(i, i + 1); } }, '↓'),
          h('button', { class: 'btn sm danger', title: mp('remove'), onclick: () => { plan.points.splice(i, 1); plan.result = null; autoName(); redrawPlan(); } }, '×'))))]));
      resultHost.replaceChildren();
      textBox.textContent = planText() || mp('textBox');
      textBox.classList.toggle('empty', plan.points.length === 0);
      showDistance();
    }

    // The length of the route is the server's: the planner only shows it, a moment after the last change.
    let distTimer = null;
    function showDistance() {
      clearTimeout(distTimer);
      if (plan.points.length < 2) { distHost.textContent = ''; return; }
      distTimer = setTimeout(async () => {
        try {
          const d = await api('POST', '/api/route/distance', { points: plan.points.map((p) => ({ lat: p.lat, lon: p.lon })) });
          distHost.textContent = `${mp('distance')}: ${fmtNum(d.totalNm)} ${mp('nm')}`;
          listHost.querySelectorAll('.planleg').forEach((el) => {
            const v = d.legs[Number(el.getAttribute('data-i'))];
            el.textContent = v ? ` \u00b7 ${fmtNum(v)} ${mp('nm')}` : '';
          });
        } catch (e) { distHost.textContent = ''; }
      }, 150);
    }
    function swap(a, b) {
      const t0 = plan.points[a]; plan.points[a] = plan.points[b]; plan.points[b] = t0;
      plan.result = null; autoName(); redrawPlan();
    }

    function showResult(res) {
      resultHost.replaceChildren(
        h('div', null, h('span', { class: 'badge' }, `${res.points.length} ${mp('places')}`),
          h('span', { class: 'badge' }, `${fmtNum(res.totalNm)} ${mp('nm')}`),
          res.problems.length ? h('span', { class: 'badge warn' }, `${res.problems.length} ${mp('problems')}`) : h('span', { class: 'badge ok' }, mp('ok'))),
        res.problems.length ? h('div', { class: 'warnbox', style: 'margin-top:6px' }, res.problems.map((q) => h('div', null, h('strong', null, q.token), ' ', q.message))) : null);
    }

    async function validatePlan() {
      if (!plan.points.length) return;
      status.textContent = '...';
      try {
        const res = await api('POST', '/api/route/resolve', { route: planText(), choices: planChoices() });
        status.textContent = '';
        showResult(res);
      } catch (e) { status.textContent = mp('failed') + ': ' + e.message; }
    }

    // The route as text, in a bigger box: write it, paste it or change it.
    function openTextDialog() {
      const ta = h('textarea', { rows: 6, class: 'routetext', spellcheck: 'false', placeholder: 'SBSJ BOBI3A DAGUB UZ6 SBGR' });
      ta.value = planText();
      const msg = h('div', { class: 'muted', style: 'margin:8px 0; min-height:18px' });
      const onKey = (e) => { if (e.key === 'Escape') close(); };
      const close = () => { back.remove(); document.removeEventListener('keydown', onKey); };
      const apply = async () => {
        const text = ta.value.trim();
        if (!text) { msg.textContent = mp('textEmpty'); return; }
        msg.textContent = '...';
        let res;
        try { res = await api('POST', '/api/route/resolve', { route: text, choices: planChoices() }); } catch (e) { msg.textContent = mp('failed') + ': ' + e.message; return; }
        if (!res.points.length) {
          msg.replaceChildren(mp('textNone'), ...res.problems.map((q) => h('div', null, h('strong', null, q.token || ''), ' ', q.message)));
          return;
        }
        plan.points = res.points.map(toPlanPoint);
        plan.result = null;
        autoName();
        redrawPlan();
        showResult(res);
        map.fitBounds(L.latLngBounds(plan.points.map((q) => [q.lat, q.lon])), { padding: [40, 40] });
        close();
      };
      ta.addEventListener('keydown', (e) => { if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); apply(); } });
      const back = h('div', { class: 'modal' },
        h('div', { class: 'card modalbox', style: 'max-width:640px' },
          h('h3', null, mp('textTitle')),
          h('p', { class: 'muted', style: 'margin:0 0 8px' }, mp('textHelp')),
          ta, msg,
          h('div', { class: 'row' },
            h('button', { class: 'btn primary', onclick: apply }, mp('apply')),
            h('button', { class: 'btn', onclick: close }, mp('cancel')))));
      back.addEventListener('mousedown', (e) => { if (e.target === back) close(); });
      document.addEventListener('keydown', onKey);
      document.body.appendChild(back);
      ta.focus();
    }

    async function savePlan() {
      if (plan.points.length < 1) return;
      const body = { name: nameIn.value, text: planText(), choices: planChoices() };
      try {
        const r = plan.id ? await api('PUT', '/api/routes/' + plan.id, body) : await api('POST', '/api/routes', body);
        plan.id = r.id; plan.name = r.name; nameIn.value = r.name;
        toast(mp('saved') + ': ' + r.name);
      } catch (e) { toast(mp('failed') + ': ' + e.message); }
    }

    openSel.addEventListener('change', async () => {
      const r = saved.find((x) => String(x.id) === openSel.value);
      if (!r) return;
      status.textContent = '...';
      try {
        const res = r.result || await api('POST', '/api/route/resolve', { route: r.text, choices: r.choices || [] });
        plan.id = r.id; plan.name = r.name; plan.auto = false; nameIn.value = r.name;
        plan.points = res.points.map(toPlanPoint);
        plan.result = null;
        redrawPlan();
        status.textContent = '';
        if (plan.points.length) map.fitBounds(L.latLngBounds(plan.points.map((p) => [p.lat, p.lon])), { padding: [40, 40] });
      } catch (e) { status.textContent = mp('failed') + ': ' + e.message; }
    });

    // ---- what is on the map follows the view
    let seq = 0;
    let timer = null;
    async function refresh() {
      const my = ++seq;
      const z = map.getZoom();
      const b = map.getBounds().pad(0.1);
      const q = `minLat=${b.getSouth()}&minLon=${b.getWest()}&maxLat=${b.getNorth()}&maxLon=${b.getEast()}&fixes=${z >= 7 ? 1 : 0}&segments=${z >= 6 ? 1 : 0}`;
      let a;
      try { a = await api('GET', '/api/nav/area?' + q); } catch (e) { return; }
      if (my !== seq) return;
      layers.airports.clearLayers(); layers.fixes.clearLayers(); layers.segs.clearLayers();
      known.airports = a.airports; known.fixes = a.fixes; known.segs = a.segments;
      a.airports.forEach((x) => {
        const m = L.marker([x.lat, x.lon], { icon: L.divIcon({ className: 'plan-ap', html: `<b>${x.icao}</b>`, iconSize: null, iconAnchor: [-6, 8] }) });
        m.bindTooltip(`${x.icao} ${x.name || ''}`);
        m.on('click', (ev) => pickPlace(ev, airportPlace(x)));
        m.addTo(layers.airports);
        L.circleMarker([x.lat, x.lon], { radius: 6, color: '#1d4ed8', weight: 2, fillColor: '#bfdbfe', fillOpacity: 1 })
          .on('click', (ev) => pickPlace(ev, airportPlace(x))).addTo(layers.airports);
      });
      a.fixes.forEach((x) => {
        const m = L.circleMarker([x.lat, x.lon], { radius: x.kind === 'W' ? 3 : 5, color: colorOf[x.kind] || '#6b7280', weight: 1.5, fillOpacity: 0.7 });
        m.bindTooltip(`${x.ident} ${x.region} ${KIND_WORD[x.kind] || ''}${x.name ? ' ' + x.name : ''}`);
        m.on('click', (ev) => pickPlace(ev, fixPlace(x)));
        m.addTo(layers.fixes);
      });
      a.segments.forEach((s) => {
        L.polyline([[s.lat, s.lon], [s.toLat, s.toLon]], { color: '#94a3b8', weight: 1.5, opacity: 0.8 }).bindTooltip(s.airway, { sticky: true }).addTo(layers.segs);
      });
      layers.charts.clearLayers();
      let chartsCut = false;
      if (map.hasLayer(layers.charts) && z >= 6) {
        try {
          const f = await api('GET', `/api/charts/footprints?minLat=${b.getSouth()}&minLon=${b.getWest()}&maxLat=${b.getNorth()}&maxLon=${b.getEast()}&limit=300`);
          if (my !== seq) return;
          chartsCut = f.cut;
          f.charts.forEach((c) => {
            const poly = L.polygon(c.corners.map((p) => [p.lat, p.lon]), { color: '#2563eb', weight: 1.5, fillOpacity: 0.04 });
            poly.bindTooltip(c.name || c.id, { sticky: true });
            poly.on('click', (ev) => {
              L.DomEvent.stopPropagation(ev);
              const on = !!overlays[c.id];
              const box = h('div', { class: 'chartpop' },
                h('strong', null, c.name || c.id),
                h('div', { class: 'row', style: 'margin-top:8px' },
                  h('button', { class: 'btn sm primary', onclick: () => { map.closePopup(); on ? removeOverlay(c.id) : showOverlay(c); } }, on ? mp('hideChart') : mp('showChart')),
                  h('a', { class: 'btn sm', href: '#chart/' + encodeURIComponent(c.id) }, mp('openChart'))));
              L.popup().setLatLng(ev.latlng).setContent(box).openOn(map);
            });
            poly.addTo(layers.charts);
          });
        } catch (e) { /* the outlines are an extra */ }
      }
      let hint = '';
      if (chartsCut) hint = mp('chartsCut');
      else if (z < 6) hint = mp('zoomIn');
      else if (a.fixesCut || a.segmentsCut) hint = mp('cut');
      else if (!a.airports.length && z >= 6) hint = mp('noAirports');
      hintHost.textContent = hint;
    }
    const later = () => { clearTimeout(timer); timer = setTimeout(refresh, 250); };
    map.on('moveend', later);

    // ---- charts over the map
    function drawChartList() {
      chartsHost.replaceChildren(...(Object.keys(overlays).length === 0 ? [] : [
        h('h3', { style: 'margin:10px 0 4px' }, mp('chartsOnMap')),
        ...Object.keys(overlays).map((id) => {
          const o = overlays[id];
          const slider = h('input', { type: 'range', min: 20, max: 100, value: String(Math.round(o.opacity * 100)), title: mp('opacity'), style: 'width:80px' });
          slider.addEventListener('input', () => { o.opacity = slider.value / 100; o.layer.setOpacity(o.opacity); });
          return h('div', { class: 'planrow' },
            h('span', { class: 'grow', title: o.name }, o.name),
            slider,
            h('button', { class: 'btn sm danger', title: mp('hideChart'), onclick: () => removeOverlay(id) }, '\u00d7'));
        })]));
    }
    function removeOverlay(id) {
      const o = overlays[id];
      if (!o) return;
      map.removeLayer(o.layer);
      delete overlays[id];
      drawChartList();
    }
    async function showOverlay(c) {
      if (overlays[c.id]) {
        if (c.goTo && overlays[c.id].meta.geoCorners) map.fitBounds(L.latLngBounds(overlays[c.id].meta.geoCorners.map((p) => [p.lat, p.lon])), { padding: [30, 30] });
        return;
      }
      status.textContent = mp('loadingChart');
      try {
        await loadGeo();
        const meta = await (await fetch('/charts/' + encodeURIComponent(c.id) + '/chart.json?v=' + Date.now())).json();
        // a pyramid keeps the whole chart small in chart.png; any other chart: a reduced copy is enough here
        const file = meta.tiles || meta.image.file !== 'chart.png' ? meta.image.file : (meta.image.width > 3000 ? 'chart@4.png' : 'chart.png');
        const img = await new Promise((resolve, reject) => {
          const im = new Image();
          im.onload = () => resolve(im);
          im.onerror = () => reject(new Error('image'));
          im.src = '/charts/' + encodeURIComponent(c.id) + '/' + file;
        });
        const layer = chartWarpLayer(L, meta, img, 0.75).addTo(map);
        overlays[c.id] = { layer, meta, name: c.name || c.id, opacity: 0.75 };
        status.textContent = '';
        drawChartList();
        if (c.goTo && meta.geoCorners && meta.geoCorners.length) {
          map.fitBounds(L.latLngBounds(meta.geoCorners.map((p) => [p.lat, p.lon])), { padding: [30, 30] });
        }
      } catch (e) { status.textContent = mp('chartFail'); }
    }

    outlinesCb.addEventListener('change', () => {
      if (outlinesCb.checked) map.addLayer(layers.charts); else map.removeLayer(layers.charts);
      later();
    });
    let chartTimer = null;
    let chartSeq = 0;
    chartIn.addEventListener('input', () => {
      clearTimeout(chartTimer);
      chartTimer = setTimeout(async () => {
        const q = chartIn.value.trim();
        const my = ++chartSeq;
        if (q.length < 2) { chartRes.replaceChildren(); return; }
        let list;
        try { list = await api('GET', '/api/charts?limit=12&q=' + encodeURIComponent(q)); } catch (e) { return; }
        if (my !== chartSeq) return;
        if (!list.length) { chartRes.replaceChildren(h('div', { class: 'muted' }, mp('chartNone'))); return; }
        chartRes.replaceChildren(...list.map((c) => {
          const noGeo = !!c.unreferenced;
          return h('div', { class: 'searchrow', title: noGeo ? mp('chartNoGeo') : mp('showChart'), style: noGeo ? 'opacity:.5' : '',
            onclick: () => { if (!noGeo) showOverlay({ id: c.id, name: c.name, goTo: true }); } },
            h('span', { class: 'grow' }, h('strong', null, c.icao || ''), ' ', h('span', { class: 'muted' }, c.name)),
            noGeo ? h('span', { class: 'badge warn' }, '\u2014') : h('button', { class: 'btn sm', title: mp('showChart'), onclick: (ev) => { ev.stopPropagation(); showOverlay({ id: c.id, name: c.name, goTo: true }); } }, '+'));
        }));
      }, 250);
    });

    // ---- a click on a mark: when other places are close to it the click is ambiguous, so nothing is
    // added; a box lists them (and the airways that pass there) and the user picks
    const airportPlace = (x) => ({ point: { ident: x.icao, region: '', kind: 'airport', lat: x.lat, lon: x.lon }, label: mp('typeAirport'), name: x.name || '' });
    const fixPlace = (x) => ({ point: { ident: x.ident, region: x.region, kind: KIND_WORD[x.kind] || 'waypoint', lat: x.lat, lon: x.lon }, label: MP[lang].kindsShort[x.kind] || '', name: x.name || '' });
    const minePlace = (x) => ({ point: { ident: x.ident, region: 'USR', kind: 'waypoint', lat: x.lat, lon: x.lon }, label: mp('typePoint'), name: x.name || '' });
    const NEAR_PX = 14;
    const NEAR_MAX = 12;
    function placesNear(center) {
      const c = map.latLngToContainerPoint(center);
      const out = [];
      const consider = (pl) => {
        const d = c.distanceTo(map.latLngToContainerPoint([pl.point.lat, pl.point.lon]));
        if (d <= NEAR_PX) out.push({ pl, d });
      };
      if (map.hasLayer(layers.airports)) known.airports.forEach((x) => consider(airportPlace(x)));
      if (map.hasLayer(layers.fixes)) known.fixes.forEach((x) => consider(fixPlace(x)));
      if (map.hasLayer(layers.mine)) known.mine.forEach((x) => consider(minePlace(x)));
      return out.sort((p, q) => p.d - q.d).map((o) => o.pl);
    }
    function airwaysNear(center) {
      if (!map.hasLayer(layers.segs)) return [];
      const c = map.latLngToContainerPoint(center);
      const names = new Set();
      known.segs.forEach((sg) => {
        const a = map.latLngToContainerPoint([sg.lat, sg.lon]);
        const b = map.latLngToContainerPoint([sg.toLat, sg.toLon]);
        const dx = b.x - a.x, dy = b.y - a.y;
        const len2 = dx * dx + dy * dy;
        const t = len2 === 0 ? 0 : Math.max(0, Math.min(1, ((c.x - a.x) * dx + (c.y - a.y) * dy) / len2));
        if (Math.hypot(c.x - (a.x + t * dx), c.y - (a.y + t * dy)) <= 8) names.add(sg.airway);
      });
      return Array.from(names).sort();
    }
    function pickPlace(ev, pl) {
      L.DomEvent.stopPropagation(ev);
      const center = ev.latlng || L.latLng(pl.point.lat, pl.point.lon);
      const near = placesNear(center);
      if (near.length <= 1) { add(pl.point); return; }
      const ways = airwaysNear(center);
      const box = h('div', { class: 'nearbox' },
        h('strong', null, mp('nearTitle')),
        h('div', { class: 'muted', style: 'margin:2px 0 6px' }, mp('nearHelp')),
        h('div', { class: 'nearlist' }, near.slice(0, NEAR_MAX).map((q) => h('div', { class: 'nearrow' },
          h('span', { class: 'grow' }, h('strong', null, q.point.ident), ' ', h('span', { class: 'muted' }, [q.point.region, q.label, q.name].filter(Boolean).join(' \u00b7 '))),
          h('button', { class: 'btn sm', title: mp('addTo'), onclick: () => { map.closePopup(); add(q.point); } }, '+')))),
        near.length > NEAR_MAX ? h('div', { class: 'muted', style: 'margin-top:4px' }, `+${near.length - NEAR_MAX} ` + mp('nearMore')) : null,
        ways.length ? h('div', { class: 'muted', style: 'margin-top:6px' }, mp('airwaysHere') + ': ', h('strong', null, ways.join(', '))) : null);
      L.popup({ maxWidth: 340, className: 'nearpop', autoPanPadding: [20, 30] }).setLatLng(center).setContent(box).openOn(map);
    }

    // ---- search: when there are many small marks, ask for the one by name
    let searchSeq = 0;
    let searchTimer = null;
    let ring = null;
    function showPlace(h0) {
      if (ring) { map.removeLayer(ring); ring = null; }
      map.setView([h0.lat, h0.lon], Math.max(map.getZoom(), 11));
      ring = L.circleMarker([h0.lat, h0.lon], { radius: 16, color: '#f59e0b', weight: 4, fillOpacity: 0, interactive: false }).addTo(map);
      const mine = ring;
      setTimeout(() => { if (ring === mine) { map.removeLayer(mine); ring = null; } }, 8000);
    }
    searchIn.addEventListener('input', () => {
      clearTimeout(searchTimer);
      searchTimer = setTimeout(async () => {
        const q = searchIn.value.trim();
        const my = ++searchSeq;
        if (q.length < 2) { searchHost.replaceChildren(); return; }
        let r;
        try { r = await api('GET', '/api/nav/search?q=' + encodeURIComponent(q) + '&limit=12'); } catch (e) { return; }
        if (my !== searchSeq) return;
        if (!r.results.length) { searchHost.replaceChildren(h('div', { class: 'muted' }, mp('noResults'))); return; }
        searchHost.replaceChildren(...r.results.map((x) => {
          const kind = x.type === 'airport' ? mp('typeAirport') : (x.type === 'point' ? mp('typePoint') : (MP[lang].kindsShort[x.kind] || ''));
          const pt = x.type === 'airport' ? { ident: x.ident, region: '', kind: 'airport', lat: x.lat, lon: x.lon }
            : (x.type === 'point' ? { ident: x.ident, region: 'USR', kind: 'waypoint', lat: x.lat, lon: x.lon }
              : { ident: x.ident, region: x.region, kind: KIND_WORD[x.kind] || 'waypoint', lat: x.lat, lon: x.lon });
          return h('div', { class: 'searchrow', title: mp('goTo'), onclick: () => showPlace(x) },
            h('span', { class: 'grow' }, h('strong', null, x.ident), ' ', h('span', { class: 'muted' }, [x.region, kind, x.name].filter(Boolean).join(' \u00b7 '))),
            h('button', { class: 'btn sm', title: mp('addTo'), onclick: (ev) => { ev.stopPropagation(); add(pt); } }, '+'));
        }));
      }, 250);
    });

    // ---- the user's own points
    const mineLayer = async () => {
      layers.mine.clearLayers();
      const pts = (await api('GET', '/api/nav/points')).points;
      known.mine = pts;
      pts.forEach((p) => {
        const m = L.circleMarker([p.lat, p.lon], { radius: 5, color: '#7c3aed', weight: 2, fillColor: '#ddd6fe', fillOpacity: 1 });
        m.bindTooltip(`${p.ident}${p.name ? ' ' + p.name : ''}`);
        m.on('click', (ev) => pickPlace(ev, minePlace(p)));
        m.addTo(layers.mine);
      });
    };
    mineLayer();
    map.on('contextmenu', async (ev) => {
      const ident = (prompt(mp('newPoint'), '') || '').trim().toUpperCase();
      if (!ident) return;
      try {
        const p = await api('POST', '/api/nav/points', { ident, name: '', lat: Number(ev.latlng.lat.toFixed(5)), lon: Number(ev.latlng.lng.toFixed(5)) });
        await mineLayer();
        add({ ident: p.ident, region: 'USR', kind: 'waypoint', lat: p.lat, lon: p.lon });
      } catch (e) { toast(mp('pointFailed') + ': ' + e.message); }
    });

    // the first view: the route in progress; else where the aircraft is; else the last route that was
    // saved; else where the stored airports are; else Brazil
    if (plan.points.length) map.fitBounds(L.latLngBounds(plan.points.map((p) => [p.lat, p.lon])), { padding: [40, 40] });
    else {
      let placed = false;
      try {
        const pos = await api('GET', '/api/position');
        if (pos && pos.valid && pos.lat !== undefined && !pos.simulated) { map.setView([pos.lat, pos.lon], 9); placed = true; }
      } catch (e) { /* no position */ }
      if (!placed && saved.length) {
        try {
          const res = saved[0].result || await api('POST', '/api/route/resolve', { route: saved[0].text, choices: saved[0].choices || [] });
          if (res.points.length) {
            map.fitBounds(L.latLngBounds(res.points.map((p) => [p.lat, p.lon])), { padding: [40, 40] });
            placed = true;
          }
        } catch (e) { /* the next one */ }
      }
      if (!placed) {
        let c = [-15, -50], z = 5;
        try {
          const first = (await api('GET', '/api/nav/airports')).airports[0];
          if (first) {
            const ap = await api('GET', '/api/nav/airport/' + encodeURIComponent(first.icao));
            if (ap && ap.lat) { c = [ap.lat, ap.lon]; z = 8; }
          }
        } catch (e) { /* default */ }
        map.setView(c, z);
      }
    }
    redrawPlan();
    refresh();
    setTimeout(() => map.invalidateSize(), 80);
  }

  // ---- Guide: what every button of the in-simulator screen does. The simulator's browser shows no
  // tooltips, so this page is the legend: a drawing of the screen with numbers, and the list.
  const G = (d) => '<svg viewBox="0 0 32 32" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">' + d + '</svg>';
  const ICON = {
    route: G('<path d="M5 25l8-12 6 7 8-12"/><circle cx="5" cy="25" r="2.2"/><circle cx="27" cy="8" r="2.2"/>'),
    pin: G('<path d="M11 4h10l-1 9 4 5H8l4-5z"/><path d="M16 18v10"/>'),
    search: G('<circle cx="14" cy="14" r="8"/><path d="M20 20l8 8"/>'),
    strip: G('<path d="M5 9h13M5 16h13M5 23h13"/><path d="M23 7h6l-.7 6 2.7 3.4H20l2.7-3.4z"/><path d="M26 16.5V26"/>'),
    eye: G('<path d="M3 16C6.5 10 11 7.5 16 7.5S25.5 10 29 16C25.5 22 21 24.5 16 24.5S6.5 22 3 16z"/><circle cx="16" cy="16" r="4"/>'),
    zin: G('<path d="M16 7v18M7 16h18"/>'),
    zout: G('<path d="M7 16h18"/>'),
    follow: G('<circle cx="16" cy="16" r="7"/><path d="M16 3v6M16 23v6M3 16h6M23 16h6"/>'),
    orient: G('<circle cx="16" cy="16" r="12"/><path d="M16 6l3.5 10h-7z" fill="#d32f2f" stroke="#d32f2f"/><path d="M16 26l-3.5-10h7z"/>'),
    regional: G('<circle cx="16" cy="16" r="11"/><path d="M16 8.5l4 11-4-2.5-4 2.5z"/>'),
    fit: G('<path d="M5 12V5h7M20 5h7v7M27 20v7h-7M12 27H5v-7"/>'),
    reload: G('<path d="M26 16a10 10 0 1 1-3-7.1"/><path d="M24 4v6h-6"/>'),
    close: G('<path d="M9 9l14 14M23 9L9 23"/>'),
    overlay: G('<path d="M16 5l11 6-11 6-11-6z"/><path d="M5 17l11 6 11-6"/><path d="M5 22l11 6 11-6"/>'),
    chart: G('<rect x="8" y="4" width="16" height="24" rx="1.5"/><path d="M11 10h10M11 15h10M11 20h6"/>'),
    blend: G('<circle cx="16" cy="16" r="11"/><circle cx="16" cy="16" r="7.5" fill="currentColor" stroke="none" fill-opacity=".5"/>'),
    multi: G('<rect x="4" y="9" width="17" height="17" rx="1.5"/><path d="M10 5h17.5v17.5"/><path d="M9 15h8M9 20h5"/>'),
    updown: G('<path d="M6 12l10-8 10 8M6 20l10 8 10-8"/>'),
    bang: '<svg viewBox="0 0 32 32"><circle cx="16" cy="16" r="12" fill="#f59e0b" stroke="none"/><path d="M16 9v9" stroke="#fff" stroke-width="3.2"/><circle cx="16" cy="23" r="1.8" fill="#fff" stroke="none"/></svg>',
    dots: G('<circle cx="16" cy="7" r="2" fill="currentColor"/><circle cx="16" cy="16" r="2" fill="currentColor"/><circle cx="16" cy="25" r="2" fill="currentColor"/>')
  };

  // x, y, w, h: where the button is on the 468 x 696 screen of the simulator's tablet (lists: no position)
  const GUIDE = [
    { key: 'box', x: 10, y: 38, w: 109, h: 44, group: 'top',
      en: ['Route box', 'Shows the route in use (origin > destination). Tap the text to open the box where you write, paste or change the route as text.'],
      pt: ['Caixa da rota', 'Mostra a rota em uso (origem > destino). Toque no texto para abrir a caixa onde você escreve, cola ou altera a rota como texto.'] },
    { key: 'route', x: 125, y: 38, w: 44, h: 44, group: 'top',
      en: ['Route places', 'Opens the list of the places of the route (each with a menu: Direct to, Remove) and, below it, your saved routes from the Routes tab of the manager: one tap loads one.'],
      pt: ['Pontos da rota', 'Abre a lista dos pontos da rota (cada um com um menu: Direto para, Remover) e, abaixo, as suas rotas salvas na aba Rotas do gerenciador: um toque carrega uma.'] },
    { key: 'pin', x: 175, y: 38, w: 90, h: 44, group: 'top',
      en: ['Pin / Pinned', 'Adds the chart on screen to your list of pinned charts (the button then reads "Pinned"). Tap again to unpin.'],
      pt: ['Fixar / Fixada', 'Coloca a carta que está na tela na sua lista de cartas fixadas (o botão passa a dizer "Fixada"). Toque de novo para desafixar.'] },
    { key: 'search', x: 314, y: 38, w: 44, h: 44, group: 'top',
      en: ['Search charts', 'Opens the chart search. With a route and nothing typed, it already lists the charts of the route airports (the ones for the procedures in your route come first).'],
      pt: ['Buscar cartas', 'Abre a busca de cartas. Com uma rota e nada digitado, já lista as cartas dos aeroportos da rota (as dos procedimentos da rota vêm primeiro).'] },
    { key: 'strip', x: 364, y: 38, w: 44, h: 44, group: 'top',
      en: ['Pinned charts', 'Opens your list of pinned charts, grouped by airport. Tap one to open it.'],
      pt: ['Cartas fixadas', 'Abre a sua lista de cartas fixadas, agrupadas por aeroporto. Toque numa para abrir.'] },
    { key: 'eye', x: 414, y: 38, w: 44, h: 44, group: 'top',
      en: ['Hide controls', 'Hides every button so only the chart or the map is left. Tap again to bring them back.'],
      pt: ['Ocultar controles', 'Esconde todos os botões e deixa só a carta ou o mapa. Toque de novo para voltar.'] },
    { key: 'zin', x: 10, y: 214, w: 48, h: 48, group: 'left',
      en: ['Zoom in', 'Brings the map or chart closer.'], pt: ['Aproximar', 'Aproxima o mapa ou a carta.'] },
    { key: 'zout', x: 10, y: 268, w: 48, h: 48, group: 'left',
      en: ['Zoom out', 'Moves the map or chart away.'], pt: ['Afastar', 'Afasta o mapa ou a carta.'] },
    { key: 'follow', x: 10, y: 322, w: 48, h: 48, group: 'left',
      en: ['Follow aircraft', 'Keeps the aircraft in the centre of the screen. Dragging the map turns it off; tap to turn it on again.'],
      pt: ['Seguir a aeronave', 'Mantém a aeronave no centro da tela. Arrastar o mapa desliga; toque para ligar de novo.'] },
    { key: 'orient', x: 10, y: 376, w: 48, h: 48, group: 'left',
      en: ['Map orientation', 'A compass. North up by default (the needle points up). Tap it and the map turns with the aircraft, which always points up and stays in the centre; the button then shows an arrow in a ring whose N turns to show where north is. Tap again, or turn follow off, to go back to north up. While the map turns, dragging it is off.'],
      pt: ['Orientação do mapa', 'Uma bússola. Por padrão, norte para cima (a agulha aponta para cima). Toque e o mapa gira junto com a aeronave, que fica sempre apontando para cima e no centro; o botão passa a mostrar uma seta dentro de um anel cujo N gira para mostrar onde está o norte. Toque de novo, ou desligue o seguir, para voltar ao norte para cima. Enquanto o mapa gira, arrastar fica desligado.'] },
    { key: 'regional', x: 10, y: 430, w: 48, h: 48, group: 'left',
      en: ['Regional zoom', 'Frames about 30 km around the aircraft. Good for big charts that cover a whole region.'],
      pt: ['Zoom regional', 'Enquadra cerca de 30 km em volta da aeronave. Bom para cartas grandes que cobrem uma região.'] },
    { key: 'fit', x: 10, y: 484, w: 48, h: 48, group: 'left',
      en: ['Whole chart', 'Shows the whole chart on the screen.'], pt: ['Carta inteira', 'Mostra a carta inteira na tela.'] },
    { key: 'reload', x: 10, y: 538, w: 48, h: 48, group: 'left',
      en: ['Reload', 'Reloads this screen. Use it if something looks stuck.'], pt: ['Recarregar', 'Recarrega esta tela. Use se algo parecer travado.'] },
    { key: 'close', x: 410, y: 322, w: 48, h: 48, group: 'right',
      en: ['Close chart', 'Takes the chart off the screen. With several charts on the map, the one below comes up.'],
      pt: ['Fechar a carta', 'Tira a carta da tela. Com várias cartas no mapa, a que está embaixo sobe.'] },
    { key: 'overlay', x: 410, y: 376, w: 48, h: 48, group: 'right',
      en: ['Chart over the map', 'Shows the chart placed over the map, in its real position, with your aircraft on it.'],
      pt: ['Carta sobre o mapa', 'Mostra a carta posicionada sobre o mapa, no lugar real, com a sua aeronave nela.'] },
    { key: 'chart', x: 410, y: 430, w: 48, h: 48, group: 'right',
      en: ['Chart only', 'Shows only the chart, with no map. Charts that cannot be placed on the map (view-only) always open like this.'],
      pt: ['Só a carta', 'Mostra só a carta, sem mapa. Cartas que não podem ser posicionadas no mapa (só visualização) sempre abrem assim.'] },
    { key: 'blend', x: 410, y: 484, w: 48, h: 48, group: 'right',
      en: ['Chart opacity', 'Steps through how much of the chart shows over the map: 100%, 75%, 50%, 25%.'],
      pt: ['Opacidade da carta', 'Alterna quanto da carta aparece sobre o mapa: 100%, 75%, 50%, 25%.'] },
    { key: 'multi', x: 410, y: 538, w: 48, h: 48, group: 'right',
      en: ['Several charts', 'When on, opening another chart keeps the previous ones on the map under it (up to 5 together). The view does not move.'],
      pt: ['Várias cartas', 'Quando ligado, abrir outra carta mantém as anteriores no mapa por baixo dela (até 5 juntas). A vista não se mexe.'] },
    { key: 'pin', group: 'lists',
      en: ['Pushpin on a row', 'In the search and in the pinned list: filled means pinned. Tap to pin or unpin that chart without opening it.'],
      pt: ['Pushpin na linha', 'Na busca e na lista de fixadas: cheio quer dizer fixada. Toque para fixar ou desafixar aquela carta sem abri-la.'] },
    { key: 'eye', group: 'lists',
      en: ['Eye on a row', 'Only for charts that are on the map. Moves the view to that chart. Adding a chart never moves the view by itself.'],
      pt: ['Olho na linha', 'Só para cartas que estão no mapa. Leva a vista até aquela carta. Adicionar uma carta nunca mexe na vista sozinho.'] },
    { key: 'updown', group: 'lists',
      en: ['Up / down arrows', 'At the bottom of every list. They scroll the list, because touch scrolling is hard on the simulator screen.'],
      pt: ['Setas subir / descer', 'No rodapé de toda lista. Rolam a lista, porque rolar com o toque é difícil na tela do simulador.'] },
    { key: 'bang', group: 'lists',
      en: ['Orange "!" on a route place', 'More than one place has that name. The closest to the previous point is already used, so you do not have to do anything; tap the mark to see the others and pick one. It turns grey once you chose.'],
      pt: ['"!" laranja num ponto da rota', 'Mais de um lugar tem esse nome. Já está sendo usado o mais próximo do ponto anterior, então você não precisa fazer nada; toque na marca para ver os outros e escolher. Ela fica cinza depois que você escolhe.'] },
    { key: 'dots', group: 'lists',
      en: ['Three dots on a route place', 'In the route list: "Direct to" drops every place before this one; "Remove" takes this place out.'],
      pt: ['Três pontos num ponto da rota', 'Na lista da rota: "Direto para" descarta tudo antes deste ponto; "Remover" tira este ponto.'] }
  ];

  function guideIcon(key, cls) {
    const s = h('span', { class: 'gicon' + (cls ? ' ' + cls : '') });
    if (key === 'box') s.textContent = 'KJFK > KBOS'; else s.innerHTML = ICON[key];
    return s;
  }

  function renderGuide() {
    const groups = [
      ['top', t('guideTop')], ['left', t('guideLeft')], ['right', t('guideRight')], ['lists', t('guideLists')]];
    let n = 0;
    const numbered = GUIDE.map((g) => Object.assign({}, g, { n: g.group === 'lists' ? 0 : ++n }));
    const W = 468, H = 696;
    const mock = h('div', { class: 'mock' },
      h('div', { class: 'mockband' }),
      ...numbered.filter((g) => g.n).map((g) => {
        const b = h('div', { class: 'mockbtn' + (g.key === 'box' ? ' mockbox' : '') + (g.group === 'right' || g.group === 'left' ? ' mockside' : ''),
          style: `left:${g.x / W * 100}%;top:${g.y / H * 100}%;width:${g.w / W * 100}%;height:${g.h / H * 100}%` });
        b.appendChild(guideIcon(g.key, 'mockicon'));
        b.appendChild(h('span', { class: 'mocknum' }, String(g.n)));
        return b;
      }),
      h('div', { class: 'mocknote' }, 'Open Charts'));
    const list = h('div', null, groups.map(([id, title]) => h('div', { class: 'card' },
      h('h3', null, title),
      ...numbered.filter((g) => g.group === id).map((g) => h('div', { class: 'gitem' },
        g.n ? h('span', { class: 'gnum' }, String(g.n)) : h('span', { class: 'gnum none' }),
        guideIcon(g.key, 'gchip' + (g.key === 'box' ? ' gbox' : '')),
        h('div', { class: 'gtext' }, h('strong', null, g[lang][0]), h('div', { class: 'muted' }, g[lang][1])))))));
    view.replaceChildren(h('h2', null, t('guide')),
      h('p', { class: 'muted' }, t('guideIntro')),
      h('div', { class: 'guidepage' }, h('div', { class: 'mockwrap' }, mock), list));
  }

  function renderAbout() {
    view.replaceChildren(h('h2', null, t('about')),
      h('div', { class: 'card' },
        h('h3', null, 'Open Charts ' + VERSION),
        h('p', null, t('aboutWhat')),
        h('p', { class: 'warnbox' }, t('disclaimer')),
        h('p', null, t('aboutNotAffil')),
        h('p', null, t('aboutData'))),
      h('div', { class: 'card' },
        h('h3', null, t('aboutProject')),
        h('p', null, t('aboutBeta')),
        h('div', { class: 'row' },
          h('a', { class: 'btn', href: REPO, target: '_blank', rel: 'noopener noreferrer' }, t('aboutRepo')),
          h('a', { class: 'btn', href: REPO + '/issues', target: '_blank', rel: 'noopener noreferrer' }, t('aboutIssues')),
          h('a', { class: 'btn primary', href: REPO + '/issues/new?template=bug_report.yml', target: '_blank', rel: 'noopener noreferrer' }, t('aboutBug')),
          h('a', { class: 'btn', href: REPO + '/issues/new?template=feature_request.yml', target: '_blank', rel: 'noopener noreferrer' }, t('aboutFeature'))),
        h('p', { class: 'muted', style: 'margin-top:10px' }, t('aboutLogs') + ': ', h('code', null, '%LOCALAPPDATA%\\OpenCharts\\opencharts.log'))),
      h('div', { class: 'card' },
        h('h3', null, t('aboutCredits')),
        h('p', null, t('aboutMap')),
        h('p', null, t('aboutLibs')),
        h('p', null, t('aboutLicense'))));
  }

  function drawFooter() {
    document.getElementById('footer').replaceChildren(
      h('div', null, t('disclaimer')),
      h('div', { class: 'muted' }, 'Open Charts ' + VERSION + '  ·  ',
        h('a', { href: '#about' }, t('about'))));
  }

  // ---- shell
  const langSel = document.getElementById('lang');
  langSel.value = lang;
  langSel.addEventListener('change', () => {
    lang = langSel.value;
    try { localStorage.setItem('oc.lang', lang); } catch (e) { /* ignore */ }
    document.documentElement.lang = lang;
    render();
  });
  document.documentElement.lang = lang;
  window.addEventListener('hashchange', render);
  render();
  api('GET', '/api/version').then((v) => {
    if (!v || !v.display) return;
    VERSION = v.display + ' Open Beta';
    drawFooter();
  }).catch(() => { /* keep the built-in label */ });
})();
