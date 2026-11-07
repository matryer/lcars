// Decoration: small grids of lights (6 × 3 boxes of 20 × 5px) that flicker like
// live data. The flicker is a CSS animation whose duration, delay, depth and
// easing are drawn per box from the seed, so there is no JS timer; with motion
// off the boxes keep their varied resting opacities.
function lights(r, className) {
  const grid = node('span', undefined, 'lights' + (className ? ' ' + className : ''));
  const rows = [0, 1, 2].map(() => pick(r, theme4));
  for (let i = 0; i < 18; i++) {
    const cell = node('i');
    cell.style.setProperty('--tone', `var(--${r() < .75 ? rows[Math.floor(i / 6)] : pick(r, theme4)})`);
    const rest = r() < .2 ? .2 : .5 + r() * .5;
    cell.style.setProperty('--o', String(fixed(rest)));
    if (r() < .45) {
      cell.className = 'f';
      cell.style.setProperty('--lo', String(fixed(rest * (.08 + r() * .4))));
      cell.style.setProperty('--d', fixed(.5 + r() * 4) + 's');
      cell.style.setProperty('--t', fixed(-r() * 5) + 's');
      if (r() < .35) cell.style.setProperty('--ease', 'steps(2,jump-none)');
    }
    grid.append(cell);
  }
  return grid;
}
// Some panels get a light grid as a black cut-out in their top bar, between the
// heading and the Expand control, shown only while it clears the heading.
const panelLightsWidth = 6 * 20 + 5 * 4 + 2 * 8;
function placePanelLights(panel) {
  const grid = panel.querySelector(':scope > .panel-lights'); if (!grid) return;
  const heading = panel.querySelector(':scope > .panel-heading'), expand = panel.querySelector(':scope > .panel-expand');
  const box = panel.getBoundingClientRect(), pill = expand?.offsetParent ? expand.getBoundingClientRect() : null;
  const right = Math.round(pill?.width ? box.right - pill.left + 14 : 40);
  grid.style.right = right + 'px';
  grid.hidden = box.right - right - panelLightsWidth - 20 < (heading ? heading.getBoundingClientRect().right : box.right);
}
// Idempotent; 40-panels.js calls it for every panel wrapper in every pooled section.
function addPanelLights(panel) {
  if (panel.querySelector(':scope > .panel-lights') || !panel.dataset.panelId) return;
  const r = random('lights:' + panel.dataset.panelId);
  if (r() >= .4) return;
  const grid = lights(r, 'panel-lights'); grid.setAttribute('aria-hidden', 'true'); grid.hidden = true;
  panel.append(grid);
  fitObserver?.observe(panel); const heading = panel.querySelector(':scope > .panel-heading'); if (heading) fitObserver?.observe(heading);
}
// Scanning and flicker pause while the page is hidden (the JS drift timer already skips hidden pages).
const markHidden = () => document.documentElement.toggleAttribute('data-page-hidden', document.hidden);
document.addEventListener('visibilitychange', markHidden); markHidden();
