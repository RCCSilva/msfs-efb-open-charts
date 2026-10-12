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
      pkgEta: 'left', pkgDuration: { s: 'a few seconds', m: '{n} min', h: '{h} h {m} min' },
      inbox: 'Inbox folder', inboxHelp: 'What you add here stays in this folder, on this computer. PDFs are imported when you click the button; for a .zip, read it and choose the aerodromes.',
      importInbox: 'Import the PDFs of the inbox', noInbox: 'The inbox is empty.',
      imported: 'imported', skipped: 'skipped', failed: 'failed', done: 'Done', error: 'Error',
      builtin: 'Built in', noAuto: 'No automatic download', policyLink: 'Usage policy', official: 'Official page',
      howTo: 'How to get charts from this source', locate: 'Where to get the package',
      locateHelp: 'Addresses derived from the file naming; they open in your browser and you download the file yourself.',
      unverified: 'not confirmed', package: 'Downloaded package', pkgPath: 'Path to the .zip file (or pick one from the inbox)',
      readPackage: 'Read package', airports: 'airports', importAirport: 'Import',
      catalog: 'Catalog', catalogHelp: 'Open the official page, save the PDF, then use "Add file" on the same row.',
      openOfficial: '1. Open official page', addFile: '2. Add file', installed: 'Installed', snapshot: 'Snapshot of',
      tabCatalog: 'Visual charts (catalog)', tabPackage: 'Aerodromes (package)', tabFiles: 'Add files',
      stepsFiles: 'Add what you downloaded', capCatalog: 'Catalog', capPackage: 'Package', capLocate: 'By date', capUpload: 'Upload',
      about: 'About', setup: 'Setup', map: 'Map',
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
      reviewTitle: 'Review before importing', icao: 'ICAO', ctype: 'Type', ctitle: 'Title', validity: 'Valid from', cname: 'Display name',
      importOne: 'Import', importAll: 'Import all', notGeo: 'This PDF has no georeferencing and cannot be used.', dup: 'Already in the library as',
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
      pkgEta: 'restantes', pkgDuration: { s: 'alguns segundos', m: '{n} min', h: '{h} h {m} min' },
      inbox: 'Pasta de entrada', inboxHelp: 'O que você adiciona aqui fica nesta pasta, no seu computador. Os PDFs são importados quando você clicar no botão; de um .zip, leia o pacote e escolha os aeródromos.',
      importInbox: 'Importar os PDFs da pasta de entrada', noInbox: 'A pasta de entrada está vazia.',
      imported: 'importadas', skipped: 'ignoradas', failed: 'com erro', done: 'Concluído', error: 'Erro',
      builtin: 'Embutida', noAuto: 'Sem download automático', policyLink: 'Política de uso', official: 'Página oficial',
      howTo: 'Como obter cartas desta fonte', locate: 'Onde obter o pacote',
      locateHelp: 'Endereços derivados do padrão de nome; abrem no seu navegador e você baixa o arquivo.',
      unverified: 'não confirmado', package: 'Pacote baixado', pkgPath: 'Caminho do arquivo .zip (ou escolha um da pasta de entrada)',
      readPackage: 'Ler pacote', airports: 'aeródromos', importAirport: 'Importar',
      catalog: 'Catálogo', catalogHelp: 'Abra a página oficial, salve o PDF e use "Adicionar arquivo" na mesma linha.',
      openOfficial: '1. Abrir página oficial', addFile: '2. Adicionar arquivo', installed: 'Instalada', snapshot: 'Retrato de',
      tabCatalog: 'Cartas visuais (catálogo)', tabPackage: 'Aeródromos (pacote)', tabFiles: 'Adicionar arquivos',
      stepsFiles: 'Adicione o que você baixou', capCatalog: 'Catálogo', capPackage: 'Pacote', capLocate: 'Por data', capUpload: 'Envio',
      about: 'Sobre', setup: 'Configuração', map: 'Mapa',
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
      reviewTitle: 'Revise antes de importar', icao: 'ICAO', ctype: 'Tipo', ctitle: 'Título', validity: 'Válida desde', cname: 'Nome exibido',
      importOne: 'Importar', importAll: 'Importar todas', notGeo: 'Este PDF não tem georreferenciamento e não pode ser usado.', dup: 'Já está na biblioteca como',
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
  let VERSION = '0.2.0 Open Beta';   // replaced by what the running program reports (/api/version): '0.1.1-8fa9b5b' while developing
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
    for (const kid of kids.flat()) {
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

  async function render() {
    const r = parseRoute();
    const tab = r.name === 'source' ? 'sources' : r.name;
    if (state.local === undefined) {
      try { state.local = (await api('GET', '/api/health')).local !== false; } catch (e) { state.local = true; }
    }
    document.getElementById('tabs').replaceChildren(...['library', 'sources', 'groups', 'setup', 'about'].filter((id) => id !== 'setup' || state.local).map((id) =>
      h('button', { class: id === tab ? 'on' : '', onclick: () => go('#' + id) }, t(id))).concat([
        // The map (same page the EFB shows) opens in its own tab, outside the simulator too.
        h('a', { class: 'navlink', href: '/viewer.html?lang=' + lang, target: '_blank', rel: 'noopener' }, t('map') + ' \u2197')]));
    try {
      await loadGroups();
      drawGroupBar();
      drawFooter();
      if (r.name === 'sources') await renderSources();
      else if (r.name === 'source') await renderSource(r.arg);
      else if (r.name === 'groups') await renderGroups();
      else if (r.name === 'setup') await renderSetup();
      else if (r.name === 'about') renderAbout();
      else await renderLibrary();
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

  // ---- Edit a chart's properties. The server validates and generates the name; this only collects.
  function editChart(c) {
    const field = (label, val, ph) => {
      const input = h('input', { type: 'text', value: val || '', placeholder: ph || '' });
      return [h('label', { class: 'muted' }, label), input];
    };
    const [lI, icao] = field(t('icao'), c.icao, 'SBSP');
    const [lT, ctype] = field(t('ctype'), c.type, 'IAC');
    const [lTi, title] = field(t('ctitle'), c.title);
    const [lV, validity] = field(t('validity'), c.validity, 'YYYY-MM-DD');
    const err = h('div');
    const close = () => overlay.remove();
    const save = h('button', { class: 'btn primary', onclick: async () => {
      save.disabled = true;
      err.replaceChildren();
      try {
        await api('PATCH', '/api/charts/' + encodeURIComponent(c.id), {
          icao: icao.value, type: ctype.value, title: title.value, validity: validity.value
        });
        close();
        toast(t('saved'));
        render();
      } catch (e) {
        err.replaceChildren(h('span', { class: 'badge bad' }, e.message));
        save.disabled = false;
      }
    } }, t('save'));
    const overlay = h('div', { class: 'modal', onclick: (e) => { if (e.target === overlay) close(); } },
      h('div', { class: 'card modalbox' },
        h('h3', null, t('editChart')),
        h('div', { class: 'muted', style: 'margin-bottom:8px' }, c.name),
        h('div', { class: 'formgrid' }, lI, icao, lT, ctype, lTi, title, lV, validity),
        h('div', { class: 'muted' }, t('editHelp')),
        h('div', { class: 'muted' }, t('editWarnSource')),
        err,
        h('div', { class: 'row end', style: 'margin-top:12px' },
          h('button', { class: 'btn', onclick: close }, t('cancel')), save)));
    document.body.appendChild(overlay);
    icao.focus();
  }

  // ---- Library
  async function renderLibrary() {
    const group = currentGroup();
    const pins = new Set(group.pins);
    const all = await api('GET', '/api/charts?limit=1000');
    const byId = new Map(all.map((c) => [c.id, c]));
    const selected = new Set();
    const q = h('input', { type: 'text', placeholder: t('search') });
    const type = h('select', null, h('option', { value: '' }, t('all')),
      [...new Set(all.map((c) => c.type).filter(Boolean))].sort().map((x) => h('option', { value: x }, x)));
    const body = h('tbody');
    const count = h('span', { class: 'muted' });
    const bulk = h('button', { class: 'btn primary sm', hidden: true });
    const side = h('div', { class: 'card side' });

    async function setPin(id, on) {
      await api(on ? 'PUT' : 'DELETE', `/api/groups/${encodeURIComponent(group.id)}/pins/${encodeURIComponent(id)}`);
      on ? pins.add(id) : pins.delete(id);
    }

    function drawSide() {
      const list = group.pins.filter((id) => pins.has(id)).concat([...pins].filter((id) => !group.pins.includes(id)));
      side.replaceChildren(
        h('h3', null, `${t('thisGroup')} (${pins.size})`),
        h('div', { class: 'muted', style: 'margin-bottom:6px' }, group.name),
        ...(list.length ? list.map((id) => {
          const c = byId.get(id);
          return h('div', { class: 'item' },
            h('span', { class: 't', title: c ? c.name : id }, c ? c.name : id),
            h('button', { class: 'btn sm', title: t('removeFromGroup'), onclick: async () => { await setPin(id, false); drawAll(); } }, '✕'));
        }) : [h('div', { class: 'muted' }, t('noPins'))]));
    }

    function drawBulk() {
      bulk.hidden = selected.size === 0;
      bulk.textContent = `${t('addSelected')} (${selected.size})`;
    }

    function row(c) {
      const inGroup = pins.has(c.id);
      const check = h('input', { type: 'checkbox', checked: selected.has(c.id), onchange: (e) => {
        e.target.checked ? selected.add(c.id) : selected.delete(c.id);
        drawBulk();
      } });
      return h('tr', null,
        h('td', null, check),
        h('td', null, h('strong', null, c.name), h('div', { class: 'muted' }, `${c.validity || ''}  ·  ${c.source}`)),
        h('td', { class: 'actions' },
          h('button', { class: 'btn sm' + (inGroup ? ' in' : ''), title: inGroup ? t('removeFromGroup') : '', onclick: async () => {
            await setPin(c.id, !inGroup);
            drawAll();
          } }, inGroup ? '✓ ' + t('inGroup') : t('addToGroup')), ' ',
          h('details', { class: 'more' }, h('summary', { class: 'btn sm' }, '⋯'),
            h('div', { class: 'menu' },
              h('a', { href: '/viewer.html?chart=' + encodeURIComponent(c.id) + '&mode=chart&lang=' + lang, target: '_blank' }, t('chartOnly')),
              h('a', { href: '/viewer.html?chart=' + encodeURIComponent(c.id) + '&mode=overlay&lang=' + lang, target: '_blank' }, t('onMap')),
              h('button', { onclick: () => editChart(c) }, t('editChart')),
              h('button', { class: 'danger', onclick: async () => {
                if (confirm(t('confirmDel'))) { await api('DELETE', '/api/charts/' + encodeURIComponent(c.id)); render(); }
              } }, t('del'))))));
    }

    let shown = all;
    async function load() {
      shown = await api('GET', `/api/charts?limit=500&q=${encodeURIComponent(q.value)}&type=${encodeURIComponent(type.value)}`);
      drawAll();
    }
    function drawAll() {
      count.textContent = `${shown.length} ${t('charts')}`;
      body.replaceChildren(...(shown.length ? shown.map(row) :
        [h('tr', null, h('td', { colspan: 3, class: 'muted' }, t('noCharts')))]));
      drawSide();
      drawBulk();
    }

    bulk.addEventListener('click', async () => {
      for (const id of selected) if (!pins.has(id)) await setPin(id, true);
      selected.clear();
      drawAll();
      drawGroupBar();
      toast(t('done'));
    });
    let timer = null;
    q.addEventListener('input', () => { clearTimeout(timer); timer = setTimeout(load, 150); });
    type.addEventListener('change', load);

    view.replaceChildren(
      h('div', { class: 'row' }, h('h2', { class: 'grow' }, t('library')),
        h('a', { class: 'btn', href: '#sources' }, t('addCharts'))),
      h('div', { class: 'split' },
        h('div', { class: 'card' },
          h('div', { class: 'row' }, h('div', { class: 'grow' }, q), type, count, bulk),
          h('table', null, body)),
        side));
    shown = all;
    drawAll();
  }

  // ---- Sources: list, then one page per source
  async function renderSources() {
    const list = await api('GET', '/api/sources');
    const capName = { catalog: 'capCatalog', package: 'capPackage', locate: 'capLocate' };
    view.replaceChildren(h('h2', null, t('sources')),
      h('div', { class: 'cols' }, ...list.map((s) => h('button', { class: 'card src', onclick: () => go('#source/' + encodeURIComponent(s.id)) },
        h('h3', null, tx(s.name)),
        h('div', { class: 'muted' }, tx(s.description)),
        h('div', { style: 'margin-top:8px' },
          s.builtin ? h('span', { class: 'badge' }, t('builtin')) : null,
          (s.capabilities || []).map((c) => h('span', { class: 'badge' }, t(capName[c] || c))),
          !s.policy.automatedFetch && !s.builtin ? h('span', { class: 'badge warn' }, t('noAuto')) : null)))));
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
    if (s.homepage) links.appendChild(linkRow(t('linkHub'), s.homepage, t('linkHubNote')));
    // each request answers with its rows; they are added in a fixed order whatever arrives first
    const none = () => [];
    const visual = (kind) => caps.includes('catalog')
      ? api('GET', '/api/sources/' + s.id + '/catalog?limit=1&type=' + kind).then((r) =>
        r.items.map((it) => linkRow(t('exVisual') + ' ' + it.type + ': ' + it.name, it.url, it.file))).catch(none)
      : Promise.resolve([]);
    const groups = await Promise.all([visual('REA'), visual('WAC')]);
    const rows = groups.flat();
    rows.forEach((r) => links.appendChild(r));
    const linksShown = !!s.homepage || rows.length > 0;

    // 3. what the user downloaded goes here, PDFs and packages alike
    view.replaceChildren(head, ...(linksShown ? [links] : []), filesPanel(s));
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
    const [lV, validity] = field(t('validity'), sg.validity, 'YYYY-MM-DD');
    const [lN, name] = field(t('cname'), '', t('nameAuto'));
    const status = h('div');
    const btn = h('button', { class: 'btn primary', disabled: !ins.georeferenced, onclick: async () => {
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
      !ins.georeferenced ? h('div', { class: 'badge bad' }, t('notGeo')) : null,
      ins.duplicate ? h('div', { class: 'badge warn' }, t('dup') + ' ' + ins.duplicate.name) : null,
      h('div', { class: 'formgrid' }, lI, icao, lT, ctype, lTi, title, lV, validity, lN, name),
      h('div', { class: 'row' }, btn, status));
    card.importNow = () => { if (!card.done && !btn.disabled) btn.click(); };
    return card;
  }

  function filesPanel(s) {
    const reviews = h('div');
    const packages = h('div');
    const drop = h('div', { class: 'drop' });
    const file = h('input', { type: 'file', accept: '.pdf,.zip,application/pdf,application/zip', multiple: true, hidden: true });
    drop.append(t('dropPdf') + ' ', h('a', { href: '#', onclick: (e) => { e.preventDefault(); file.click(); } }, t('choose')));
    const cards = [];
    const isZip = (name) => /\.zip$/i.test(name);

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
            h('td', null, h('strong', null, a.icao)),
            h('td', { class: 'muted' }, Object.keys(a.types).sort().map((k) => `${k} ${a.types[k]}`).join(' \u00b7 ')),
            h('td', { class: 'actions' }, fresh === 0
              ? h('span', { class: 'badge ok' }, '\u2713 ' + t('pkgAlready'))
              : h('button', { class: 'btn sm primary', onclick: () => run({ icao: a.icao }) },
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
    const st = await api('GET', '/api/setup');
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

    if (!st.installs.length) {
      const path = h('input', { type: 'text', placeholder: 'C:\\...\\Packages\\Community' });
      cards.push(h('div', { class: 'card' }, h('h3', null, t('stInstall')), h('div', { class: 'muted' }, t('stNoInstall')),
        h('div', { class: 'row', style: 'margin-top:8px' }, h('div', { class: 'grow' }, path),
          h('button', { class: 'btn primary', onclick: () => act(() => api('POST', '/api/setup/efb', { community: path.value }), t('stRestart')) }, t('btnInstall')))));
    }
    view.replaceChildren(h('h2', null, t('setup')), ...cards);
  }

  // ---- About
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
