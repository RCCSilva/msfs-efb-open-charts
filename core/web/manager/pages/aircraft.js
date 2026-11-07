import { h } from '../core/dom.js';
import { api, toast } from '../core/api.js';
import { lang } from '../core/i18n.js';
import { confirmModal } from '../core/modal.js';

// The aircraft profiles: how fast each aircraft climbs, cruises and descends and what it burns, read from the files of
// Little Navmap or typed here. The numbers are the user's to change. The estimate of a route is made by the program
// (/api/route/estimate); this page only asks for it and shows it.

const AC = {
  en: {
    title: 'Aircraft', intro: 'The performance of the aircraft you fly. Import the performance files of Little Navmap (.lnmperf) or fill one in by hand, then choose it on a route to get the time and the fuel. Planning for simulation only, never for real flight.',
    mine: 'Your aircraft', none: 'No aircraft yet. Import a Little Navmap file or create one.', newAircraft: 'New aircraft', importFiles: 'Import files',
    imported: 'imported', failed: 'Could not do that', save: 'Save', del: 'Delete', confirmDel: 'Delete this aircraft? The routes that use it keep their altitude.', saved: 'Saved',
    name: 'Name', type: 'Type (ICAO)', fuelUnit: 'Fuel unit', gallons: 'Gallons', pounds: 'Pounds', jet: 'Jet fuel', runway: 'Runway', minRunway: 'Minimum runway (ft)',
    runways: { '': '-', hard: 'Hard', soft: 'Soft', water: 'Water' }, description: 'Notes',
    fuelTitle: 'Fuel', usable: 'Usable', taxi: 'Taxi', reserve: 'Reserve', extra: 'Extra', contingency: 'Contingency (% of the trip)',
    phases: 'Phases', climb: 'Climb', cruise: 'Cruise', descent: 'Descent', alternate: 'Alternate', speed: 'Speed (kt TAS)', flow: 'Fuel flow', rate: 'Rate (ft/min)',
    unitPerHour: '/h',
    // route planning
    plan: 'Flight estimate', elev: 'Airport elevations', aircraft: 'Aircraft', noAircraft: '- none -', cruiseAlt: 'Cruise altitude (ft)', wind: 'Wind (kt, - = headwind)', estimate: 'Estimate',
    needRoute: 'Validate the route to estimate it.', needChoice: 'Choose an aircraft and an altitude.', noneToChoose: 'No aircraft yet: add one in the Aircraft tab.',
    time: 'Time', distance: 'Distance', trip: 'Trip fuel', required: 'Fuel to carry', usableFuel: 'Usable', enough: 'It fits in the tanks.', notEnough: 'It does not fit: more fuel is needed than the tanks hold.',
    truncated: 'The route is too short to climb to that altitude and come down: the estimate uses', topClimb: 'Top of climb', topDescent: 'Top of descent', from: 'from the start',
    contingencyRow: 'Contingency', alternateRow: 'Alternate', reserveRow: 'Reserve', extraRow: 'Extra', taxiRow: 'Taxi', total: 'Total', weight: 'weight',
    phase: 'Phase', leg: 'Leg', nm: 'nm', min: 'min', fuel: 'Fuel', ft: 'ft', note: 'Still air unless a wind is given; constant speeds for each phase; airports taken at sea level.'
  },
  pt: {
    title: 'Aviões', intro: 'O desempenho dos aviões que você voa. Importe os arquivos de desempenho do Little Navmap (.lnmperf) ou preencha à mão e depois escolha o avião na rota para ter o tempo e o combustível. Planejamento só para simulação, nunca para voo real.',
    mine: 'Seus aviões', none: 'Nenhum avião ainda. Importe um arquivo do Little Navmap ou crie um.', newAircraft: 'Novo avião', importFiles: 'Importar arquivos',
    imported: 'importados', failed: 'Não foi possível', save: 'Salvar', del: 'Excluir', confirmDel: 'Excluir este avião? As rotas que o usam mantêm a altitude.', saved: 'Salvo',
    name: 'Nome', type: 'Tipo (ICAO)', fuelUnit: 'Unidade de combustível', gallons: 'Galões', pounds: 'Libras', jet: 'Combustível de jato', runway: 'Pista', minRunway: 'Pista mínima (ft)',
    runways: { '': '-', hard: 'Dura', soft: 'Macia', water: 'Água' }, description: 'Observações',
    fuelTitle: 'Combustível', usable: 'Utilizável', taxi: 'Táxi', reserve: 'Reserva', extra: 'Extra', contingency: 'Contingência (% da viagem)',
    phases: 'Fases', climb: 'Subida', cruise: 'Cruzeiro', descent: 'Descida', alternate: 'Alternativa', speed: 'Velocidade (kt TAS)', flow: 'Consumo', rate: 'Razão (ft/min)',
    unitPerHour: '/h',
    plan: 'Estimativa do voo', elev: 'Elevação dos aeroportos', aircraft: 'Avião', noAircraft: '- nenhum -', cruiseAlt: 'Altitude de cruzeiro (ft)', wind: 'Vento (kt, - = de proa)', estimate: 'Estimar',
    needRoute: 'Valide a rota para estimar.', needChoice: 'Escolha um avião e uma altitude.', noneToChoose: 'Nenhum avião ainda: cadastre na aba Aviões.',
    time: 'Tempo', distance: 'Distância', trip: 'Combustível da viagem', required: 'Combustível a embarcar', usableFuel: 'Utilizável', enough: 'Cabe nos tanques.', notEnough: 'Não cabe: precisa de mais combustível do que os tanques levam.',
    truncated: 'A rota é curta demais para subir até essa altitude e descer: a estimativa usa', topClimb: 'Topo de subida', topDescent: 'Início da descida', from: 'desde o início',
    contingencyRow: 'Contingência', alternateRow: 'Alternativa', reserveRow: 'Reserva', extraRow: 'Extra', taxiRow: 'Táxi', total: 'Total', weight: 'peso',
    phase: 'Fase', leg: 'Trecho', nm: 'nm', min: 'min', fuel: 'Combustível', ft: 'ft', note: 'Ar parado, a menos que haja vento; velocidades constantes em cada fase; aeroportos ao nível do mar.'
  }
};
const at = (k) => (AC[lang] && AC[lang][k]) || AC.en[k] || k;

