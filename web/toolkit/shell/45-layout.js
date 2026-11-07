// Mosaic layout: panels pack into the shortest columns of a fine-row grid, and
// host decoration tiles fill what is left so every column finishes level.
// Placement is explicit (grid-column/grid-row); DOM order is never changed here,
// because moving a panel wrapper would reload its frame.
const TILE_MIN = 120, TILE_MAX = 440;
function columnCount(container) { return getComputedStyle(container).gridTemplateColumns.split(' ').filter(value => /px$/.test(value)).length; }
// A panel's own height, whatever its grid area: the frame top to the end of its body.
function naturalHeight(record, padding) { return record.body.getBoundingClientRect().bottom - record.wrapper.getBoundingClientRect().top + padding; }
function spanOf(record, columns) { const size = record.wrapper.dataset.size; return size === 'full' ? columns : Math.min(size === 'wide' ? 2 : 1, columns); }
function place(element, column, row) {
  if (element.style.gridColumn !== column) element.style.gridColumn = column;
  if (element.style.gridRow !== row) element.style.gridRow = row;
}
// One column (or nothing to lay out): plain flow, no tiles.
function flatten(entry) {
  entry.container.classList.remove('mosaic');
  for (const record of entry.records) for (const element of [record.wrapper, record.placeholder]) if (element) { element.style.gridColumn = ''; element.style.gridRow = ''; }
  for (const tile of entry.tiles.values()) tile.remove();
  entry.tiles.clear();
}
// Pack items (span, rows) into columns; returns placements and the holes to tile.
function pack(items, columns, minRows) {
  const heights = new Array(columns).fill(0), last = new Array(columns).fill(-1), placed = [], holes = [];
  // A small hole is absorbed by stretching the item above it, if nothing sits on that item yet.
  const stretch = (column, to) => {
    const index = last[column]; if (index < 0) return false;
    const item = placed[index];
    for (let k = item.column; k < item.column + item.span; k++) if (heights[k] !== item.end) return false;
    item.end = to; for (let k = item.column; k < item.column + item.span; k++) heights[k] = to;
    return true;
  };
  const hole = (column, from, to) => { if (to - from >= minRows) holes.push({column, from, to}); else stretch(column, to); };
  for (const {span, rows} of items) {
    let best;
    for (let column = 0; column + span <= columns; column++) {
      const slice = heights.slice(column, column + span), start = Math.max(...slice);
      const score = start + slice.reduce((sum, height) => sum + start - height, 0) / 2;
      // Near-ties go to the leftmost position, so reading order follows DOM order.
      if (!best || score < best.score - 2) best = {column, start, score};
    }
    const item = {column: best.column, span, start: best.start, end: best.start + rows};
    for (let k = item.column; k < item.column + span; k++) if (heights[k] < item.start) hole(k, heights[k], item.start);
    item.start = Math.max(item.start, ...heights.slice(item.column, item.column + span));
    item.end = item.start + rows;
    placed.push(item);
    for (let k = item.column; k < item.column + span; k++) { heights[k] = item.end; last[k] = placed.length - 1; }
  }
  // Finish level: short columns get a tile; if a sliver can't be absorbed, add a tile row everywhere.
  let bottom = Math.max(...heights);
  const slivers = heights.map((height, column) => [column, bottom - height]).filter(([, deficit]) => deficit > 0 && deficit < minRows);
  const saved = heights.slice(), savedEnds = placed.map(item => item.end);
  if (!slivers.every(([column]) => stretch(column, bottom))) {
    heights.splice(0, columns, ...saved); placed.forEach((item, index) => { item.end = savedEnds[index]; });
    bottom += minRows;
  }
  for (let column = 0; column < columns; column++) if (heights[column] < bottom) holes.push({column, from: heights[column], to: bottom});
  // Neighbouring columns with the same hole share one wide tile.
  holes.sort((a, b) => a.from - b.from || a.to - b.to || a.column - b.column);
  const tiles = [];
  for (const item of holes) {
    const previous = tiles.at(-1);
    if (previous && previous.from === item.from && previous.to === item.to && previous.column + previous.span === item.column) previous.span++;
    else tiles.push({column: item.column, span: 1, from: item.from, to: item.to});
  }
  return {placed, tiles};
}
// Tall holes become a stack of tiles of uneven, stable heights.
function splitTile(tile, maxRows, minRows, seed) {
  const rows = tile.to - tile.from, count = Math.ceil(rows / maxRows);
  if (count < 2) return [tile];
  const r = random(seed), weights = Array.from({length: count}, () => 0.75 + r() * 0.5), total = weights.reduce((a, b) => a + b, 0);
  const pieces = []; let from = tile.from;
  weights.forEach((weight, index) => {
    const size = index === count - 1 ? tile.to - from : Math.max(minRows, Math.round(rows * weight / total));
    pieces.push({...tile, from, to: from + size}); from += size;
  });
  return pieces;
}
function layoutMosaic(entry = pool.get(section)) {
  if (!entry) return;
  const container = entry.container;
  const columns = columnCount(container);
  const records = entry.records.filter(record => record.wrapper.isConnected);
  if (columns < 2 || !records.length) { flatten(entry); return; }
  // Read everything first, then write, so a layout costs at most one forced reflow.
  const style = getComputedStyle(container);
  const unit = parseFloat(style.getPropertyValue('--mosaic-row')) || 8, gap = parseFloat(style.getPropertyValue('--mosaic-gap')) || 32;
  const padding = parseFloat(getComputedStyle(records[0].wrapper).paddingBottom) || 0;
  const rowsFor = height => Math.max(1, Math.ceil((height + gap) / unit));
  const items = records.map(record => ({span: spanOf(record, columns), rows: rowsFor(record.lifted ? record.liftHeight : naturalHeight(record, padding))}));
  const minRows = rowsFor(TILE_MIN), maxRows = rowsFor(TILE_MAX);
  const {placed, tiles} = pack(items, columns, minRows);
  container.classList.add('mosaic');
  placed.forEach((item, index) => place(records[index].placeholder || records[index].wrapper, `${item.column + 1} / span ${item.span}`, `${item.start + 1} / ${item.end + 1}`));
  const next = new Map(), counts = new Map();
  for (const tile of tiles) {
    const key = tile.column + ':' + tile.span, ordinal = counts.get(key) || 0; counts.set(key, ordinal + 1);
    const base = `deco:${entry.id}:${key}:${ordinal}`;
    splitTile(tile, maxRows, minRows, base).forEach((piece, index) => {
      const seed = base + ':' + index;
      let element = entry.tiles.get(seed) || next.get(seed);
      if (!element) {
        element = decoration(seed, piece.span);
        if (booted && motionOn() && entry.id === section) enterTile(element, seed);
        container.append(element);
      }
      next.set(seed, element);
      place(element, `${piece.column + 1} / span ${piece.span}`, `${piece.from + 1} / ${piece.to + 1}`);
      const height = (piece.to - piece.from) * unit - gap + 'px';
      if (element.style.height !== height) element.style.height = height;
    });
  }
  for (const [seed, element] of entry.tiles) if (!next.has(seed)) element.remove();
  entry.tiles = next;
}
function enterTile(element, seed) { element.classList.add('enter'); element.style.animationDelay = entryDelay('tile:' + seed) + 'ms'; element.addEventListener('animationend', () => element.classList.remove('enter'), {once: true}); }
// Heights change as slots animate: a ResizeObserver delivers at most one batch
// per rendering frame, after layout, so spans update in the same frame.
const layoutObserver = window.ResizeObserver ? new ResizeObserver(changes => {
  const entry = pool.get(section);
  if (entry && changes.some(change => entry.container.contains(change.target))) layoutMosaic(entry);
}) : undefined;
function observeLayout(element) { layoutObserver?.observe(element); }
function unobserveLayout(element) { layoutObserver?.unobserve(element); }
if (!layoutObserver) addEventListener('resize', () => requestAnimationFrame(() => layoutMosaic()));
