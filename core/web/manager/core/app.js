// What the pages share: the state, the page element, the way to go to a page and to draw it again.
import { h } from './dom.js';

export const state = { groups: { current: '', groups: [] } };
export const view = document.getElementById('view');

// The shell sets render (the pages call it without importing the shell).
export const app = { render: () => {} };

export const go = (hash) => { if (location.hash === hash) app.render(); else location.hash = hash; };

// A page is a real link (an <a>): it can be opened in a new tab with the middle button or ctrl+click.
// A plain click is handled in place, without reloading the page.
export function pageLink(hash, cls, ...kids) {
  return h('a', { class: cls, href: location.pathname + location.search + hash, onclick: (e) => {
    if (e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    go(hash);
  } }, ...kids);
}