const hm = (min) => {
  const m = Math.round(min);
  return Math.floor(m / 60) + ':' + String(m % 60).padStart(2, '0');
};
const num = (v) => (v === '' || v === null || v === undefined ? 0 : Number(v));
const fmt = (v, d = 1) => Number(v).toFixed(d).replace(/\.0+$/, '');

// ---- the page of the aircraft ----

const state = { id: 0, draft: null };

const blank = () => ({
  id: 0, name: '', aircraftType: '', fuelUnit: 'gal', jet: false, description: '', runwayType: '', minRunwayFt: 0,
  usableFuel: 0, taxiFuel: 0, reserveFuel: 0, extraFuel: 0, contingencyPct: 0,
  climb: { speedKt: 0, fuelFlow: 0, vertFpm: 0 }, cruise: { speedKt: 0, fuelFlow: 0 },
  descent: { speedKt: 0, fuelFlow: 0, vertFpm: 0 }, alternate: { speedKt: 0, fuelFlow: 0 }
});

export async function renderAircraft(view) {
  const list = (await api('GET', '/api/aircraft')).aircraft;
  if (state.id && !list.find((a) => a.id === state.id)) { state.id = 0; state.draft = null; }
  if (!state.draft) state.draft = state.id ? structuredClone(list.find((a) => a.id === state.id)) : blank();
  const d = state.draft;
  const redraw = () => renderAircraft(view);
  const pick = (a) => { state.id = a ? a.id : 0; state.draft = a ? structuredClone(a) : blank(); redraw(); };

  const field = (label, value, set, opt = {}) => {
    const input = h('input', { type: opt.text ? 'text' : 'number', step: 'any', min: opt.text ? null : '0', value: value === 0 && !opt.text ? '' : String(value ?? '') });
    input.addEventListener('input', () => set(opt.text ? input.value : num(input.value)));
    return h('label', { class: 'fld' }, h('span', { class: 'muted' }, label), input);
  };
  const unit = () => (d.fuelUnit === 'lb' ? 'lb' : 'gal');
  const flowLabel = () => at('flow') + ' (' + unit() + at('unitPerHour') + ')';

  const fuelUnit = h('select', null, h('option', { value: 'gal' }, at('gallons')), h('option', { value: 'lb' }, at('pounds')));
  fuelUnit.value = d.fuelUnit;
  fuelUnit.addEventListener('change', () => { d.fuelUnit = fuelUnit.value; redraw(); });
  const jet = h('input', { type: 'checkbox' });
  jet.checked = d.jet;
  jet.addEventListener('change', () => { d.jet = jet.checked; });
  const runway = h('select', null, Object.entries(at('runways')).map(([k, v]) => h('option', { value: k }, v)));
  runway.value = d.runwayType || '';
  runway.addEventListener('change', () => { d.runwayType = runway.value; });
  const notes = h('textarea', { rows: 3 });
  notes.value = d.description || '';
  notes.addEventListener('input', () => { d.description = notes.value; });

  const grid = (...kids) => h('div', { class: 'ac-grid' }, kids);
  const save = async () => {
    try {
      const r = d.id ? await api('PUT', '/api/aircraft/' + d.id, d) : await api('POST', '/api/aircraft', d);
      state.id = r.id;
      state.draft = null;
      toast(at('saved'));
      redraw();
    } catch (e) { toast(at('failed') + ': ' + e.message); }
  };
  const remove = async () => {
    if (!(await confirmModal({ title: at('del'), text: at('confirmDel'), ok: at('del') }))) return;
    try { await api('DELETE', '/api/aircraft/' + d.id); state.id = 0; state.draft = null; redraw(); } catch (e) { toast(at('failed') + ': ' + e.message); }
  };

  const files = h('input', { type: 'file', multiple: true, accept: '.lnmperf,.xml,.ini,.txt', hidden: true });
  files.addEventListener('change', async () => {
    let ok = 0;
    const errors = [];
    let last = null;
    for (const f of files.files) {
      try {
        const res = await fetch('/api/aircraft/import', { method: 'POST', body: await f.text() });
        const data = await res.json().catch(() => ({}));
        if (!res.ok) throw new Error(data.error || res.statusText);
        ok++;
        last = data;
      } catch (e) { errors.push(f.name + ': ' + e.message); }
    }
    toast(ok + ' ' + at('imported') + (errors.length ? ' - ' + errors.join('; ') : ''));
    if (last) { state.id = last.id; state.draft = null; }
    redraw();
  });

  const listCard = h('div', { class: 'card' },
    h('div', { class: 'row', style: 'margin-bottom:8px' }, h('h3', { style: 'margin:0', class: 'grow' }, at('mine'))),
    h('div', { class: 'row', style: 'margin-bottom:8px' },
      h('button', { class: 'btn sm primary', onclick: () => pick(null) }, '+ ' + at('newAircraft')),
      h('button', { class: 'btn sm', onclick: () => files.click() }, at('importFiles')), files),
    list.length === 0 ? h('p', { class: 'muted' }, at('none')) : list.map((a) => h('div', {
      class: 'routeitem' + (a.id === state.id ? ' on' : ''), onclick: () => pick(a)
    }, h('strong', null, a.name), h('div', { class: 'muted' }, [a.aircraftType, a.fuelUnit === 'lb' ? 'lb' : 'gal', a.cruise.speedKt + ' kt'].filter(Boolean).join(' - ')))));

  const editor = h('div', { class: 'card' },
    grid(field(at('name'), d.name, (v) => { d.name = v; }, { text: true }), field(at('type'), d.aircraftType, (v) => { d.aircraftType = v; }, { text: true }),
      h('label', { class: 'fld' }, h('span', { class: 'muted' }, at('fuelUnit')), fuelUnit),
      h('label', { class: 'fld' }, h('span', { class: 'muted' }, at('jet')), jet)),
    h('h4', null, at('fuelTitle') + ' (' + unit() + ')'),
    grid(field(at('usable'), d.usableFuel, (v) => { d.usableFuel = v; }), field(at('taxi'), d.taxiFuel, (v) => { d.taxiFuel = v; }),
      field(at('reserve'), d.reserveFuel, (v) => { d.reserveFuel = v; }), field(at('extra'), d.extraFuel, (v) => { d.extraFuel = v; }),
      field(at('contingency'), d.contingencyPct, (v) => { d.contingencyPct = v; })),
    h('h4', null, at('climb')),
    grid(field(at('speed'), d.climb.speedKt, (v) => { d.climb.speedKt = v; }), field(flowLabel(), d.climb.fuelFlow, (v) => { d.climb.fuelFlow = v; }),
      field(at('rate'), d.climb.vertFpm, (v) => { d.climb.vertFpm = v; })),
    h('h4', null, at('cruise')),
    grid(field(at('speed'), d.cruise.speedKt, (v) => { d.cruise.speedKt = v; }), field(flowLabel(), d.cruise.fuelFlow, (v) => { d.cruise.fuelFlow = v; })),
    h('h4', null, at('descent')),
    grid(field(at('speed'), d.descent.speedKt, (v) => { d.descent.speedKt = v; }), field(flowLabel(), d.descent.fuelFlow, (v) => { d.descent.fuelFlow = v; }),
      field(at('rate'), d.descent.vertFpm, (v) => { d.descent.vertFpm = v; })),
    h('h4', null, at('alternate')),
    grid(field(at('speed'), d.alternate.speedKt, (v) => { d.alternate.speedKt = v; }), field(flowLabel(), d.alternate.fuelFlow, (v) => { d.alternate.fuelFlow = v; })),
    grid(h('label', { class: 'fld' }, h('span', { class: 'muted' }, at('runway')), runway), field(at('minRunway'), d.minRunwayFt, (v) => { d.minRunwayFt = v; })),
    h('label', { class: 'fld', style: 'margin-top:10px' }, h('span', { class: 'muted' }, at('description')), notes),
    h('div', { class: 'row', style: 'margin-top:12px' },
      h('button', { class: 'btn primary', onclick: save }, at('save')),
      d.id ? h('button', { class: 'btn danger', onclick: remove }, at('del')) : null));

  view.replaceChildren(h('h2', null, at('title')), h('p', { class: 'muted' }, at('intro')), h('div', { class: 'routespage' }, listCard, editor));
}

