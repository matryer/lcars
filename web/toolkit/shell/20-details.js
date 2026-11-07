// In-panel detail views (breadcrumb and Back on the panel frame) and the panel time details row.
// A frame draws its own detail view; the host only mirrors it: "TITLE › DETAIL" in the frame
// heading and a Back control on the top bar that asks the frame to go back.
function detailTitle(raw) {
  if (raw === null) return null;
  if (typeof raw !== 'string' || raw.length > 4000) return;
  const title = raw.trim().slice(0,200);
  return title || undefined;
}
function detailParts(record) {
  if (record.detail) return record.detail;
  const box = record.heading.parentElement;
  const back = node('button',undefined,'panel-back'); back.type = 'button'; back.hidden = true;
  const arrow = node('span','◄'); arrow.setAttribute('aria-hidden','true');
  back.append(arrow,node('span','Back'));
  const crumb = node('span',undefined,'panel-crumb'); crumb.hidden = true;
  const separator = node('span','›','crumb-separator'); separator.setAttribute('aria-hidden','true');
  const title = node('span',undefined,'crumb-title');
  crumb.append(separator,title);
  box.insertBefore(back,record.heading);
  record.heading.after(crumb);
  back.addEventListener('click',()=>{
    const instance = record.display;
    if (!instance) return;
    // Focus the frame first, so it may return focus to the element that opened the detail.
    instance.frame.focus({preventScroll:true});
    post(instance,'back');
    if (instance.failed) { instance.detailTitle = undefined; syncDetail(record); }
  });
  record.detail = {back,crumb,title,shown:''};
  return record.detail;
}
// Mirror the displayed frame's detail state; anything else (a replaced or failed frame) clears it.
function syncDetail(record) {
  const instance = record.display;
  const title = instance && !instance.failed ? instance.detailTitle : undefined;
  if (!title && !record.detail) return;
  const parts = detailParts(record);
  if (!title) {
    if (!parts.shown) return;
    parts.shown = ''; parts.back.hidden = true; parts.crumb.hidden = true; parts.title.dataset.text = ''; parts.title.textContent = '';
    record.wrapper.classList.remove('detail-open');
    if (document.activeElement === parts.back) instance?.frame.focus({preventScroll:true});
    return;
  }
  const opening = !parts.shown;
  parts.shown = title; parts.back.hidden = false; parts.crumb.hidden = false;
  parts.back.setAttribute('aria-label','Back to '+(record.latest.title || 'panel'));
  typeText(parts.title,title);
  record.wrapper.classList.add('detail-open');
  if (opening && record.heading.parentElement.getBoundingClientRect().top < 0) record.heading.parentElement.scrollIntoView({block:'start',behavior:motionOn()?'smooth':'auto'});
}
function frameDetail(instance, raw) {
  const title = detailTitle(raw);
  if (title === undefined) return;
  instance.detailTitle = title || undefined;
  if (instance.record.display === instance) syncDetail(instance.record);
}
// The panel time button toggles an inline row of exact times in the panel meta area.
let timeRows = 0;
function prepareTimes(record) {
  if (record.timeDetails) return record.timeDetails;
  const row = node('div',undefined,'time-details'); row.id = 'time-details-'+(++timeRows); row.hidden = true;
  record.updated.removeAttribute('aria-haspopup');
  record.updated.setAttribute('aria-expanded','false');
  record.updated.setAttribute('aria-controls',row.id);
  record.updated.parentElement.append(row);
  record.timeDetails = row;
  return row;
}
function renderTimes(record) {
  const row = record.timeDetails, times = record.times || {};
  const list = node('dl');
  const add = (label, time, fallback) => {
    const value = node('dd');
    if (time) {
      const exactTime = node('time',exact(time)); exactTime.dateTime = time;
      const ago = node('span',compact(time),'muted'); ago.dataset.relative = time;
      value.append(exactTime,' · ',ago);
    } else value.textContent = fallback;
    list.append(node('dt',label),value);
  };
  add(times.failed ? 'Last verified update' : 'Updated',times.updated,'Unknown');
  add('Observed',times.observed,'Unknown');
  if (times.observed && times.staleAfter > 0) add('Stale after',undefined,times.staleAfter+' seconds'+(times.stale ? ' · Stale now' : ''));
  row.replaceChildren(list,node('p','Updated is when this panel’s document, resources or live data last changed on disk. Observed is when the producer says the data was measured; LCARS never invents it.'));
}
function showTimes(record) {
  const row = prepareTimes(record), open = row.hidden;
  if (open) renderTimes(record);
  row.hidden = !open;
  record.updated.setAttribute('aria-expanded',String(open));
  updateTimes();
}
// Refresh every open time details row from record.times (set by updateTimes) and
// return the times it shows, so the caller can schedule the next change.
function refreshTimeDetails() {
  const shown = [];
  for (const record of panels.values()) {
    if (!record.timeDetails || record.timeDetails.hidden) continue;
    renderTimes(record);
    for (const element of record.timeDetails.querySelectorAll('[data-relative]')) shown.push(element.dataset.relative);
  }
  return shown;
}
function closeTimeDetails() {
  for (const record of panels.values()) if (record.timeDetails && !record.timeDetails.hidden) { record.timeDetails.hidden = true; record.updated.setAttribute('aria-expanded','false'); }
}
