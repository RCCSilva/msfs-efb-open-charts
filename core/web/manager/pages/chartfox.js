import { h } from '../core/dom.js';
import { api, toast } from '../core/api.js';
import { t, lang } from '../core/i18n.js';
import { pageLink } from '../core/app.js';

const CF_TYPES = [['IAC', 'cfTypeIAC'], ['SID', 'cfTypeSID'], ['STAR', 'cfTypeSTAR'], ['ADC', 'cfTypeADC'], ['', 'cfTypeOther']];
const CF_BATCH = 20; // what one import request may carry

// The airport last looked up on the ChartFox page, kept while the manager stays open.
const cfLookup = { icao: '' };

// The lookup of ChartFox, shown on its page under Sources: type an airport, tick charts, import. It redraws itself in
// place (no page of its own).
export function chartFoxPanel() {
  const root = h('div');
  const draw = async () => {
    const icao = cfLookup.icao;
    let st;
    try { st = await api('GET', '/api/chartfox'); } catch (e) { root.replaceChildren(h('div', { class: 'card' }, h('span', { class: 'badge bad' }, e.message))); return; }
    const credit = h('div', { class: 'muted', style: 'margin-bottom:16px' },
      h('a', { href: 'https://chartfox.org', target: '_blank', rel: 'noopener' }, t('cfCredit')), ' · ', t('cfSimOnly'));
    if (!st.available) { root.replaceChildren(h('div', { class: 'card' }, h('div', { class: 'muted' }, t('cfUnavailable')))); return; }
    if (!st.connected) {
      root.replaceChildren(h('div', { class: 'card' }, h('h3', null, t('cfLookupTitle')), h('p', null, t('cfNeedConnect')), pageLink('#setup', 'btn primary', t('cfGoSetup'))), credit);
      return;
    }

    const box = h('input', { type: 'text', placeholder: t('cfIcao'), value: icao, maxlength: '4', style: 'text-transform:uppercase;max-width:140px' });
    const form = h('form', { class: 'row', onsubmit: (e) => {
      e.preventDefault();
      cfLookup.icao = box.value.trim().toUpperCase();
      draw();
    } }, box, h('button', { class: 'btn primary', type: 'submit' }, t('cfLookup')));
    const out = h('div');
    root.replaceChildren(h('div', { class: 'card' }, h('h3', null, t('cfLookupTitle')), form), out, credit);
    if (!icao) { out.append(h('div', { class: 'muted', style: 'margin-bottom:16px' }, t('cfHint'))); return; }

    out.append(h('div', { class: 'muted' }, t('cfLoading')));
    let data;
    try { data = await api('GET', '/api/chartfox/airports/' + encodeURIComponent(icao)); } catch (e) {
      out.replaceChildren(h('div', { class: 'card' }, h('span', { class: 'badge bad' }, e.message)));
      return;
    }

    const picked = new Set();
    const importBtn = h('button', { class: 'btn primary' }, '');
    const sync = () => {
      importBtn.textContent = t('cfImport') + ' (' + picked.size + ')';
      importBtn.disabled = picked.size === 0;
    };
    importBtn.addEventListener('click', async () => {
      const ids = [...picked];
      importBtn.disabled = true;
      importBtn.textContent = t('cfImporting');
      let ok = 0, already = 0, failed = '';
      try {
        for (let i = 0; i < ids.length; i += CF_BATCH) {
          const r = await api('POST', '/api/chartfox/import', { ids: ids.slice(i, i + CF_BATCH) });
          r.results.forEach((x) => {
            if (x.status === 'ok') ok++; else if (x.status === 'already') already++; else if (!failed) failed = x.message;
          });
          if (failed) break;
        }
      } catch (e) { failed = e.message; }
      toast(ok + ' ' + t('cfImported') + (already ? ', ' + already + ' ' + t('cfAlready') : '') + (failed ? ' · ' + failed : ''));
      draw();
    });

    const a = data.airport || {};
    const groups = CF_TYPES.map(([code, label]) => ({
      label,
      items: data.charts.filter((c) => (c.type || '') === code).sort((p, q) => p.name.localeCompare(q.name, lang, { numeric: true })),
    })).filter((g) => g.items.length);

    const cards = groups.map((g, gi) => {
      const rows = g.items.map((c) => {
        const cb = h('input', { type: 'checkbox', disabled: c.imported, onchange: (e) => {
          if (e.target.checked) picked.add(c.id); else picked.delete(c.id);
          sync();
        } });
        cb.dataset.id = c.id;
        return h('label', { class: 'row', style: 'padding:4px 0;cursor:pointer' }, cb, h('span', null, c.name),
          c.code ? h('span', { class: 'badge' }, c.code) : null,
          c.imported ? h('span', { class: 'badge ok' }, t('cfInLibrary')) : null);
      });
      const card = h('div', { class: 'card' });
      const toggle = h('button', { class: 'btn sm', onclick: () => {
        const boxes = [...card.querySelectorAll('input[type=checkbox]:not(:disabled)')];
        const on = boxes.some((b) => !b.checked);
        boxes.forEach((b) => { b.checked = on; if (on) picked.add(b.dataset.id); else picked.delete(b.dataset.id); });
        sync();
      } }, t('cfSelectAll'));
      card.append(h('div', { class: 'row' }, h('h3', { style: 'margin:0' }, t(g.label) + ' (' + g.items.length + ')'), toggle), ...rows);
      return card;
    });
    out.replaceChildren(
      h('div', { class: 'row', style: 'margin-bottom:12px' }, h('strong', null, (a.ident || icao) + (a.name ? ' — ' + a.name : '')), h('span', { class: 'muted' }, data.charts.length + ' ' + t('charts')), importBtn),
      cards.length ? h('div', null, cards) : h('div', { class: 'muted' }, t('cfNone')));
    sync();
  };
  draw();
  return root;
}
