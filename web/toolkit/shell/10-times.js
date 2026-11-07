// Time formatting, the single scheduled relative-time timer, and updateTimes.
function exact(value) { const date = new Date(value); return validTime(value) ? new Intl.DateTimeFormat(undefined,{dateStyle:'full',timeStyle:'long'}).format(date) : 'Unknown'; }
function relative(value) {
  const ms = Date.parse(value); if (!validTime(value)) return '—';
  const age = Date.parse(displayClock())-ms;
  if (age < -1000) return exact(value) + ' (clock skew: future timestamp)';
  if (age < 3600000) return Math.max(1, Math.floor(age/60000)) + 'm';
  if (age < 86400000) return Math.floor(age/3600000) + 'h';
  return Math.floor(age/86400000) + 'd';
}
function compact(value) {
  const delta = Date.parse(value)-Date.parse(displayClock()), size = Math.abs(delta);
  const [step,letter] = size < 3600000 ? [60000,'m'] : size < 86400000 ? [3600000,'h'] : [86400000,'d'];
  const count = Math.max(1, Math.floor(size/step));
  return delta > 0 ? 'in '+count+letter : count+letter;
}
// One timer, set for the next moment any visible relative label would change.
let timesTimer;
function untilChange(value) {
  const ms = Date.parse(value); if (!Number.isFinite(ms)) return Infinity;
  const delta = ms - Date.parse(displayClock()), size = Math.abs(delta);
  const step = size < 3600000 ? 60000 : size < 86400000 ? 3600000 : 86400000;
  return delta <= 0 ? (Math.floor(size/step)+1)*step - size : (size % step) || step;
}
function scheduleTimes(values) {
  clearTimeout(timesTimer);
  if (frozenClock || document.hidden) return;
  const wait = Math.min(3600000, ...values.map(untilChange));
  if (Number.isFinite(wait)) timesTimer = setTimeout(updateTimes, Math.max(1000, wait + 50));
}
function updateTimes() {
  let latest; const watched = [];
  // Only the current section is shown; parked sections refresh when they are opened.
  for (const record of currentRecords()) {
    const instance = record.display;
    const updated = instance ? updatedFor(instance.panel,instance.ackData) : '';
    if (updated && (!latest || Date.parse(updated)>Date.parse(latest))) latest = updated;
    const observed = observation(instance);
    const known = validTime(observed.date);
    const age = known ? Date.parse(displayClock())-Date.parse(observed.date) : NaN;
    const stale = Number.isFinite(age) && observed.stale > 0 && age > observed.stale*1000;
    // One visible time: when the data was observed if the producer said so, otherwise when it was published.
    const shown = known ? observed.date : updated;
    record.times = {updated, observed: known ? observed.date : '', staleAfter: observed.stale, stale, failed: !!instance?.failed};
    setText(record.updated, (stale ? 'Stale · ' : '') + relative(shown));
    if (validTime(shown)) watched.push(shown);
    record.updated.title = validTime(shown) ? exact(shown) : '';
    record.updated.setAttribute('aria-label', (stale ? 'Stale. ' : '') + (known ? 'Observed ' : 'Updated ') + relative(shown) + '. Show time details');
    record.updated.classList.toggle('stale',stale);
    record.unavailable.hidden = !(instance && instance.panel.kind === 'html' && instance.ackData?.missing);
  }
  setText($('section-updated'), latest ? relative(latest) : '');
  $('section-updated').title = latest ? exact(latest) : '';
  if (latest) watched.push(latest);
  watched.push(...refreshTimeDetails());
  scheduleTimes(watched);
}
