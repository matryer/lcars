// Panel records, navigation and sub-navigation links, and apply() for each state.
let headingIds = 0;
function makePanel(panel, sectionId) {
  const wrapper=node('section',undefined,'panel');wrapper.dataset.panelId=panel.id;wrapper.dataset.size=panel.size || 'normal';wrapper.dataset.accent=panel.accent || 'lilac';wrapper.style.setProperty('--accent',palette[panel.accent]||palette.lilac);
  const headingBox=node('div',undefined,'panel-heading');const heading=node('h2');heading.tabIndex=-1;heading.id='panel-heading-'+(++headingIds);headingBox.append(heading);typeText(heading,panel.title);
  // Expand lifts the panel into a large overlay in place; its frame is never moved, so its state survives.
  const expand=node('button','Expand','panel-expand');expand.type='button';expand.setAttribute('aria-pressed','false');expand.setAttribute('aria-label','Expand '+panel.title);
  const body=node('div',undefined,'panel-body');const slot=node('div',undefined,'frame-slot');const problems=node('div');
  const meta=node('div',undefined,'panel-meta');const updated=node('button','—','panel-time');updated.type='button';
  const unavailable=node('p','Data unavailable');unavailable.hidden=true;
  const source=node('a','Source','source');source.target='_blank';source.rel='noopener noreferrer';source.hidden=true;
  const sourceLinks=node('div');meta.append(updated,unavailable,source,sourceLinks);body.append(slot,problems,meta);wrapper.append(headingBox,expand,body);
  const record={wrapper,heading,expand,body,slot,problems,updated,unavailable,source,sourceLinks,runtime:[],latest:panel,failedHash:'',section:sectionId};
  updated.addEventListener('click',()=>showTimes(record));
  expand.addEventListener('click',()=>{ if (lifted?.record === record) closeExpand(); else openExpand(record); });
  observeLayout(body);
  return record;
}
function markCurrent(container) {
  for (const link of container.children) {
    const id = link.dataset.tab, here = section === id || (container.id === 'navigation' && section.startsWith(id + '/'));
    if (here) link.setAttribute('aria-current','page'); else link.removeAttribute('aria-current');
  }
}
// Links build in with stable random delays, so the rail and tab bar assemble organically.
function tabLinks(container, tabs) {
  const scope = container.id === 'navigation' ? 'nav:' : 'subnav:' + (tabs[0]?.parent || tabs[0]?.id || '') + ':';
  container.replaceChildren(...tabs.map((tab, index) => {
    const link = node('a', undefined); link.href = tabHref(tab.id); link.dataset.tab = tab.id;
    if (container.id === 'navigation' && index < 9) { link.title = `${tab.title || tab.id} (${index + 1})`; link.setAttribute('aria-keyshortcuts', String(index + 1)); }
    const delay = motionOn() ? entryDelay(scope + tab.id) : 0;
    typeText(link, tab.title || tab.id, delay);
    if (motionOn()) { link.classList.add('enter'); link.style.animationDelay = delay + 'ms'; }
    return link;
  }));
}
// A section with only sub-sections opens its first sub-section.
function landing(id, tabs = state?.tabs || [], list = state?.panels || []) {
  const tab = tabs.find(item => item.id === id);
  if (!tab || tab.parent || list.some(panel => panel.tab === id)) return id;
  return tabs.find(item => item.parent === id)?.id || id;
}
// Bring one section's records in line with the state: create, update and stage
// panels, remove those that disappeared, and keep DOM order within its container.
function syncSection(entry, list) {
  const ids = new Set(list.map(panel => panel.id));
  for (const record of entry.records) if (!ids.has(record.latest.id) && panels.get(record.latest.id) === record) removeRecord(record);
  entry.records = [];
  let previous;
  for (const panel of list) {
    let record = panels.get(panel.id);
    if (record && record.section !== entry.id) { removeRecord(record); record = undefined; }
    if (!record) {
      record = makePanel(panel, entry.id); panels.set(panel.id, record);
      if (entry.id === section) { holdPlaceholder(record); if (booted && !navigating) enter(record); }
    }
    entry.records.push(record);
    record.latest = panel;
    showProblems(record);
    stage(record, panel);
    // Only out-of-order wrappers move; placeholders and tiles are skipped over.
    let expected = previous ? previous.nextElementSibling : entry.container.firstElementChild;
    while (expected && !expected.classList.contains('panel')) expected = expected.nextElementSibling;
    if (expected !== record.wrapper) entry.container.insertBefore(record.wrapper, expected);
    // Decoration parts that watch only the page's first container still see every pooled panel.
    if (typeof addPanelLights === 'function') addPanelLights(record.wrapper);
    previous = record.placeholder || record.wrapper;
  }
}
function apply(next) {
  state=next;
  const tabs=next.tabs || [];
  const top=tabs.filter(tab=>!tab.parent);
  const navKey=JSON.stringify(top.map(tab=>[tab.id,tab.title]));
  if(navKey!==navigationKey){navigationKey=navKey;tabLinks($('navigation'),top);}
  markCurrent($('navigation'));
  const topId=section.split('/')[0];
  const topTab=tabs.find(tab=>tab.id===topId&&!tab.parent);
  const current=tabs.find(tab=>tab.id===section);
  const children=tabs.filter(tab=>tab.parent===topId);
  const ownPanels=(next.panels || []).some(panel=>panel.tab===topId);
  // A section with only sub-sections opens its first sub-section.
  if(current&&!current.parent&&children.length&&!ownPanels){section=children[0].id;history.replaceState(history.state,'',tabHref(section));return apply(next);}
  const subTabs=children.length?[...(ownPanels&&topTab?[topTab]:[]),...children]:[];
  const subKey=JSON.stringify(subTabs.map(tab=>[tab.id,tab.title]));
  if(subKey!==subnavKey){subnavKey=subKey;tabLinks($('subnav'),subTabs);}
  $('subnav').hidden=!subTabs.length;
  markCurrent($('subnav'));
  applyTheme(topTab);
  if(decorSeed!==topId){decorSeed=topId;chrome(topId);}
  typeText($('section-title'),topTab?.title || (section?'Section unavailable':'Your canvas is ready'));
  document.title=[topTab?.title || 'Canvas',current?.parent?current.title:''].filter(Boolean).join(' › ')+' - matryer/lcars';
  $('global-diagnostics').replaceChildren();
  for(const value of [...(next.diagnostics || []),...(current?.parent?topTab?.diagnostics || []:[]),...(current?.diagnostics || [])])$('global-diagnostics').append(node('p',diagnosticText(value),'diagnostic'));
  // Every pooled section stays in sync; sections that no longer exist are dropped.
  const known=new Set(tabs.map(tab=>tab.id));
  for(const entry of [...pool.values()])if(entry.id!==section&&!known.has(entry.id))dropSection(entry);
  const entry=showSection(section);
  const byTab=new Map();
  for(const panel of next.panels || []){if(!byTab.has(panel.tab))byTab.set(panel.tab,[]);byTab.get(panel.tab).push(panel);}
  for(const pooled of pool.values())syncSection(pooled,byTab.get(pooled.id) || []);
  const visible=entry.records.length;
  $('empty').hidden=!!visible;
  if(!visible){
    $('empty').replaceChildren();
    if(!current&&section){
      $('empty').append(node('h2','This section is unavailable'),node('p','It may have been moved or removed.'));
      if(tabs[0]){const link=node('a','Open '+tabs[0].title);link.href=tabHref(tabs[0].id);$('empty').append(link);}
    }else{
      $('empty').append(node('h2',current?'No content yet':'Create your first section'),node('p','Publish an HTML, Markdown or text panel in a section folder. LCARS will discover it automatically.'));
      const link=node('a','Read the authoring guide');link.href='/agent-guide';$('empty').append(link);
    }
  }
  layoutMosaic(entry);
  updateTimes();
  finishBoot();
  enforceBudget();
  schedulePreload();
}