// ---- the estimate on the page of the routes ----

// planPanel is the aircraft, the altitude and the estimate of a route. st is the state of the route being edited
// ({profileId, cruiseFt, windKt}); legs() gives the legs of the route as resolved, [] when it is not. refresh() asks the
// program again, for when the route changed.
export function planPanel(st, aircraft, legs) {
  const host = h('div', { class: 'ac-result' });
  const select = h('select', null, h('option', { value: '' }, at('noAircraft')), aircraft.map((a) => h('option', { value: a.id }, a.name)));
  select.value = st.profileId ? String(st.profileId) : '';
  const alt = h('input', { type: 'number', min: '0', step: '500', value: st.cruiseFt ? String(st.cruiseFt) : '' });
  const wind = h('input', { type: 'number', step: '1', value: st.windKt ? String(st.windKt) : '' });

  const refresh = async () => {
    st.profileId = select.value ? Number(select.value) : null;
    st.cruiseFt = num(alt.value);
    st.windKt = num(wind.value);
    const lens = legs();
    if (!aircraft.length) { host.replaceChildren(h('p', { class: 'muted' }, at('noneToChoose'))); return; }
    if (lens.length < 2) { host.replaceChildren(h('p', { class: 'muted' }, at('needRoute'))); return; }
    if (!st.profileId || st.cruiseFt <= 0) { host.replaceChildren(h('p', { class: 'muted' }, at('needChoice'))); return; }
    try {
      const first = lens[0], last = lens[lens.length - 1];
      const r = await api('POST', '/api/route/estimate', {
        profileId: st.profileId, legsNm: lens.map((l) => l.nm), cruiseFt: st.cruiseFt, windKt: st.windKt,
        departure: first.kind === 'airport' ? first.ident : '', destination: last.kind === 'airport' ? last.ident : '',
      });
      host.replaceChildren(...resultNodes(r, lens));
    } catch (e) { host.replaceChildren(h('p', { class: 'muted' }, at('failed') + ': ' + e.message)); }
  };
  [select, alt, wind].forEach((el) => el.addEventListener('change', refresh));

  const node = h('div', { class: 'planpanel' },
    h('h3', null, at('plan')),
    h('div', { class: 'ac-grid' },
      h('label', { class: 'fld' }, h('span', { class: 'muted' }, at('aircraft')), select),
      h('label', { class: 'fld' }, h('span', { class: 'muted' }, at('cruiseAlt')), alt),
      h('label', { class: 'fld' }, h('span', { class: 'muted' }, at('wind')), wind)),
    host);
  return { node, refresh };
}

