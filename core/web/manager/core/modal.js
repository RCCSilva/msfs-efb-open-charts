// A confirmation as a modal: true on the confirm button, false on Cancel, on Escape and on a click outside the box.
import { h } from './dom.js';
import { t } from './i18n.js';

export function confirmModal({ title, text, ok, danger = true }) {
  return new Promise((resolve) => {
    const onKey = (e) => { if (e.key === 'Escape') done(false); };
    const done = (answer) => {
      document.removeEventListener('keydown', onKey);
      back.remove();
      resolve(answer);
    };
    const back = h('div', { class: 'modal' },
      h('div', { class: 'card modalbox' },
        h('h3', null, title),
        h('p', { style: 'white-space:pre-line' }, text),
        h('div', { class: 'row' },
          h('button', { class: 'btn ' + (danger ? 'danger' : 'primary'), onclick: () => done(true) }, ok),
          h('button', { class: 'btn', onclick: () => done(false) }, t('cancel')))));
    back.addEventListener('mousedown', (e) => { if (e.target === back) done(false); });
    document.addEventListener('keydown', onKey);
    document.body.appendChild(back);
  });
}
