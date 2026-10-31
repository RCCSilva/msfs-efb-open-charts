import { h } from '../core/dom.js';
import { t, lang } from '../core/i18n.js';
import { view } from '../core/app.js';

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

export function renderGuide() {
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
