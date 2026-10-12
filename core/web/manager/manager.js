/* Open Charts manager. Plain JS, no build step. Only textContent is used for data (no innerHTML). */
(function () {
  'use strict';

  const STR = {
    en: {
      library: 'Library', sources: 'Sources', flights: 'Flights', back: '< Sources',
      flight: 'Flight', newFlight: 'New flight', flightName: 'Flight name',
      search: 'Search by ICAO, type or name...', all: 'All', charts: 'charts',
      addToFlight: '+ Add to flight', inFlight: 'In flight', addSelected: 'Add selected to flight',
      removeFromFlight: 'Remove from this flight', confirmDel: 'Delete this chart from the library?', newName: 'New name',
      rename: 'Rename', del: 'Delete', chartOnly: 'Chart only', onMap: 'On map',
      thisFlight: 'Charts in this flight', noPins: 'Nothing here yet. Use "Add to flight" in the library.',
      noCharts: 'No charts yet. Open Sources to add some.', addCharts: 'Add charts (Sources)',
      current: 'Current', makeCurrent: 'Use this flight', pinnedN: 'pinned',
      dropPdf: 'Drop here the PDFs you downloaded from this source (several at once). Known files are named and imported automatically. Or', choose: 'choose files',
      inbox: 'Inbox folder', inboxHelp: 'Files placed in this folder are picked up when you click the button.',
      importInbox: 'Import PDFs from the inbox', noInbox: 'The inbox is empty.',
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
      library: 'Biblioteca', sources: 'Fontes', flights: 'Voos', back: '< Fontes',
      flight: 'Voo', newFlight: 'Novo voo', flightName: 'Nome do voo',
      search: 'Buscar por ICAO, tipo ou nome...', all: 'Todas', charts: 'cartas',
      addToFlight: '+ Adicionar ao voo', inFlight: 'No voo', addSelected: 'Adicionar selecionadas ao voo',
      removeFromFlight: 'Tirar deste voo', confirmDel: 'Excluir esta carta da biblioteca?', newName: 'Novo nome',
      rename: 'Renomear', del: 'Excluir', chartOnly: 'Só a carta', onMap: 'No mapa',
      thisFlight: 'Cartas deste voo', noPins: 'Nada aqui ainda. Use "Adicionar ao voo" na biblioteca.',
      noCharts: 'Nenhuma carta ainda. Abra Fontes para adicionar.', addCharts: 'Adicionar cartas (Fontes)',
      current: 'Atual', makeCurrent: 'Usar este voo', pinnedN: 'fixadas',
      dropPdf: 'Solte aqui os PDFs que você baixou desta fonte (vários de uma vez). Os arquivos conhecidos são nomeados e importados automaticamente. Ou', choose: 'escolha arquivos',
      inbox: 'Pasta de entrada', inboxHelp: 'Arquivos colocados nesta pasta são importados quando você clicar no botão.',
      importInbox: 'Importar PDFs da pasta de entrada', noInbox: 'A pasta de entrada está vazia.',
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
  const VERSION = '0.1.1 Open Beta';
  const REPO = 'https://github.com/RCCSilva/msfs-efb-open-charts';
  const t = (k) => STR[lang][k] || STR.en[k] || k;
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
        (j.current ? '  ·  ' + j.current : '')),
      h('div', null, j.items.filter((i) => i.status === 'error').map((i) => h('div', { class: 'badge bad' }, i.file + ': ' + i.message))));
  }

  // ---- state and routing
  const state = { flights: { current: '', flights: [] } };
  const view = document.getElementById('view');
  const currentFlight = () => state.flights.flights.find((f) => f.id === state.flights.current) || { id: '', name: '-', pins: [] };

  async function loadFlights() { state.flights = await api('GET', '/api/flights'); }

  function parseRoute() {
    const [name, arg] = (location.hash || '#library').slice(1).split('/');
    return { name: name || 'library', arg: arg ? decodeURIComponent(arg) : '' };
  }
  const go = (hash) => { if (location.hash === hash) render(); else location.hash = hash; };

  async function render() {
    const r = parseRoute();
    const tab = r.name === 'source' ? 'sources' : r.name;
    document.getElementById('tabs').replaceChildren(...['library', 'sources', 'flights', 'setup', 'about'].map((id) =>
      h('button', { class: id === tab ? 'on' : '', onclick: () => go('#' + id) }, t(id))).concat([
        // The map (same page the EFB shows) opens in its own tab, outside the simulator too.
        h('a', { class: 'navlink', href: '/viewer.html?lang=' + lang, target: '_blank', rel: 'noopener' }, t('map') + ' \u2197')]));
    try {
      await loadFlights();
      drawFlightBar();
      drawFooter();
      if (r.name === 'sources') await renderSources();
      else if (r.name === 'source') await renderSource(r.arg);
      else if (r.name === 'flights') await renderFlights();
      else if (r.name === 'setup') await renderSetup();
      else if (r.name === 'about') renderAbout();
      else await renderLibrary();
    } catch (e) {
      view.replaceChildren(h('div', { class: 'card' }, h('span', { class: 'badge bad' }, e.message)));
    }
  }

  // ---- flight bar: the flight everything refers to (also the one the viewer uses)
  function drawFlightBar() {
    const sel = h('select', null, state.flights.flights.map((f) => h('option', { value: f.id }, `${f.name} (${f.pins.length})`)));
    sel.value = state.flights.current;
    sel.addEventListener('change', async () => {
      await api('PUT', '/api/flights/current', { id: sel.value });
      render();
    });
    document.getElementById('flightbar').replaceChildren(
      h('span', { class: 'lbl' }, t('flight') + ':'), sel,
      h('button', { class: 'btn sm', onclick: async () => {
        const n = prompt(t('flightName'), `${t('flight')} ${state.flights.flights.length + 1}`);
        if (n && n.trim()) { await api('POST', '/api/flights', { name: n.trim() }); render(); }
      } }, '+ ' + t('newFlight')));
  }

  // ---- Library
  async function renderLibrary() {
    const flight = currentFlight();
    const pins = new Set(flight.pins);
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
      await api(on ? 'PUT' : 'DELETE', `/api/flights/${encodeURIComponent(flight.id)}/pins/${encodeURIComponent(id)}`);
      on ? pins.add(id) : pins.delete(id);
    }

    function drawSide() {
      const list = flight.pins.filter((id) => pins.has(id)).concat([...pins].filter((id) => !flight.pins.includes(id)));
      side.replaceChildren(
        h('h3', null, `${t('thisFlight')} (${pins.size})`),
        h('div', { class: 'muted', style: 'margin-bottom:6px' }, flight.name),
        ...(list.length ? list.map((id) => {
          const c = byId.get(id);
          return h('div', { class: 'item' },
            h('span', { class: 't', title: c ? c.name : id }, c ? c.name : id),
            h('button', { class: 'btn sm', title: t('removeFromFlight'), onclick: async () => { await setPin(id, false); drawAll(); } }, '✕'));
        }) : [h('div', { class: 'muted' }, t('noPins'))]));
    }

    function drawBulk() {
      bulk.hidden = selected.size === 0;
      bulk.textContent = `${t('addSelected')} (${selected.size})`;
    }

    function row(c) {
      const inFlight = pins.has(c.id);
      const check = h('input', { type: 'checkbox', checked: selected.has(c.id), onchange: (e) => {
        e.target.checked ? selected.add(c.id) : selected.delete(c.id);
        drawBulk();
      } });
      return h('tr', null,
        h('td', null, check),
        h('td', null, h('strong', null, c.name), h('div', { class: 'muted' }, `${c.validity || ''}  ·  ${c.source}`)),
        h('td', { class: 'actions' },
          h('button', { class: 'btn sm' + (inFlight ? ' in' : ''), title: inFlight ? t('removeFromFlight') : '', onclick: async () => {
            await setPin(c.id, !inFlight);
            drawAll();
          } }, inFlight ? '✓ ' + t('inFlight') : t('addToFlight')), ' ',
          h('details', { class: 'more' }, h('summary', { class: 'btn sm' }, '⋯'),
            h('div', { class: 'menu' },
              h('a', { href: '/viewer.html?chart=' + encodeURIComponent(c.id) + '&mode=chart&lang=' + lang, target: '_blank' }, t('chartOnly')),
              h('a', { href: '/viewer.html?chart=' + encodeURIComponent(c.id) + '&mode=overlay&lang=' + lang, target: '_blank' }, t('onMap')),
              h('button', { onclick: async () => {
                const n = prompt(t('newName'), c.name);
                if (n && n.trim()) { await api('PATCH', '/api/charts/' + encodeURIComponent(c.id), { name: n.trim() }); render(); }
              } }, t('rename')),
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
      drawFlightBar();
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
    const inbox = await api('GET', '/api/inbox');
    const sections = [];
    const content = h('div');

    const head = h('div', { class: 'card' },
      h('a', { class: 'back', href: '#sources' }, t('back')),
      h('div', { class: 'row' },
        h('h2', { class: 'grow', style: 'margin:0' }, tx(s.name)),
        s.builtin ? h('span', { class: 'badge' }, t('builtin')) : null,
        !s.policy.automatedFetch && !s.builtin ? h('span', { class: 'badge warn' }, t('noAuto')) : null,
        s.policy.url ? h('a', { href: s.policy.url, target: '_blank', rel: 'noreferrer' }, t('policyLink')) : null,
        s.homepage ? h('a', { href: s.homepage, target: '_blank', rel: 'noreferrer' }, t('official')) : null),
      h('div', { class: 'muted' }, tx(s.description)),
      h('div', { class: 'muted' }, tx(s.policy.notes)),
      s.guide && s.guide.length ? h('div', null, h('h3', { style: 'margin-top:12px' }, t('howTo')),
        h('ol', { class: 'steps' }, s.guide.map((g) => h('li', null, tx(g))))) : null);

    if (caps.includes('catalog')) sections.push(['catalog', t('tabCatalog'), () => catalogPanel(s)]);
    if (caps.includes('package')) sections.push(['package', t('tabPackage'), () => packagePanel(s, inbox, true)]);
    const filesCard = filesPanel(s, inbox);

    let active = sections.length ? sections[0][0] : '';
    const seg = h('div', { class: 'seg' });
    async function show(key) {
      active = key;
      seg.replaceChildren(...sections.map(([k, label]) => h('button', { class: k === active ? 'on' : '', onclick: () => show(k) }, label)));
      content.replaceChildren(await sections.find((x) => x[0] === key)[2]());
    }
    if (!sections.length) { view.replaceChildren(head, filesCard); return; }
    view.replaceChildren(head, filesCard, seg, content);
    await show(active);
  }

  // Upload PDFs on behalf of a source (recognition and naming use that source's rules).
  async function uploadFiles(source, files, log, extra) {
    for (const f of files) {
      const line = h('div', null, f.name + ' ...');
      log.appendChild(line);
      const fd = new FormData();
      fd.append('file', f);
      fd.append('source', source);
      for (const key of Object.keys(extra || {})) if (extra[key]) fd.append(key, extra[key]);
      try {
        const c = await api('POST', '/api/import', fd);
        line.replaceChildren(h('span', { class: 'badge ok' }, t('imported')), c.name);
      } catch (e) {
        line.replaceChildren(h('span', { class: 'badge bad' }, t('error')), f.name + ': ' + e.message);
      }
    }
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
      try {
        const c = await api('POST', '/api/import', fd);
        card.replaceChildren(h('span', { class: 'badge ok' }, t('imported')), c.name);
        card.done = true;
      } catch (e) {
        status.replaceChildren(h('span', { class: 'badge bad' }, e.message));
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

  function filesPanel(s, inbox) {
    const reviews = h('div');
    const drop = h('div', { class: 'drop' });
    const file = h('input', { type: 'file', accept: '.pdf,application/pdf', multiple: true, hidden: true });
    drop.append(t('dropPdf') + ' ', h('a', { href: '#', onclick: (e) => { e.preventDefault(); file.click(); } }, t('choose')));
    const cards = [];

    async function inspectFiles(files) {
      for (const f of files) {
        const fd = new FormData();
        fd.append('file', f);
        fd.append('source', s.id);
        try {
          const ins = await api('POST', '/api/import/inspect', fd);
          if (ins.action === 'import') { // the server decided: import as it identified the file
            const line = h('div', { class: 'card' }, f.name + ' ...');
            reviews.appendChild(line);
            const body = new FormData();
            body.append('token', ins.token);
            body.append('source', s.id);
            try {
              const c = await api('POST', '/api/import', body);
              line.replaceChildren(h('span', { class: 'badge ok' }, t('imported')), c.name);
            } catch (e) {
              line.replaceChildren(h('span', { class: 'badge bad' }, t('error')), f.name + ': ' + e.message);
            }
            continue;
          }
          const c = reviewCard(s, ins);
          cards.push(c);
          reviews.appendChild(c);
        } catch (e) {
          reviews.appendChild(h('div', { class: 'card' }, h('span', { class: 'badge bad' }, t('error')), f.name + ': ' + e.message));
        }
      }
      if (cards.length > 1 && !reviews.querySelector('.importall')) {
        reviews.prepend(h('div', { class: 'row importall' },
          h('button', { class: 'btn primary', onclick: () => cards.forEach((c) => c.importNow()) }, t('importAll'))));
      }
    }
    file.addEventListener('change', () => inspectFiles([...file.files]));
    drop.addEventListener('dragover', (e) => { e.preventDefault(); drop.classList.add('over'); });
    drop.addEventListener('dragleave', () => drop.classList.remove('over'));
    drop.addEventListener('drop', (e) => { e.preventDefault(); drop.classList.remove('over'); inspectFiles([...e.dataTransfer.files]); });

    const prog = h('div');
    const pdfs = inbox.files.filter((f) => f.kind === 'pdf');
    const scan = h('button', { class: 'btn', disabled: pdfs.length === 0, onclick: async () => {
      const r = await api('POST', '/api/inbox/scan?source=' + encodeURIComponent(s.id));
      if (r.job) await waitJob(r.job.id, (j) => prog.replaceChildren(progress(j)));
      toast(t('done'));
    } }, t('importInbox'));

    return h('div', null,
      h('div', { class: 'card' }, h('h3', null, t('stepsFiles')), drop, file),
      reviews,
      h('div', { class: 'card' },
        h('h3', null, t('inbox')),
        h('div', { class: 'muted' }, inbox.dir),
        h('div', { class: 'muted' }, t('inboxHelp')),
        h('div', null, pdfs.length ? pdfs.map((f) => h('div', null, f.name)) : h('div', { class: 'muted' }, t('noInbox'))),
        h('div', { class: 'row', style: 'margin-top:8px' }, scan), prog));
  }

  function catalogPanel(s) {
    const q = h('input', { type: 'text', placeholder: t('search') });
    const body = h('tbody');
    const info = h('span', { class: 'muted' });
    async function load() {
      const r = await api('GET', `/api/sources/${s.id}/catalog?limit=40&q=${encodeURIComponent(q.value)}`);
      info.textContent = `${r.total} · ${t('snapshot')} ${r.snapshot}`;
      body.replaceChildren(...r.items.map((it) => {
        const log = h('span', { class: 'muted' });
        const picker = h('input', { type: 'file', accept: '.pdf,application/pdf', hidden: true });
        picker.addEventListener('change', async () => {
          await uploadFiles(s.id, [...picker.files], log, { type: it.type, title: it.name, validity: it.effective });
          load();
        });
        return h('tr', null,
          h('td', null, h('span', { class: 'badge' }, it.type)),
          h('td', null, it.name, h('div', { class: 'muted' }, `${it.number} · ${it.effective} · ${it.file}`), log),
          h('td', { class: 'actions' },
            it.installed ? h('span', { class: 'badge ok' }, '✓ ' + t('installed')) : [
              h('a', { class: 'btn sm', href: it.url, target: '_blank', rel: 'noreferrer' }, t('openOfficial')), ' ',
              h('button', { class: 'btn sm primary', onclick: () => picker.click() }, t('addFile')), picker]));
      }));
    }
    let timer = null;
    q.addEventListener('input', () => { clearTimeout(timer); timer = setTimeout(load, 200); });
    load();
    return h('div', { class: 'card' }, h('h3', null, t('catalog')), h('div', { class: 'muted' }, t('catalogHelp')),
      h('div', { class: 'row', style: 'margin-top:8px' }, h('div', { class: 'grow' }, q), info), h('table', null, body));
  }

  function packagePanel(s, inbox) {
    const path = h('input', { type: 'text', placeholder: t('pkgPath') });
    const zips = inbox.files.filter((f) => f.kind === 'zip');
    const pick = h('select', null, h('option', { value: '' }, '-'), zips.map((z) => h('option', { value: z.path }, z.name)));
    pick.addEventListener('change', () => { if (pick.value) path.value = pick.value; });
    const out = h('div');
    const prog = h('div');
    const locBox = h('div');

    if ((s.capabilities || []).includes('locate')) {
      api('GET', `/api/sources/${s.id}/locate`).then((loc) => {
        locBox.replaceChildren(h('h3', null, t('locate')), h('div', { class: 'muted' }, t('locateHelp')),
          ...loc.locations.map((l) => h('div', { class: 'row' },
            h('a', { href: l.url, target: '_blank', rel: 'noreferrer' }, tx(l.label)),
            h('span', { class: 'muted' }, l.file),
            l.verified ? null : h('span', { class: 'badge warn' }, t('unverified')))));
      });
    }

    async function read() {
      out.replaceChildren();
      try {
        const p = await api('GET', `/api/sources/${s.id}/package?path=${encodeURIComponent(path.value.trim())}`);
        const filter = h('input', { type: 'text', placeholder: 'ICAO' });
        const body = h('tbody');
        const draw = () => {
          const f = filter.value.trim().toUpperCase();
          body.replaceChildren(...p.airports.filter((a) => !f || a.icao.startsWith(f)).slice(0, 40).map((a) => h('tr', null,
            h('td', null, h('strong', null, a.icao)),
            h('td', { class: 'muted' }, Object.keys(a.types).sort().map((k) => `${k} ${a.types[k]}`).join(' · ')),
            h('td', { class: 'actions' }, h('button', { class: 'btn sm primary', onclick: () => importAirport(a.icao) }, `${t('importAirport')} (${a.count})`)))));
        };
        filter.addEventListener('input', draw);
        draw();
        out.replaceChildren(
          h('div', { class: 'muted' }, `${p.charts} ${t('charts')} · ${p.airports.length} ${t('airports')}${p.date ? ' · ' + p.date : ''}`),
          h('div', { class: 'row' }, h('div', { class: 'grow' }, filter)), h('table', null, body));
      } catch (e) {
        out.replaceChildren(h('span', { class: 'badge bad' }, e.message));
      }
    }

    async function importAirport(icao) {
      try {
        const j = await api('POST', `/api/sources/${s.id}/package/import`, { path: path.value.trim(), icao });
        await waitJob(j.id, (x) => prog.replaceChildren(progress(x)));
        toast(t('done'));
      } catch (e) {
        prog.replaceChildren(h('span', { class: 'badge bad' }, e.message));
      }
    }

    return h('div', { class: 'card' }, locBox, h('h3', { style: 'margin-top:14px' }, t('package')),
      h('div', { class: 'row' }, h('div', { class: 'grow' }, path), pick,
        h('button', { class: 'btn', onclick: read }, t('readPackage'))), out, prog);
  }

  // ---- Flights: all flights and what is pinned in each
  async function renderFlights() {
    const all = await api('GET', '/api/charts?limit=1000');
    const byId = new Map(all.map((c) => [c.id, c]));
    const cards = state.flights.flights.map((f) => h('div', { class: 'card' },
      h('div', { class: 'row' },
        h('h3', { class: 'grow' }, `${f.name} (${f.pins.length} ${t('pinnedN')})`),
        f.id === state.flights.current ? h('span', { class: 'badge ok' }, t('current')) :
          h('button', { class: 'btn sm', onclick: async () => { await api('PUT', '/api/flights/current', { id: f.id }); render(); } }, t('makeCurrent')),
        h('button', { class: 'btn sm', onclick: async () => {
          const n = prompt(t('newName'), f.name);
          if (n && n.trim()) { await api('PATCH', '/api/flights/' + encodeURIComponent(f.id), { name: n.trim() }); render(); }
        } }, t('rename')),
        h('button', { class: 'btn sm danger', disabled: state.flights.flights.length < 2, onclick: async () => {
          if (confirm(t('confirmDel'))) { await api('DELETE', '/api/flights/' + encodeURIComponent(f.id)); render(); }
        } }, t('del'))),
      f.pins.length ? f.pins.map((id) => {
        const c = byId.get(id);
        return h('div', { class: 'row' }, h('span', { class: 'grow' }, c ? c.name : id),
          h('button', { class: 'btn sm', onclick: async () => {
            await api('DELETE', `/api/flights/${encodeURIComponent(f.id)}/pins/${encodeURIComponent(id)}`);
            render();
          } }, '✕'));
      }) : h('div', { class: 'muted' }, t('noPins'))));
    view.replaceChildren(h('h2', null, t('flights')), ...cards);
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

    async function act(fn, done) {
      try { await fn(); toast(done || t('done')); } catch (e) { toast(e.message); }
      renderSetup();
    }

    for (const inst of st.installs) {
      const efb = inst.efb;
      const efbText = !efb.installed ? warn(efb.legacy ? t('stEfbLegacy') : t('stEfbNo')) :
        efb.upToDate ? ok(t('stEfbOk') + ' (' + efb.version + ')') :
          warn(t('stEfbOld') + ' (' + efb.version + ' \u2192 ' + st.efbVersion + ')');
      const efbBtns = h('span', null,
        h('button', { class: 'btn sm primary', onclick: () => act(() => api('POST', '/api/setup/efb', { community: inst.community }), t('stRestart')) },
          efb.installed ? t('btnUpdate') : t('btnInstall')), ' ',
        efb.installed ? h('button', { class: 'btn sm danger', onclick: () => act(() => api('DELETE', '/api/setup/efb?community=' + encodeURIComponent(inst.community))) }, t('btnRemove')) : null);
      cards.push(h('div', { class: 'card' },
        h('h3', null, t('stInstall') + ' \u2014 ' + (inst.edition === 'store' ? t('edStore') : t('edSteam'))),
        h('div', { class: 'muted', style: 'margin-bottom:6px' }, inst.community),
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
})();
