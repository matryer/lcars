// Decoration: chrome segments, gap fillers and their interactions.
// Decoration: drawn by the host, hidden from assistive technology and focus,
// generated from the page ID so it is identical on every load and capture.
const colours = ['apricot','apricot','lilac','blue','peach','lilac','orange'];
const pick = (r, list) => list[Math.floor(r() * list.length)];
const codeText = r => String(10 + Math.floor(r() * 90)) + '-' + String(1000 + Math.floor(r() * 9000));
const digits = (r, min = 1, max = 4) => String(Math.floor(r() * 10 ** (min + Math.floor(r() * (max - min + 1)))));
function block(r, className, label) { const element = node('i', label, className); element.style.setProperty('--tone', `var(--${pick(r, colours)})`); return element; }
// Now and then a highlight sweeps across a chrome segment (CSS only, motion on).
function glint(r, element) {
  if (r() < .45) { element.classList.add('glint'); element.style.setProperty('--glint-d', fixed(9 + r() * 12) + 's'); element.style.setProperty('--glint-t', fixed(-r() * 20) + 's'); }
  return element;
}
function chrome(seed) {
  const r = random('chrome:' + seed);
  const fill = document.querySelector('.rail-fill'); fill.replaceChildren();
  for (let i = 0, n = 3 + Math.floor(r() * 3); i < n; i++) { const segment = block(r, 'segment', codeText(r)); segment.style.flexGrow = String(1 + Math.floor(r() * 4)); fill.append(glint(r, segment)); }
  if (r() < .6) { const box = node('span', undefined, 'rail-lights'); box.append(lights(r)); fill.insertBefore(box, fill.children[1 + Math.floor(r() * fill.children.length)] || null); }
  const lit = r() < .5 ? '.masthead-segments' : '.footer-segments';
  for (const [selector, count] of [['.masthead-segments', 2 + Math.floor(r() * 3)], ['.footer-segments', 2 + Math.floor(r() * 3)]]) {
    const box = document.querySelector(selector); box.replaceChildren();
    for (let i = 0; i < count; i++) { const segment = block(r, 'segment', r() < .5 ? digits(r, 2, 4) : ''); segment.style.width = (28 + Math.floor(r() * 110)) + 'px'; box.append(glint(r, segment)); }
    if (selector === lit) box.insertBefore(lights(r, 'bar-lights'), box.children[Math.floor(r() * (count + 1))] || null);
  }
}
// Generic labels only: nothing that reads as a real control or a real alarm.
const words = ['STATUS','PRIMARY','SECONDARY','AUXILIARY','PRIMARY ACCESS','SECONDARY ACCESS','SYSTEMS','SYSTEMS ACCESS','CONTROL','MODE SELECT','DATA RELAY','ARCHIVE','SEQUENCE','INDEX','REGISTER','CHANNEL','LIBRARY','MONITOR','DIRECTORY','LOCAL'];
const letters = 'BCDFGHJKLMNPRSTVWXZAEO';
const letterCode = r => Array.from({length: 2}, () => letters[Math.floor(r() * letters.length)]).join('') + ' ' + Array.from({length: 3}, () => letters[Math.floor(r() * letters.length)]).join('');
// Every filler is built with more items than a large tile shows; data-fit
// (53-fit.js) hides the ones that would be cut by the tile's edge.
function keypad(r, element) {
  element.classList.add('keypad'); element.dataset.fit = 'trail';
  for (let i = 0; i < 96; i++) {
    const numeric = r() < .35;
    const key = block(r, 'key' + (numeric ? ' num' : '') + (r() < .35 ? ' square' : ''), numeric ? digits(r, 3, 4) : letterCode(r));
    if (r() < .18) key.classList.add('wide');
    element.append(key);
  }
}
// Consoles fill the cell exactly: flexible columns of D-shaped keys, word
// blocks and ruled gauges, after the reference panels' proportions.
function keyColumn(r, count, wordy) {
  const column = node('span', undefined, 'dcol' + (wordy ? ' words' : '')); column.dataset.fit = 'trail';
  for (let i = 0; i < count; i++) {
    const key = block(r, wordy ? 'wblock' : 'dkey' + (r() < .25 ? ' square' : ''), wordy ? pick(r, words) : letterCode(r));
    key.style.flexGrow = String(wordy ? 1 + Math.floor(r() * 2) : 1);
    column.append(key);
  }
  return column;
}
function gauge(r) {
  const element = block(r, 'gauge');
  const fill = node('span', undefined, 'gauge-fill'); element.append(fill);
  const scale = node('span', undefined, 'scale');
  for (let n = 7; n >= 1; n--) { scale.append(node('span', undefined, 'tick major')); scale.lastChild.append(node('b', String(n))); if (n > 1) scale.append(node('span', undefined, 'tick')); }
  element.append(scale);
  // A scan line crosses each gauge at its own pace.
  element.style.setProperty('--scan-d', fixed(2.4 + r() * 2.6) + 's'); element.style.setProperty('--scan-t', fixed(-r() * 5) + 's');
  setGauge(element, 20 + Math.floor(r() * 65));
  return element;
}
function setGauge(element, value) { element.dataset.level = String(value); element.querySelector('.gauge-fill').style.height = value + '%'; }
function gauges(r, element, span) {
  element.classList.add('console'); element.dataset.fit = 'trail';
  for (let i = 0; i < span + 1; i++) { element.append(gauge(r), keyColumn(r, 4 + Math.floor(r() * 3), false)); }
}
function blocks(r, element, span) {
  element.classList.add('console'); element.dataset.fit = 'trail';
  for (let i = 0; i < span + 1; i++) { element.append(keyColumn(r, 4 + Math.floor(r() * 3), true), keyColumn(r, 5 + Math.floor(r() * 3), false)); }
}
function readout(r, element) {
  element.classList.add('readout'); element.dataset.fit = 'trail';
  for (let i = 0; i < 90; i++) {
    const row = node('span', undefined, 'row');
    row.append(block(r, 'tag'), block(r, 'number', digits(r, 1, 4)), block(r, 'tag' + (r() < .5 ? ' short' : '')));
    element.append(row);
  }
}
function meters(r, element, span) {
  element.classList.add('meters'); element.dataset.fit = 'trail';
  element.style.setProperty('--scan-d', fixed(4 + r() * 3) + 's');
  for (let i = 0, n = span * 8 + 6 + Math.floor(r() * 3); i < n; i++) {
    const meter = node('span', undefined, 'meter'); meter.style.setProperty('--tone', `var(--${pick(r, colours)})`); meter.dataset.fit = 'top';
    meter.dataset.level = String(4 + Math.floor(r() * 13));
    for (let j = 0; j < meterCells; j++) meter.append(node('i'));
    meter.append(node('b', digits(r, 2, 3)));
    element.append(meter);
  }
  for (const meter of element.querySelectorAll('.meter')) level(meter, Number(meter.dataset.level));
}
function bars(r, element) {
  element.classList.add('bars'); element.dataset.fit = 'trail';
  element.style.setProperty('--scan-d', fixed(3.5 + r() * 3) + 's');
  for (let i = 0; i < 32; i++) { const bar = block(r, 'bar', codeText(r)); bar.style.width = (30 + Math.floor(r() * 70)) + '%'; element.append(bar); }
}
// Meters are built tall; in short tiles 53-fit.js drops cells from the top, so levels read from the bottom.
const meterCells = 20;
function level(meter, value) { meter.querySelectorAll('i').forEach((cell, index) => cell.classList.toggle('on', meterCells - 1 - index < value)); meter.dataset.level = String(value); }
// Weights: blocks 2, gauges 2, keypad 1, readout 1, meters 1, bars 1, dials 2, crosshair 2 (dials and crosshair: 51-dials.js).
const kinds = [blocks, gauges, keypad, blocks, gauges, readout, meters, bars, dials, dials, crosshair, crosshair];
function decoration(seed, span) {
  const r = random(seed);
  const element = node('div', undefined, 'deco'); element.setAttribute('aria-hidden', 'true');
  element.style.gridColumn = 'span ' + span; element.style.setProperty('--accent', `var(--${pick(r, colours.filter(c => c !== 'orange'))})`);
  element.append(node('span', codeText(r), 'deco-code'));
  const body = node('div', undefined, 'deco-body'); pick(r, kinds)(r, body, span); element.append(body);
  // Some tiles carry a light grid beside their code (dial tiles have one in their side column).
  if (!body.classList.contains('dials') && r() < .4) element.append(lights(r, 'deco-lights'));
  fitObserver?.observe(body);
  return element;
}
// One shared timer, only while visible with motion on: readout cursors step a
// row each tick; every third tick numbers, dial readouts, gauges and meters drift.
let decorTick = 0;
setInterval(() => {
  if (document.hidden || !motionOn()) return;
  const root = $('panels'); if (!root) return;
  for (const readout of root.querySelectorAll('.deco .readout')) {
    const rows = [...readout.children].filter(row => !row.hidden); if (!rows.length) continue;
    const current = readout.querySelector(':scope > .scan'); current?.classList.remove('scan');
    rows[(rows.indexOf(current) + 1) % rows.length].classList.add('scan');
  }
  if (++decorTick % 3) return;
  const numbers = root.querySelectorAll('.deco .number');
  for (let i = 0; i < Math.min(3, numbers.length); i++) { const cell = numbers[Math.floor(Math.random() * numbers.length)]; cell.textContent = String(Math.floor(Math.random() * 10 ** cell.textContent.length)); }
  for (const value of root.querySelectorAll('.deco .dial-value')) if (Math.random() < .3) value.textContent = String(Math.floor(Math.random() * 10 ** value.textContent.length));
  for (const element of root.querySelectorAll('.deco .gauge')) if (Math.random() < .35) setGauge(element, Math.max(10, Math.min(92, Number(element.dataset.level) + Math.round((Math.random() - .5) * 18))));
  for (const meter of root.querySelectorAll('.deco .meter')) if (Math.random() < .5) level(meter, Math.max(1, Math.min(meterCells, Number(meter.dataset.level) + (Math.random() < .5 ? -1 : 1))));
}, 400);
// Decoration responds to touch and does nothing else: keys and dial segments
// light and set off a chase, numbers re-roll, gauges and meters follow the pointer.
function flash(elements, step = 55) {
  if (!motionOn()) return;
  elements.forEach((element, index) => setTimeout(() => { element.classList.remove('flash'); void element.getBoundingClientRect(); element.classList.add('flash'); }, index * step));
}
const fraction = (event, element, axis) => { const box = element.getBoundingClientRect(); return axis === 'x' ? (event.clientX - box.left) / box.width : 1 - (event.clientY - box.top) / box.height; };
// Delegated from main: sections live in pooled containers and only the current one is #panels.
$('main').addEventListener('click', event => {
  const key = event.target.closest?.('.deco .key, .deco .tag, .deco .dkey, .deco .wblock, .deco .arc');
  if (key) {
    key.classList.toggle('lit');
    const all = [...key.parentNode.children].filter(element => element !== key && !element.hidden && (key.matches('.arc') ? element.matches('.arc') : !element.matches('.number')));
    const at = all.findIndex(element => key.compareDocumentPosition(element) & Node.DOCUMENT_POSITION_FOLLOWING);
    flash([key, ...all.slice(Math.max(0, at), Math.max(0, at) + 4)]);
    return;
  }
  const number = event.target.closest?.('.deco .number');
  if (number) { number.textContent = String(Math.floor(Math.random() * 10 ** Math.max(1, number.textContent.length))); flash([number]); return; }
  const meter = event.target.closest?.('.deco .meter');
  if (meter) { level(meter, Math.max(1, Math.min(meterCells, Math.round(fraction(event, meter, 'y') * meterCells)))); return; }
  const bar = event.target.closest?.('.deco .bar');
  if (bar) { bar.style.width = Math.max(12, Math.min(100, Math.round(fraction(event, bar.parentElement, 'x') * 100))) + '%'; flash([bar]); }
});
let dragging;
$('main').addEventListener('pointerdown', event => {
  const element = event.target.closest?.('.deco .gauge'); if (!element) return;
  dragging = element; element.setPointerCapture(event.pointerId); element.classList.add('held');
  setGauge(element, Math.max(4, Math.min(96, Math.round(fraction(event, element, 'y') * 100))));
});
$('main').addEventListener('pointermove', event => { if (dragging) setGauge(dragging, Math.max(4, Math.min(96, Math.round(fraction(event, dragging, 'y') * 100)))); });
const release = () => { dragging?.classList.remove('held'); dragging = undefined; };
$('main').addEventListener('pointerup', release); $('main').addEventListener('pointercancel', release);
document.querySelector('.rail-fill').addEventListener('click', event => { const segment = event.target.closest('.segment'); if (!segment) return; segment.classList.toggle('lit'); flash([...segment.parentElement.querySelectorAll('.segment')], 70); });
