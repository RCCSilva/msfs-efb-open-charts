import { h } from '../core/dom.js';
import { api, toast } from '../core/api.js';
import { t, lang } from '../core/i18n.js';
import { confirmModal } from '../core/modal.js';

export const CF_TERMS_URL = 'https://chartfox.org/legal/terms';
export const CF_PRIVACY_URL = 'https://chartfox.org/legal/privacy';

const link = (href, text) => h('a', { href, target: '_blank', rel: 'noopener' }, text);

// What connecting an account means, in a few points: the ChartFox terms and privacy policy are theirs, and the user
// accepts them there.
function cfPoints() {
  return h('ul', { class: 'cf-points' },
    h('li', null, t('cfLegalLead') + ' ', link(CF_TERMS_URL, t('cfTerms')), ' ' + t('cfAnd') + ' ', link(CF_PRIVACY_URL, t('cfPrivacy')), '.'),
    h('li', null, t('cfPoint2')), h('li', null, t('cfPoint3')), h('li', null, t('cfPoint4')));
}

const cfCredit = () => h('div', { class: 'muted', style: 'margin-top:8px' }, link('https://chartfox.org', t('cfCredit')));

const CF_TYPES = [['IAC', 'cfTypeIAC'], ['SID', 'cfTypeSID'], ['STAR', 'cfTypeSTAR'], ['ADC', 'cfTypeADC'], ['', 'cfTypeOther']];
const CF_BATCH = 20; // what one import request may carry

// The airport last looked up on the ChartFox page, kept while the manager stays open.
const cfLookup = { icao: '' };

// The ChartFox page under Sources: the account (connect, disconnect and what it means), the switch of the search from the
// EFB and the lookup (type an airport, tick charts, import). The lookup redraws itself in place (no page of its own).
// local: the page is open on the computer itself; a device of the network only reads what connecting means.
export function chartFoxPage(local) {
  const root = h('div');
  if (!local) { root.append(h('div', { class: 'card' }, h('h3', null, t('cfTitle')), cfPoints(), cfCredit())); return root; }
  const draw = async () => {
    const icao = cfLookup.icao;
    let st;
    try { st = await api('GET', '/api/chartfox'); } catch (e) { root.replaceChildren(h('div', { class: 'card' }, h('span', { class: 'badge bad' }, e.message))); return; }
    if (st.connecting) setTimeout(() => { if (root.isConnected) draw(); }, 2000);

    let account;
    if (!st.available) account = h('div', { class: 'muted' }, t('cfUnavailable'));
    else if (st.connecting) account = h('div', { class: 'muted' }, t('cfWaiting'));
    else if (st.connected) account = h('div', { class: 'row' },
      h('span', { class: 'grow' }, t('cfAs') + ' ' + (st.user || 'ChartFox')),
      h('button', { class: 'btn sm danger', onclick: async () => {
        if (!await confirmModal({ title: t('areYouSure'), text: t('cfConfirmDisconnect'), ok: t('cfDisconnect') })) return;
        try { await api('POST', '/api/chartfox/disconnect'); toast(t('done')); } catch (e) { toast(e.message); }
        draw();
      } }, t('cfDisconnect')));
    else account = h('div', { class: 'row' },
      h('button', { class: 'btn primary', onclick: async () => {
        try { await api('POST', '/api/chartfox/connect'); } catch (e) { toast(e.message); }
        draw();
      } }, t('cfConnect')));
    const accountCard = h('div', { class: 'card' }, h('h3', null, t('cfTitle')), account,
      st.error ? h('div', { class: 'muted', style: 'margin-top:8px' }, st.error) : null, cfPoints(), cfCredit());
    if (!st.available || !st.connected) { root.replaceChildren(accountCard); return; }

    const sw = h('button', { class: 'btn sm' + (st.search ? ' primary' : ''), onclick: async () => {
      try { await api('PUT', '/api/chartfox/search', { on: !st.search }); } catch (e) { toast(e.message); }
      draw();
    } }, st.search ? t('cfSearchOn') : t('cfSearchOff'));
    const searchCard = h('div', { class: 'card' }, h('div', { class: 'row' }, h('h3', { class: 'grow', style: 'margin:0' }, t('cfSearchTitle')), sw),
      h('div', { class: 'muted', style: 'margin-top:8px' }, t('cfSearchHelp')));

    const box = h('input', { type: 'text', placeholder: t('cfIcao'), value: icao, maxlength: '4', style: 'text-transform:uppercase;max-width:140px' });
    const form = h('form', { class: 'row', onsubmit: (e) => {
      e.preventDefault();
      cfLookup.icao = box.value.trim().toUpperCase();
      draw();
    } }, box, h('button', { class: 'btn primary', type: 'submit' }, t('cfLookup')));
    const out = h('div');
    root.replaceChildren(accountCard, searchCard, h('div', { class: 'card' }, h('h3', null, t('cfLookupTitle')), form), out);
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
