// Expand: lift a panel into a large overlay without moving its DOM node (moving
// an iframe reloads it). The wrapper becomes position:fixed over a dimmed
// backdrop; a same-size placeholder holds its place in the grid. The frame
// gets the larger width, so its content reflows and the slot animates as usual.
let lifted, closing;
// A close still animating finishes at once (before another open, a navigation or a capture).
function settleClosing() { const pending = closing; closing = undefined; pending?.(); }
// Everything outside the path from body to the element becomes inert.
function inertOutside(element, keep) {
  const changed = [];
  for (let current = element; current && current !== document.body; current = current.parentElement) {
    for (const sibling of current.parentElement?.children || []) {
      if (sibling === current || keep.includes(sibling) || sibling.hasAttribute('inert') || sibling.tagName === 'SCRIPT') continue;
      sibling.setAttribute('inert', ''); changed.push(sibling);
    }
  }
  return changed;
}
function setExpandControl(record, open) {
  const title = record.latest.title || 'panel';
  setText(record.expand, open ? 'Close' : 'Expand');
  record.expand.setAttribute('aria-pressed', String(open));
  record.expand.setAttribute('aria-label', (open ? 'Close ' : 'Expand ') + title);
}
function openExpand(record) {
  if (lifted) closeExpand(true);
  settleClosing();
  const wrapper = record.wrapper;
  const animate = motionOn() && typeof wrapper.animate === 'function';
  const from = wrapper.getBoundingClientRect();
  const padding = parseFloat(getComputedStyle(wrapper).paddingBottom) || 0;
  record.liftHeight = naturalHeight(record, padding);
  // The placeholder takes the wrapper's grid area (mosaic) or its place in the flow.
  const placeholder = node('div', undefined, 'panel-placeholder'); placeholder.setAttribute('aria-hidden', 'true');
  placeholder.style.height = from.height + 'px'; placeholder.style.gridColumn = wrapper.style.gridColumn; placeholder.style.gridRow = wrapper.style.gridRow;
  wrapper.after(placeholder);
  const backdrop = node('div', undefined, 'expand-backdrop'); backdrop.setAttribute('aria-hidden', 'true');
  backdrop.addEventListener('click', () => closeExpand());
  document.body.append(backdrop);
  record.placeholder = placeholder; record.lifted = true;
  wrapper.classList.remove('enter');
  wrapper.classList.add('lifted');
  wrapper.setAttribute('role', 'dialog'); wrapper.setAttribute('aria-modal', 'true'); wrapper.setAttribute('aria-labelledby', record.heading.id);
  lifted = {record, placeholder, backdrop, inert: inertOutside(wrapper, [backdrop])};
  setExpandControl(record, true);
  record.expand.focus({preventScroll: true});
  if (animate) {
    const to = wrapper.getBoundingClientRect(), colour = accentColour(wrapper, themeName);
    wrapper.classList.add('lifting');
    backdrop.animate([{opacity: 0}, {opacity: 1}], {duration: 260, easing: 'ease-out'});
    const flight = flyGhosts([{from: {rect: from, colour}, to: {rect: to, colour}}], {duration: 420, z: 'expand'});
    lifted.flight = flight;
    flight.finished.then(() => {
      if (lifted?.record !== record) return;
      flight.remove(); lifted.flight = undefined;
      wrapper.classList.remove('lifting'); wrapper.classList.add('lift-in');
      retype(record.heading);
      setTimeout(() => wrapper.classList.remove('lift-in'), 400);
    });
  }
  layoutMosaic();
}
function closeExpand(instant = false) {
  if (instant) settleClosing();
  if (!lifted) return;
  const {record, placeholder, backdrop, inert, flight} = lifted;
  lifted = undefined;
  flight?.remove();
  for (const element of inert) element.removeAttribute('inert');
  const wrapper = record.wrapper;
  setExpandControl(record, false);
  const hadFocus = wrapper.contains(document.activeElement) || document.activeElement === document.body;
  let done = false;
  const finish = () => {
    if (done) return; done = true;
    if (closing === finish) closing = undefined;
    wrapper.classList.remove('lifted', 'lifting', 'lift-in');
    wrapper.removeAttribute('role'); wrapper.removeAttribute('aria-modal'); wrapper.removeAttribute('aria-labelledby');
    placeholder.remove(); backdrop.remove();
    record.placeholder = undefined; record.lifted = false;
    layoutMosaic();
  };
  if (!instant && motionOn() && typeof wrapper.animate === 'function' && placeholder.isConnected && record.section === section) {
    const from = wrapper.getBoundingClientRect(), to = placeholder.getBoundingClientRect(), colour = accentColour(wrapper, themeName);
    wrapper.classList.add('lifting');
    backdrop.animate([{opacity: 1}, {opacity: 0}], {duration: 300, easing: 'ease-in', fill: 'forwards'});
    const back = flyGhosts([{from: {rect: from, colour}, to: {rect: to, colour}}], {duration: 380, z: 'expand'});
    closing = () => { back.remove(); finish(); };
    back.finished.then(() => { if (!done) { back.remove(); finish(); } });
  } else finish();
  if (hadFocus) record.expand.focus({preventScroll: true});
}
document.addEventListener('keydown', event => {
  if (lifted && event.key === 'Escape' && !event.defaultPrevented) { event.preventDefault(); closeExpand(); }
});