function resultNodes(r, lens) {
  const u = r.fuelUnit === 'lb' ? 'lb' : 'gal';
  const stat = (label, value) => h('div', { class: 'ac-stat' }, h('div', { class: 'muted' }, label), h('strong', null, value));
  const row = (label, v) => h('tr', null, h('td', null, label), h('td', { class: 'num' }, fmt(v, 1) + ' ' + u));
  const out = [];
  out.push(h('div', { class: 'ac-stats' },
    stat(at('distance'), fmt(r.totalNm) + ' nm'), stat(at('time'), hm(r.minutes)), stat(at('trip'), fmt(r.tripFuel) + ' ' + u),
    stat(at('required'), fmt(r.required) + ' ' + u + (u === 'gal' ? ' (' + Math.round(r.requiredLb) + ' lb)' : ''))));
  if (r.usable > 0) out.push(h('p', { class: r.enoughFuel ? 'muted' : 'bad' }, at('usableFuel') + ': ' + fmt(r.usable) + ' ' + u + '. ' + (r.enoughFuel ? at('enough') : at('notEnough'))));
  if (r.truncated) out.push(h('p', { class: 'bad' }, at('truncated') + ' ' + Math.round(r.cruiseFt) + ' ' + at('ft') + '.'));
  out.push(h('p', { class: 'muted' }, at('elev') + ': ' + fmt(r.departureFt) + ' ' + at('ft') + ' > ' + fmt(r.destinationFt) + ' ' + at('ft')));
  out.push(h('p', { class: 'muted' }, at('topClimb') + ': ' + fmt(r.topOfClimbNm) + ' nm - ' + at('topDescent') + ': ' + fmt(r.topOfDescentNm) + ' nm (' + at('from') + ')'));
  out.push(h('table', null,
    h('thead', null, h('tr', null, ['phase', 'nm', 'min', 'fuel'].map((k) => h('th', null, k === 'phase' ? at('phase') : (k === 'fuel' ? at('fuel') + ' (' + u + ')' : at(k)))))),
    h('tbody', null, r.phases.map((p) => h('tr', null, h('td', null, at(p.name)), h('td', { class: 'num' }, fmt(p.nm)), h('td', { class: 'num' }, hm(p.minutes)), h('td', { class: 'num' }, fmt(p.fuel, 2)))))));
  out.push(h('table', null,
    h('thead', null, h('tr', null, h('th', null, at('fuel')), h('th', { class: 'num' }, u))),
    h('tbody', null, row(at('taxiRow'), r.taxiFuel), row(at('trip'), r.tripFuel), row(at('contingencyRow'), r.contingency),
      r.alternate ? row(at('alternateRow'), r.alternate) : null, row(at('reserveRow'), r.reserve), r.extra ? row(at('extraRow'), r.extra) : null,
      h('tr', null, h('td', null, h('strong', null, at('total'))), h('td', { class: 'num' }, h('strong', null, fmt(r.required, 1) + ' ' + u))))));
  out.push(h('table', null,
    h('thead', null, h('tr', null, h('th', null, at('leg')), h('th', { class: 'num' }, at('nm')), h('th', { class: 'num' }, at('min')), h('th', { class: 'num' }, at('fuel') + ' (' + u + ')'))),
    h('tbody', null, r.legs.map((l, i) => (i === 0 ? null : h('tr', null, h('td', null, (lens[i - 1] ? lens[i - 1].ident : '') + ' > ' + (lens[i] ? lens[i].ident : '')),
      h('td', { class: 'num' }, fmt(l.nm)), h('td', { class: 'num' }, hm(l.minutes)), h('td', { class: 'num' }, fmt(l.fuel, 2))))))));
  out.push(h('p', { class: 'muted', style: 'font-size:12px' }, at('note')));
  return out;
}
