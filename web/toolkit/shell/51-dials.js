// Decoration: radial dials and crosshair button clusters, drawn in SVG.
// Each fills and centres in whatever box it is given (viewBox + meet); wide or
// tall tiles add a second figure and a side column through container queries.
const SVGNS = 'http://www.w3.org/2000/svg';
function svgNode(tag, attributes = {}, className) {
  const element = document.createElementNS(SVGNS, tag);
  for (const [key, value] of Object.entries(attributes)) element.setAttribute(key, String(value));
  if (className) element.setAttribute('class', className);
  return element;
}
const theme4 = ['apricot','lilac','blue','peach'];
const fixed = value => Math.round(value * 100) / 100;
// Angles in degrees, clockwise from twelve o'clock, around (100, 100).
const point = (radius, angle) => { const a = angle * Math.PI / 180; return fixed(100 + radius * Math.sin(a)) + ' ' + fixed(100 - radius * Math.cos(a)); };
function sector(inner, outer, from, to) {
  const large = to - from > 180 ? 1 : 0;
  return `M${point(outer, from)}A${outer} ${outer} 0 ${large} 1 ${point(outer, to)}L${point(inner, to)}A${inner} ${inner} 0 ${large} 0 ${point(inner, from)}Z`;
}
// A segment button: theme tone, sometimes dimmed; lights when clicked.
function arcButton(r, d, tone) {
  const path = svgNode('path', {d}, 'arc');
  path.style.setProperty('--tone', `var(--${tone})`);
  if (r() < .14) path.style.setProperty('--a', '.62');
  return path;
}
function svgFigure(className) {
  return svgNode('svg', {viewBox: '0 0 200 200', preserveAspectRatio: 'xMidYMid meet', focusable: 'false'}, 'figure ' + className);
}
// Concentric rings split into arc segments with black gaps, a central readout
// and a radar sweep; the innermost tick ring turns slowly.
function dial(r) {
  const figure = svgFigure('dial');
  const spin = svgNode('g', {}, 'dial-spin'); spin.style.setProperty('--spin-d', (50 + Math.floor(r() * 60)) + 's');
  let outer = 97, tone = pick(r, theme4);
  const rings = 2 + Math.floor(r() * 2);
  for (let ring = 0; ring < rings; ring++) {
    const thickness = ring === 0 ? 10 + Math.floor(r() * 6) : 9 + Math.floor(r() * 8), inner = outer - thickness;
    const count = pick(r, ring === 0 ? [8, 10, 12, 16, 18] : [4, 6, 8, 10, 12]);
    // Some rings are open arcs: a span of buttons with the rest left dark.
    const span = r() < .35 ? 180 + Math.floor(r() * 7) * 20 : 360, start = span < 360 ? 180 - span / 2 + pick(r, [0, 180, 90]) : r() * 360 / count;
    const gap = 3.4 / ((inner + outer) / 2) * 180 / Math.PI, step = span / count;
    for (let i = 0; i < count; i++) {
      if (r() < .35) tone = pick(r, theme4);
      if (r() < .07) continue;
      const from = start + i * step + gap / 2, to = start + (i + 1) * step - gap / 2;
      figure.append(arcButton(r, sector(inner, outer, from, to), tone));
    }
    outer = inner - 4 - Math.floor(r() * 4);
  }
  // Fine tick ring, rotating while motion is on.
  const ticks = pick(r, [24, 36, 48]), tickInner = outer - 5;
  for (let i = 0; i < ticks; i++) {
    const from = i * 360 / ticks + .8, to = (i + 1) * 360 / ticks - .8;
    const tick = svgNode('path', {d: sector(tickInner, outer, from, to)}, 'tick-arc');
    tick.style.setProperty('--tone', `var(--${i % 6 === 0 ? 'apricot' : pick(r, theme4)})`);
    tick.style.opacity = String(i % 3 === 0 ? 1 : .35 + r() * .3);
    spin.append(tick);
  }
  figure.append(spin);
  const core = tickInner - 5;
  figure.append(svgNode('circle', {cx: 100, cy: 100, r: core}, 'dial-core'));
  // Radar sweep: a fading wedge led by a bright line, over the rings.
  const sweep = svgNode('g', {}, 'dial-sweep'); sweep.style.setProperty('--sweep-d', (5 + Math.floor(r() * 5)) + 's');
  for (let i = 0; i < 6; i++) { const wedge = svgNode('path', {d: sector(core + 1, 98, -(i + 1) * 7, -i * 7)}); wedge.style.opacity = String(fixed(.2 * (1 - i / 6))); sweep.append(wedge); }
  sweep.append(svgNode('path', {d: sector(core + 1, 98, -.6, .6)}, 'sweep-edge'));
  figure.append(sweep);
  const valueTone = pick(r, theme4);
  const value = svgNode('text', {x: 100, y: 100 + core * .14, 'font-size': fixed(core * .62)}, 'dial-value'); value.textContent = digits(r, 2, 3);
  value.style.setProperty('--tone', `var(--${valueTone})`);
  const label = svgNode('text', {x: 100, y: 100 + core * .5, 'font-size': fixed(core * .2)}, 'dial-label'); label.textContent = codeText(r);
  figure.append(value, label);
  return figure;
}
// Plus-shaped clusters: four arms of blocks ending in D-shaped buttons with
// black labels, quadrant arcs between them and a slowly turning outer ring.
function cross(r) {
  const figure = svgFigure('cross');
  const spin = svgNode('g', {}, 'dial-spin'); spin.style.setProperty('--spin-d', (40 + Math.floor(r() * 40)) + 's');
  spin.append(svgNode('circle', {cx: 100, cy: 100, r: 88, 'stroke-dasharray': pick(r, ['2 5', '1 4', '6 4'])}, 'dial-dash'));
  for (let i = 0; i < 4; i++) { const mark = svgNode('path', {d: sector(79, 84, i * 90 + 38, i * 90 + 52)}, 'tick-arc'); mark.style.setProperty('--tone', `var(--${pick(r, theme4)})`); spin.append(mark); }
  figure.append(spin);
  const width = 26 + Math.floor(r() * 3) * 2, half = width / 2, length = 14, gap = 4, first = 22;
  const blocks = r() < .5 ? 3 : 2;
  for (let arm = 0; arm < 4; arm++) {
    const tone = pick(r, theme4), group = svgNode('g', {transform: `rotate(${arm * 90} 100 100)`});
    for (let k = 0; k < blocks; k++) {
      const near = first + k * (length + gap), far = near + length, last = k === blocks - 1;
      // Arm pointing up: y = 100 - distance. The last block ends in a half disc.
      const d = last
        ? `M${100 - half} ${100 - near}V${100 - far}A${half} ${half} 0 0 1 ${100 + half} ${100 - far}V${100 - near}Z`
        : `M${100 - half} ${100 - near}V${100 - far}H${100 + half}V${100 - near}Z`;
      group.append(arcButton(r, d, k && r() < .3 ? pick(r, theme4) : tone));
      if (last) {
        // Labels stay upright whichever way the arm points.
        const angle = arm * 90, distance = far + half * .35, [x, y] = point(distance, angle).split(' ').map(Number);
        const text = svgNode('text', {x, y: fixed(y + 3.2), 'font-size': 9, transform: `rotate(${-angle} ${x} ${y})`}, 'cross-label'); text.textContent = digits(r, 2, 3);
        group.append(text);
      }
    }
    figure.append(group);
  }
  const reach = first + blocks * (length + gap) - gap;
  for (let q = 0; q < 4; q++) {
    const centre = q * 90 + 45, tone = pick(r, theme4);
    const inner = Math.max(34, half / Math.sin(30 * Math.PI / 180) + 2), outer = Math.min(reach + 4, inner + 22);
    const spread = Math.asin(Math.min(.95, (half + 5) / inner)) * 180 / Math.PI;
    figure.append(arcButton(r, sector(inner, outer, q * 90 + spread, (q + 1) * 90 - spread), tone));
    if (r() < .6) figure.append(arcButton(r, sector(outer + 4, outer + 10, centre - 18, centre + 18), pick(r, theme4)));
  }
  figure.append(arcButton(r, `M100 ${100 - first + 6}A${first - 6} ${first - 6} 0 1 1 99.99 ${100 - first + 6}Z`, pick(r, theme4)));
  // A sweep that circles the cluster between arms.
  const sweep = svgNode('g', {}, 'dial-sweep'); sweep.style.setProperty('--sweep-d', (7 + Math.floor(r() * 5)) + 's');
  for (let i = 0; i < 5; i++) { const wedge = svgNode('path', {d: sector(first - 4, 96, -(i + 1) * 6, -i * 6)}); wedge.style.opacity = String(fixed(.16 * (1 - i / 5))); sweep.append(wedge); }
  figure.append(sweep);
  return figure;
}
// The side column: a light grid, a short readout and D-shaped keys.
function dialSide(r) {
  const side = node('span', undefined, 'dial-side');
  side.append(lights(r, 'side-lights'));
  const value = node('span', undefined, 'side-readout');
  for (let i = 0; i < 2; i++) value.append(block(r, 'number', digits(r, 2, 4)));
  side.append(value, keyColumn(r, 3 + Math.floor(r() * 3), false));
  return side;
}
function figures(r, element, first, second) {
  element.classList.add('dials');
  const lead = r() < .5;
  element.append(first(r), second(r), dialSide(r));
  element.lastChild.classList.toggle('lead', lead);
  element.children[1].classList.add('extra');
}
function dials(r, element) { figures(r, element, dial, r() < .6 ? cross : dial); }
function crosshair(r, element) { figures(r, element, cross, dial); }
