// Decoration fit: tiles never show a control cut by their edge. Containers
// marked data-fit drop the children that would be clipped by the tile body:
//   trail: hide the first child that crosses the bottom or right edge and every child after it;
//   top:   hide leading children until the last one fits (meters lose cells from the top);
//   each:  hide each child that does not fit on its own.
// One shared ResizeObserver refits whenever a tile, panel or heading changes
// size; its callback runs once per frame, before paint, so a resize never
// shows a clipped frame. The visible set depends only on the tile's size.
const crosses = (element, clip) => {
  const box = element.getBoundingClientRect();
  return (box.width || box.height) && (box.bottom > clip.bottom + .5 || box.right > clip.right + .5);
};
function cut(element) { element.hidden = true; element.dataset.cut = ''; }
function fitDeco(body) {
  for (const element of body.querySelectorAll('[data-cut]')) { element.hidden = false; delete element.dataset.cut; }
  const clip = body.getBoundingClientRect();
  if (!clip.width || !clip.height) return;
  for (const group of [body, ...body.querySelectorAll('[data-fit]')]) {
    const mode = group.dataset.fit; if (!mode || group.closest('[data-cut]')) continue;
    const items = [...group.children].filter(element => !element.hidden);
    if (mode === 'trail') { const at = items.findIndex(element => crosses(element, clip)); if (at >= 0) items.slice(at).forEach(cut); }
    else if (mode === 'each') items.filter(element => crosses(element, clip)).forEach(cut);
    else if (mode === 'top') { const last = items.at(-1); for (const element of items.slice(0, -1)) { if (!last || !crosses(last, clip)) break; cut(element); } }
  }
  // The heading light grid must clear the tile code and the tile's right edge.
  const tile = body.parentElement, grid = tile?.querySelector(':scope > .deco-lights'), code = tile?.querySelector(':scope > .deco-code');
  if (grid) {
    grid.hidden = false;
    const box = grid.getBoundingClientRect(), frame = tile.getBoundingClientRect();
    grid.hidden = box.right > frame.right - 8 || (!!code && box.left < code.getBoundingClientRect().right + 16);
  }
}
function refit(target) {
  if (target.classList.contains('deco-body')) fitDeco(target);
  else placePanelLights(target.closest('.panel'));
}
const fitObserver = window.ResizeObserver ? new ResizeObserver(entries => { for (const entry of entries) { if (entry.target.isConnected) refit(entry.target); else fitObserver.unobserve(entry.target); } }) : null;
// Label widths change when the font arrives, without changing any tile's size.
document.fonts?.addEventListener?.('loadingdone', () => {
  for (const body of $('main').querySelectorAll('.deco-body')) fitDeco(body);
  for (const panel of $('main').querySelectorAll('.panel')) placePanelLights(panel);
});
