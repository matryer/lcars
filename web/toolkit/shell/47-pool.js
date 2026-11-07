// Section pool: each loaded section renders into its own grid container. The
// current one has id="panels"; the rest are parked (laid out at the real width
// but invisible, inert and taking no space), so returning to them, or opening a
// preloaded one, shows frames that are already running. Containers and panel
// wrappers never move in the DOM: re-parenting an iframe reloads it.
const pool = new Map();
const POOL_BUDGET = 48; // live panels across parked sections
// The page's own container is reused rather than removed, so listeners bound to it survive.
const pageContainer = $('panels');
let visits = 0, poolOrder = 0, navigating = false, preloading = '', preloadQueued = false;
observeLayout(pageContainer);
const currentRecords = () => pool.get(section)?.records || [];
function park(container) {
  container.classList.add('parked'); container.setAttribute('inert', ''); container.setAttribute('aria-hidden', 'true');
  if (container.id === 'panels') container.removeAttribute('id');
}
function unpark(container) {
  container.classList.remove('parked'); container.removeAttribute('inert'); container.removeAttribute('aria-hidden');
  container.id = 'panels';
}
// New sections start parked; showSection() unparks the current one.
function sectionEntry(id) {
  let entry = pool.get(id);
  if (entry) return entry;
  let container = pageContainer;
  if ([...pool.values()].some(other => other.container === pageContainer)) { container = node('div', undefined, 'panels'); $('empty').before(container); observeLayout(container); }
  park(container);
  container.dataset.section = id;
  entry = {id, container, records: [], tiles: new Map(), visited: 0, order: ++poolOrder};
  pool.set(id, entry);
  return entry;
}
// Make a section current: park every other container and unpark this one.
function showSection(id) {
  const entry = sectionEntry(id);
  if (entry.container.classList.contains('parked')) {
    for (const other of pool.values()) if (other !== entry) park(other.container);
    unpark(entry.container);
    entry.visited = ++visits;
    for (const record of entry.records) holdPlaceholder(record);
  }
  if (preloading === id) preloading = '';
  return entry;
}
function removeRecord(record) {
  if (lifted?.record === record) closeExpand(true);
  clearTimeout(record.holdTimer);
  dispose(record.candidate); dispose(record.display); record.candidate = record.display = undefined;
  unobserveLayout(record.body);
  record.wrapper.remove();
  if (panels.get(record.latest.id) === record) panels.delete(record.latest.id);
}
// Dispose a parked section's frames and its container (never the current one).
function dropSection(entry) {
  if (entry.id === section) return;
  for (const record of entry.records) removeRecord(record);
  for (const tile of entry.tiles.values()) tile.remove();
  entry.records = []; entry.tiles.clear();
  pool.delete(entry.id);
  if (preloading === entry.id) preloading = '';
  if (entry.container === pageContainer) { pageContainer.replaceChildren(); delete pageContainer.dataset.section; park(pageContainer); }
  else { unobserveLayout(entry.container); entry.container.remove(); }
}
const parkedLive = () => [...pool.values()].reduce((sum, entry) => sum + (entry.id === section ? 0 : entry.records.length), 0);
// Over budget, the least recently visited parked sections go first (preloaded, never-visited ones before visited ones).
function enforceBudget() {
  while (parkedLive() > POOL_BUDGET) {
    const victim = [...pool.values()].filter(entry => entry.id !== section).sort((a, b) => a.visited - b.visited || b.order - a.order)[0];
    if (!victim) return;
    dropSection(victim);
  }
}
// A section is settled when every panel is displayed or has failed.
const settled = entry => !!entry && entry.records.every(record => record.display || record.failedHash || !record.latest.bundleHash || !record.latest.url);
// Preload order: the current section's sibling tabs, then top-level sections in
// rail order (each landing on its first sub-section when it has no own panels),
// then every other sub-section.
function preloadCandidates() {
  const tabs = state?.tabs || [], counts = new Map();
  for (const panel of state?.panels || []) counts.set(panel.tab, (counts.get(panel.tab) || 0) + 1);
  const top = section.split('/')[0];
  const ids = [
    ...tabs.filter(tab => tab.id === top || tab.parent === top).map(tab => tab.id),
    ...tabs.filter(tab => !tab.parent).map(tab => landing(tab.id)),
    ...tabs.filter(tab => tab.parent).map(tab => tab.id),
  ];
  return [...new Set(ids)].filter(id => id !== section && !pool.has(id) && counts.get(id)).map(id => [id, counts.get(id)]);
}
function idle(callback) { if (window.requestIdleCallback) requestIdleCallback(callback, {timeout: 2000}); else setTimeout(callback, 150); }
// Called whenever a panel is displayed or fails: once the current section (and
// any section being preloaded) has settled, preload the next one when idle.
function sectionProgress() { schedulePreload(); }
function schedulePreload() {
  if (preloadQueued || capturing || !state || !settled(pool.get(section))) return;
  if (preloading && pool.has(preloading) && !settled(pool.get(preloading))) return;
  preloadQueued = true;
  idle(() => { preloadQueued = false; preloadNext(); });
}
function preloadNext() {
  if (capturing || !state || !settled(pool.get(section))) return;
  if (preloading && pool.has(preloading) && !settled(pool.get(preloading))) return;
  preloading = '';
  const used = parkedLive();
  const next = preloadCandidates().find(([, count]) => used + count <= POOL_BUDGET);
  if (!next) return;
  const entry = sectionEntry(next[0]);
  preloading = entry.id;
  syncSection(entry, (state.panels || []).filter(panel => panel.tab === entry.id));
  schedulePreload();
}
