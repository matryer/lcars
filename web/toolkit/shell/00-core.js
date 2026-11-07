// Helpers, themes, shared state, motion, typewriter and frame messaging helpers.
const $ = id => document.getElementById(id);
// Accent names resolve through the page theme; orange (warning) and
// critical stay fixed so health colours keep their meaning.
const palette = {apricot:'var(--apricot)',orange:'var(--orange)',lilac:'var(--lilac)',blue:'var(--blue)',peach:'var(--peach)',critical:'var(--critical)'};
const themes = {
  classic:{apricot:'#FFCC99',lilac:'#CC99CC',blue:'#9999FF',peach:'#FF9966'},
  pastel:{apricot:'#FFDD99',lilac:'#C9B6F2',blue:'#99CCFF',peach:'#FFB38A'},
  nebula:{apricot:'#F2A93B',lilac:'#D99AD0',blue:'#B49CE6',peach:'#E8B87A'},
  tidal:{apricot:'#8FB3FF',lilac:'#5C7FE0',blue:'#9FE0FF',peach:'#6FA8F0'},
  sand:{apricot:'#E8C48F',lilac:'#C7A383',blue:'#A6BFDD',peach:'#E49C73'},
  meridian:{apricot:'#FFCC66',lilac:'#E3DCB4',blue:'#9DDFFF',peach:'#3D85E8'},
};
const themeNames = Object.keys(themes);
// FNV-1a: a page keeps the same theme and decoration on every load.
function hash(text) { let h = 0x811c9dc5; for (let i = 0; i < text.length; i++) { h ^= text.charCodeAt(i); h = Math.imul(h, 0x01000193); } return h >>> 0; }
// Seeds are mixed (murmur3 finaliser) so similar page names diverge from the first draw.
function mix(h) { h ^= h >>> 16; h = Math.imul(h, 0x85ebca6b); h ^= h >>> 13; h = Math.imul(h, 0xc2b2ae35); return (h ^ h >>> 16) >>> 0; }
function random(seed) { let a = mix(hash(seed)); return () => { a = a + 0x6D2B79F5 | 0; let t = Math.imul(a ^ a >>> 15, 1 | a); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296; }; }
let themeName = 'classic', decorSeed = '';
function applyTheme(tab) {
  const name = themes[tab?.theme] ? tab.theme : themeNames[hash(tab?.id || '') % themeNames.length];
  if (name === themeName && document.documentElement.dataset.theme) return;
  themeName = name; document.documentElement.dataset.theme = name;
  for (const [key, value] of Object.entries(themes[name])) document.documentElement.style.setProperty('--' + key, value);
}
// Sections live at /s/{id} or /s/{id}/{sub}. Navigation between them stays in the page.
const sectionFromPath = () => location.pathname.startsWith('/s/') ? location.pathname.slice(3).split('/').map(decodeURIComponent).join('/') : '';
const tabHref = id => '/s/' + id.split('/').map(encodeURIComponent).join('/');
let section = sectionFromPath();
const panels = new Map(), instances = new Map();
function freshNonce() { return Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2,'0')).join(''); }
let state, etag = '', navigationKey = '', subnavKey = '', timer, backoff = 2000, lastCheck, polling = false, capturing = false, latched = false;
let frozenClock, captureError;
let eventStream, eventsHealthy = false, pendingCheck = false, eventFailures = 0;
let storedMotion = false;
try { storedMotion = localStorage.getItem('lcars-motion-off') === 'true'; } catch {}
const reduced = matchMedia('(prefers-reduced-motion: reduce)');
const motionOn = () => !storedMotion && !reduced.matches && !capturing;
function applyMotion() { document.documentElement.dataset.motion = motionOn() ? 'on' : 'off'; if (!motionOn()) finishTyping(); }
// Headings and labels type in quickly; with motion off they appear at once.
const typing = new Map();
function typeText(element, text, delay = 0) {
  text = String(text ?? '');
  if (element.dataset.text === text) return;
  element.dataset.text = text;
  cancelAnimationFrame(typing.get(element)); typing.delete(element);
  if (!motionOn() || !text) { element.textContent = text; element.classList.remove('typing'); element.removeAttribute('aria-label'); return; }
  const start = performance.now() + delay, perCharacter = Math.max(6, Math.min(16, 360 / text.length)) * (0.6 + hash(text) % 101 / 100); // 0.6–1.6×, fixed per text
  element.textContent = ''; element.classList.add('typing'); element.setAttribute('aria-label', text);
  const step = now => {
    const count = Math.max(0, Math.min(text.length, Math.ceil((now - start) / perCharacter)));
    if (element.textContent.length !== count) element.textContent = text.slice(0, count);
    if (count < text.length) typing.set(element, requestAnimationFrame(step));
    else { typing.delete(element); element.classList.remove('typing'); element.removeAttribute('aria-label'); }
  };
  typing.set(element, requestAnimationFrame(step));
}
function finishTyping() {
  for (const [element, frame] of typing) { cancelAnimationFrame(frame); element.textContent = element.dataset.text; element.classList.remove('typing'); element.removeAttribute('aria-label'); }
  typing.clear();
}
const setText = (element, text) => { if (element.textContent !== text) element.textContent = text; };
function node(tag, text, className) { const element = document.createElement(tag); if (text !== undefined) element.textContent = text; if (className) element.className = className; return element; }
const errorText = error => String(error?.message || error || 'Unknown error').slice(0,1500);
function diagnosticText(item) { return typeof item === 'string' ? item : item?.message || item?.error || JSON.stringify(item); }
function displayClock() { return frozenClock || new Date().toISOString(); }
// Only frames in the current section are visible; parked (preloaded) sections' frames are not.
function frameState(instance) { return {visible: !document.hidden && !capturing && (!instance || instance.record?.section === section), motion: !storedMotion && !reduced.matches && !capturing, capture: capturing, clock: displayClock(), theme: themes[themeName]}; }
// Every state sent to a frame is that frame's own state, whoever built it.
function post(instance, type, values = {}) { if (values.state) values = {...values, state: frameState(instance)}; instance.frame.contentWindow?.postMessage({v:1,nonce:instance.nonce,type,...values}, '*'); }
function sendLifecycle() { for (const instance of instances.values()) post(instance,'state',{state:frameState(instance)}); }
// Build-in delays: random but stable per key (0–520ms, weighted early), so pages assemble organically.
function entryDelay(key, span = 520) { const u = mix(hash(key)) / 4294967296; return Math.round(span * Math.pow(u, 1.6)); }
function dispose(instance) { if (!instance) return; clearTimeout(instance.deadline); instances.delete(instance.nonce); instance.frame.remove(); }
function dataValue(panel) { return panel.data || {hash:'missing',missing:true,envelope:null}; }
function dataHash(panel) { return dataValue(panel).hash || 'missing'; }
function validTime(value) { return !!value && Number.isFinite(Date.parse(value)) && new Date(value).getUTCFullYear() > 1; }
function updatedFor(panel, data) {
  const dates = [panel.updatedAt, data?.updatedAt].filter(validTime);
  return dates.sort((a,b) => Date.parse(b)-Date.parse(a))[0] || '';
}
function observation(instance) {
  if (!instance) return {};
  const data = instance.ackData;
  if (data && !data.missing) return {date:data.envelope?.updatedAt,stale:data.envelope?.staleAfterSeconds};
  return {date:instance.panel.observedAt,stale:instance.panel.staleAfterSeconds};
}
