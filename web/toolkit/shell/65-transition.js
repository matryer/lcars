// Morph between sections: lightweight LCARS ghost frames (an elbow in the
// accent colour) fly from the old section's frames to the new section's in a
// fixed overlay, while the new real frames wait hidden; then the frames appear,
// content fades in and headings retype. Real iframes are never scaled.
const fixedTones = {orange: '#FF9900', critical: '#FF6666'};
// The accent a frame will settle on in a theme (the page's own colours transition, so read the table).
function accentColour(element, theme) {
  const name = /--([a-z]+)/.exec(element.style.getPropertyValue('--accent'))?.[1] || 'lilac';
  return themes[theme]?.[name] || fixedTones[name] || themes[theme]?.lilac || '#CC99CC';
}
const byReadingOrder = (a, b) => Math.round(a.rect.top / 8) - Math.round(b.rect.top / 8) || a.rect.left - b.rect.left;
// The frames of a container that are in view, in reading order.
function frameRects(container, theme) {
  const result = [];
  for (const element of container.children) {
    if (!element.matches('.panel, .deco') || element.classList.contains('lifted')) continue;
    const rect = element.getBoundingClientRect();
    if (rect.width < 1 || rect.height < 1 || rect.bottom < 0 || rect.top > innerHeight || rect.right < 0 || rect.left > innerWidth) continue;
    result.push({rect, colour: accentColour(element, theme), key: element.dataset.panelId || ''});
  }
  return result.sort(byReadingOrder);
}
let ghostLayers = 0;
// Fly ghost frames between pairs of {rect, colour}; a missing end grows in or shrinks away.
function flyGhosts(pairs, {duration = 480, stagger = true, z = 'morph'} = {}) {
  const layer = node('div', undefined, 'morph-layer' + (z === 'expand' ? ' over-expand' : '')); layer.setAttribute('aria-hidden', 'true');
  document.body.append(layer);
  const box = ({left, top, width, height}, colour, opacity) => ({left: left + 'px', top: top + 'px', width: width + 'px', height: height + 'px', color: colour, opacity});
  const small = rect => ({left: rect.left, top: rect.top, width: Math.max(48, rect.width * 0.4), height: Math.max(36, Math.min(rect.height, 120) * 0.5)});
  const layerId = ++ghostLayers;
  const animations = pairs.map(({from, to}, index) => {
    const ghost = node('div', undefined, 'morph-ghost'); layer.append(ghost);
    const frames = from && to ? [box(from.rect, from.colour, 1), box(to.rect, to.colour, 1)]
      : to ? [box(small(to.rect), to.colour, 0), box(to.rect, to.colour, 1)]
      : [box(from.rect, from.colour, 1), box(small(from.rect), from.colour, 0)];
    const delay = stagger ? entryDelay('ghost:' + layerId + ':' + index, 90) : 0;
    return ghost.animate(frames, {duration, delay, easing: 'cubic-bezier(.55,0,.15,1)', fill: 'both'});
  });
  const finished = Promise.all(animations.map(animation => animation.finished)).then(() => true, () => false);
  return {layer, animations, finished, remove() { for (const animation of animations) animation.cancel(); layer.remove(); }};
}
// Headings (and tile codes) type in again after a morph.
function retype(element, delay = 0) {
  const text = element.dataset.text ?? element.textContent;
  delete element.dataset.text;
  typeText(element, text, delay);
}
let morph, morphReveal;
// Where the old frames are now: the running morph's ghosts, or the current container.
function morphSources() {
  if (morph) {
    return [...morph.flight.layer.children].map(ghost => ({rect: ghost.getBoundingClientRect(), colour: getComputedStyle(ghost).color, opacity: Number(getComputedStyle(ghost).opacity)}))
      .filter(item => item.opacity > 0.3 && item.rect.width > 1 && item.rect.bottom > 0 && item.rect.top < innerHeight).sort(byReadingOrder);
  }
  const entry = pool.get(section);
  return entry ? frameRects(entry.container, themeName) : [];
}
function endMorph(reveal) {
  if (!morph) return;
  const {flight, entry, timer} = morph;
  morph = undefined;
  clearTimeout(timer); flight.remove();
  entry.container.classList.remove('morphing');
  if (!reveal || !motionOn() || entry.id !== section) return;
  clearTimeout(morphReveal?.timer);
  morphReveal?.container.classList.remove('morph-in');
  entry.container.classList.add('morph-in');
  morphReveal = {container: entry.container, timer: setTimeout(() => entry.container.classList.remove('morph-in'), 450)};
  entry.records.forEach((record, index) => retype(record.heading, 40 + Math.min(index, 8) * 25));
  for (const tile of entry.tiles.values()) { const code = tile.querySelector('.deco-code'); if (code) retype(code, 60); }
}
function startMorph(before, entry) {
  const after = frameRects(entry.container, themeName);
  if (!before.length && !after.length) return;
  entry.container.classList.add('morphing');
  const pairs = Array.from({length: Math.max(before.length, after.length)}, (_, index) => ({from: before[index], to: after[index]}));
  const flight = flyGhosts(pairs, {duration: 480});
  const current = {flight, entry};
  current.timer = setTimeout(() => { if (morph === current) endMorph(true); }, 1400);
  morph = current;
  flight.finished.then(done => { if (done && morph === current) endMorph(true); });
}
