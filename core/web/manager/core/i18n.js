// The language of the manager and the lookup of its texts.
import en from '../i18n/en.js';
import pt from '../i18n/pt.js';

export const STR = { en, pt };

export let lang = 'en';
try { lang = localStorage.getItem('oc.lang') || ((navigator.language || 'en').toLowerCase().startsWith('pt') ? 'pt' : 'en'); } catch (e) { /* ignore */ }
const qLang = new URLSearchParams(location.search).get('lang');
if (qLang === 'pt' || qLang === 'en') lang = qLang;
export const t = (k) => STR[lang][k] || STR.en[k] || k;
export const fill = (text, vars) => text.replace(/\{(\w+)\}/g, (m, k) => (k in vars ? vars[k] : m));
// 45 -> "a few seconds", 600 -> "10 min", 7500 -> "2 h 5 min" (an estimate: never more precise than a minute)
export function fmtDuration(sec) {
  const d = STR[lang].pkgDuration || STR.en.pkgDuration;
  if (sec < 60) return d.s;
  const min = Math.max(1, Math.round(sec / 60));
  if (min < 60) return fill(d.m, { n: min });
  return fill(d.h, { h: Math.floor(min / 60), m: min % 60 });
}
export const tx = (o) => (o && (o[lang] || o.en)) || '';

// setLang changes the language (the pages read `lang` through the live binding).
export function setLang(l) { lang = l; }
