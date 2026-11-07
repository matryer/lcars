/* LCARS shell, bridge and capture contract v1. */
(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const palette = {apricot:'#FFCC99',orange:'#FF9900',lilac:'#CC99CC',blue:'#9999FF',peach:'#FF9966',critical:'#FF6666'};
  // Sections live at /s/{id} or /s/{id}/{sub}. Navigation between them stays in the page.
  const sectionFromPath = () => location.pathname.startsWith('/s/') ? location.pathname.slice(3).split('/').map(decodeURIComponent).join('/') : '';
  const tabHref = id => '/s/' + id.split('/').map(encodeURIComponent).join('/');
  let section = sectionFromPath();
  const panels = new Map(), instances = new Map();
  function freshNonce() { return Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2,'0')).join(''); }
  let state, etag = '', navigationKey = '', subnavKey = '', timer, backoff = 2000, lastCheck, polling = false, capturing = false, latched = false;
  let frozenClock, captureError;
  let eventStream, eventsHealthy = false, pendingCheck = false, eventFailures = 0;
  let storedMotion = false;
  try { storedMotion = localStorage.getItem('lcars-motion-off') === 'true'; } catch {}
  const reduced = matchMedia('(prefers-reduced-motion: reduce)');
  const motionOn = () => !storedMotion && !reduced.matches && !capturing;
  function applyMotion() { document.documentElement.dataset.motion = motionOn() ? 'on' : 'off'; if (!motionOn()) finishTyping(); }
  // Headings and labels type in quickly; with motion off they appear at once.
  const typing = new Map();
  function typeText(element, text, delay = 0) {
    text = String(text ?? '');
    if (element.dataset.text === text) return;
    element.dataset.text = text;
    cancelAnimationFrame(typing.get(element)); typing.delete(element);
    if (!motionOn() || !text) { element.textContent = text; element.classList.remove('typing'); element.removeAttribute('aria-label'); return; }
    const start = performance.now() + delay, perCharacter = Math.max(6, Math.min(16, 360 / text.length));
    element.textContent = ''; element.classList.add('typing'); element.setAttribute('aria-label', text);
    const step = now => {
      const count = Math.max(0, Math.min(text.length, Math.ceil((now - start) / perCharacter)));
      if (element.textContent.length !== count) element.textContent = text.slice(0, count);
      if (count < text.length) typing.set(element, requestAnimationFrame(step));
      else { typing.delete(element); element.classList.remove('typing'); element.removeAttribute('aria-label'); }
    };
    typing.set(element, requestAnimationFrame(step));
  }
  function finishTyping() {
    for (const [element, frame] of typing) { cancelAnimationFrame(frame); element.textContent = element.dataset.text; element.classList.remove('typing'); element.removeAttribute('aria-label'); }
    typing.clear();
  }
  const setText = (element, text) => { if (element.textContent !== text) element.textContent = text; };
  function node(tag, text, className) { const element = document.createElement(tag); if (text !== undefined) element.textContent = text; if (className) element.className = className; return element; }
  const errorText = error => String(error?.message || error || 'Unknown error').slice(0,1500);
  function diagnosticText(item) { return typeof item === 'string' ? item : item?.message || item?.error || JSON.stringify(item); }
  function displayClock() { return frozenClock || new Date().toISOString(); }
  function frameState() { return {visible: !document.hidden && !capturing, motion: !storedMotion && !reduced.matches && !capturing, capture: capturing, clock: displayClock()}; }
  function post(instance, type, values = {}) { instance.frame.contentWindow?.postMessage({v:1,nonce:instance.nonce,type,...values}, '*'); }
  function sendLifecycle() { for (const instance of instances.values()) post(instance,'state',{state:frameState()}); }
  function dispose(instance) { if (!instance) return; clearTimeout(instance.deadline); instances.delete(instance.nonce); instance.frame.remove(); }
  function dataValue(panel) { return panel.data || {hash:'missing',missing:true,envelope:null}; }
  function dataHash(panel) { return dataValue(panel).hash || 'missing'; }
  function validTime(value) { return !!value && Number.isFinite(Date.parse(value)) && new Date(value).getUTCFullYear() > 1; }
  function updatedFor(panel, data) {
    const dates = [panel.updatedAt, data?.updatedAt].filter(validTime);
    return dates.sort((a,b) => Date.parse(b)-Date.parse(a))[0] || '';
  }
  function observation(instance) {
    if (!instance) return {};
    const data = instance.ackData;
    if (data && !data.missing) return {date:data.envelope?.updatedAt,stale:data.envelope?.staleAfterSeconds};
    return {date:instance.panel.observedAt,stale:instance.panel.staleAfterSeconds};
  }
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
  let modalClosed;
  function modalSpec(raw) {
    if (!raw || typeof raw !== 'object' || typeof raw.title !== 'string' || !raw.title.trim()) return;
    const fields = (Array.isArray(raw.fields) ? raw.fields : []).slice(0,40).map(field => {
      const label = typeof field?.label === 'string' ? field.label.slice(0,100) : '';
      if (!label) return;
      if (typeof field.time === 'string' && validTime(field.time)) return {label,time:field.time};
      if (typeof field.value === 'string' || typeof field.value === 'number') return {label,value:String(field.value).slice(0,500)};
    }).filter(Boolean);
    return {title:raw.title.trim().slice(0,200),body:typeof raw.body === 'string' ? raw.body.slice(0,4000) : '',fields};
  }
  function refreshModalTimes() { for (const element of $('modal').querySelectorAll('[data-relative]')) setText(element, compact(element.dataset.relative)); }
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
  function showModal(spec, accent, onClose) {
    const dialog = $('modal');
    if (dialog.open) dialog.close();
    $('modal-title').textContent = spec.title;
    const body = $('modal-body'); body.replaceChildren();
    for (const paragraph of spec.body.split(/\n\s*\n/).map(text => text.trim()).filter(Boolean)) body.append(node('p',paragraph));
    if (spec.fields.length) {
      const list = node('dl',undefined,'modal-fields');
      for (const field of spec.fields) {
        const value = node('dd');
        if (field.time) {
          const time = node('time',exact(field.time)); time.dateTime = field.time;
          const ago = node('span',compact(field.time),'muted'); ago.dataset.relative = field.time;
          value.append(time,ago);
        } else value.textContent = field.value;
        list.append(node('dt',field.label),value);
      }
      body.append(list);
    }
    dialog.querySelector('.modal-panel').style.setProperty('--accent',accent || palette.lilac);
    modalClosed = onClose;
    dialog.showModal();
    $('modal-close').focus();
  }
  $('modal').addEventListener('close',()=>{ const done = modalClosed; modalClosed = undefined; done?.(); });
  $('modal').addEventListener('click',event=>{ if (event.target === $('modal')) $('modal').close(); });
  $('modal-close').addEventListener('click',()=>$('modal').close());
  function showTimes(record) {
    const times = record.times || {};
    const fields = [];
    fields.push(times.updated ? {label:times.failed ? 'Last verified update' : 'Updated',time:times.updated} : {label:'Updated',value:'Unknown'});
    fields.push(times.observed ? {label:'Observed',time:times.observed} : {label:'Observed',value:'Unknown'});
    if (times.observed && times.staleAfter > 0) fields.push({label:'Stale after',value:times.staleAfter+' seconds'+(times.stale ? ' · Stale now' : '')});
    const body = 'Updated is when this panel’s document, resources or live data last changed on disk. Observed is when the producer says the data was measured; LCARS never invents it.';
    showModal({title:record.latest.title || 'Panel',body,fields},palette[record.display?.panel.accent || record.latest.accent],undefined);
  }
  function updateTimes() {
    let latest; const watched = [];
    for (const record of panels.values()) {
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
    if ($('modal').open) { refreshModalTimes(); for (const element of $('modal').querySelectorAll('[data-relative]')) watched.push(element.dataset.relative); }
    scheduleTimes(watched);
  }
  function showProblems(record) {
    const values = [...(record.latest.diagnostics || []).map(diagnosticText),...record.runtime];
    record.problems.replaceChildren();
    for (const value of [...new Set(values)]) record.problems.append(node('p',value,'diagnostic'));
    if (record.runtime.length) {
      const retry = node('button','Retry latest'); retry.type='button'; retry.addEventListener('click',()=>{record.failedHash='';record.runtime=[];if(record.display?.failed){dispose(record.display);record.display=undefined;}stage(record,record.latest);showProblems(record);});
      const box = node('div',undefined,'diagnostic'); box.append(retry); record.problems.append(box);
    }
  }
  function fail(instance, reason) {
    if (!instances.has(instance.nonce)) return;
    const record = instance.record;
    clearTimeout(instance.deadline); instance.deadline = undefined; instance.failed = true;
    const message = errorText(reason);
    record.runtime = [message]; record.failedHash = instance.panel.bundleHash;
    if (record.candidate === instance) { record.candidate = undefined; dispose(instance); }
    else if (record.display === instance) record.runtime.push('Display unverified. Hashes and time describe the last verified render.');
    showProblems(record); updateTimes();
    if (capturing) captureError = message;
  }
  function dataDeadline(instance) {
    if (instance.deadline || instance.failed) return;
    instance.deadline = setTimeout(()=>fail(instance,'Data rendering acknowledgement exceeded 10 seconds'),10000);
  }
  function deliver(instance, panel) {
    if (!instance || instance.failed) return;
    instance.desired = dataValue(panel);
    instance.dataRevisions.set(instance.desired.hash,instance.desired);
    if (instance.ackData?.hash === instance.desired.hash) return;
    if (instance.ready) { post(instance,'data',{data:instance.desired}); dataDeadline(instance); }
  }
  function promote(instance) {
    const record = instance.record;
    if (record.candidate !== instance || !instance.ready || instance.failed || instance.ackData?.hash !== instance.desired.hash) return;
    const old = record.display;
    const hadFocus = old && document.activeElement === old.frame;
    record.display = instance; record.candidate = undefined;
    clearTimeout(instance.deadline);instance.deadline=undefined;
    instance.frame.classList.remove('frame-candidate'); instance.frame.removeAttribute('aria-hidden'); instance.frame.removeAttribute('tabindex'); instance.frame.inert=false;
    dispose(old);
    typeText(record.heading, instance.panel.title);
    reveal(record, instance);
    record.wrapper.dataset.size = instance.panel.size || 'normal';
    record.wrapper.style.setProperty('--accent',palette[instance.panel.accent] || palette.lilac);
    record.source.hidden = !instance.panel.sourceUrl;
    if (instance.panel.sourceUrl) record.source.href = instance.panel.sourceUrl;
    record.sourceLinks.replaceChildren();
    for (const item of (instance.panel.sources || []).slice(0,100)) {
      if (!item || typeof item.url !== 'string' || !/^https?:\/\//i.test(item.url)) continue;
      const link = node('a',item.title || item.url,'source');link.href=item.url;link.target='_blank';link.rel='noopener noreferrer';record.sourceLinks.append(link);
    }
    record.runtime=[];showProblems(record); updateTimes();
    if (hadFocus) record.heading.focus({preventScroll:true});
  }
  // The slot grows to the frame's height, then a first display fades in.
  function sizeSlot(record, height) {
    if (!Number.isFinite(height)) return;
    record.slot.style.height = height + 'px';
  }
  function reveal(record, instance) {
    sizeSlot(record, parseFloat(instance.frame.style.height) || 80);
    if (record.revealed) return;
    record.revealed = true;
    record.slot.classList.remove('pending');
    if (motionOn()) instance.frame.classList.add('reveal');
  }
  function stage(record,panel) {
    if (!panel.bundleHash || !panel.url) return;
    if (record.failedHash === panel.bundleHash) return;
    if (record.display?.panel.bundleHash === panel.bundleHash && !record.display.failed) { deliver(record.display,panel); return; }
    if (record.candidate?.panel.bundleHash === panel.bundleHash) { deliver(record.candidate,panel); return; }
    dispose(record.candidate);
    const nonce = freshNonce();
    const iframe = node('iframe'); iframe.title=panel.title; iframe.setAttribute('sandbox','allow-scripts'); iframe.setAttribute('referrerpolicy','no-referrer'); iframe.className='frame-candidate';iframe.setAttribute('aria-hidden','true');iframe.tabIndex=-1;iframe.inert=true;
    const url = new URL(panel.url,location.href);url.searchParams.set('nonce',nonce);
    const instance={record,panel:{...panel},frame:iframe,nonce,ready:false,failed:false,desired:dataValue(panel),dataRevisions:new Map([[dataHash(panel),dataValue(panel)]]),overflow:[]};
    record.candidate=instance;instances.set(nonce,instance);
    instance.deadline=setTimeout(()=>fail(instance,'Replacement rendering exceeded 10 seconds. Retaining the last verified display.'),10000);
    iframe.src=url.href;record.slot.append(iframe);
  }
  function makePanel(panel) {
    const wrapper=node('section',undefined,'panel');wrapper.dataset.panelId=panel.id;wrapper.dataset.size=panel.size || 'normal';wrapper.style.setProperty('--accent',palette[panel.accent]||palette.lilac);
    const headingBox=node('div',undefined,'panel-heading');const heading=node('h2');heading.tabIndex=-1;headingBox.append(heading);typeText(heading,panel.title);
    const body=node('div',undefined,'panel-body');const slot=node('div',undefined,'frame-slot pending');const problems=node('div');
    const meta=node('div',undefined,'panel-meta');const updated=node('button','—','panel-time');updated.type='button';updated.setAttribute('aria-haspopup','dialog');
    const unavailable=node('p','Data unavailable');unavailable.hidden=true;
    const source=node('a','Source','source');source.target='_blank';source.rel='noopener noreferrer';source.hidden=true;
    const sourceLinks=node('div');meta.append(updated,unavailable,source,sourceLinks);body.append(slot,problems,meta);wrapper.append(headingBox,body);
    const record={wrapper,heading,slot,problems,updated,unavailable,source,sourceLinks,runtime:[],latest:panel,failedHash:''};
    updated.addEventListener('click',()=>showTimes(record));
    return record;
  }
  function markCurrent(container) {
    for (const link of container.children) {
      const id = link.dataset.tab, here = section === id || (container.id === 'navigation' && section.startsWith(id + '/'));
      if (here) link.setAttribute('aria-current','page'); else link.removeAttribute('aria-current');
    }
  }
  function tabLinks(container, tabs, stagger) {
    container.replaceChildren(...tabs.map((tab, index) => {
      const link = node('a', undefined); link.href = tabHref(tab.id); link.dataset.tab = tab.id;
      typeText(link, tab.title || tab.id, stagger ? index * 70 : 0);
      if (stagger && motionOn()) { link.classList.add('enter'); link.style.animationDelay = (index * 70) + 'ms'; }
      return link;
    }));
  }
  function apply(next) {
    state=next;
    const tabs=next.tabs || [];
    const top=tabs.filter(tab=>!tab.parent);
    const navKey=JSON.stringify(top.map(tab=>[tab.id,tab.title]));
    if(navKey!==navigationKey){navigationKey=navKey;tabLinks($('navigation'),top,true);}
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
    if(subKey!==subnavKey){subnavKey=subKey;tabLinks($('subnav'),subTabs,false);}
    $('subnav').hidden=!subTabs.length;
    markCurrent($('subnav'));
    typeText($('section-title'),topTab?.title || (section?'Section unavailable':'Your canvas is ready'));
    document.title=[current?.parent?current.title:'',topTab?.title || 'Canvas','LCARS'].filter(Boolean).join(' · ');
    $('global-diagnostics').replaceChildren();
    for(const value of [...(next.diagnostics || []),...(current?.parent?topTab?.diagnostics || []:[]),...(current?.diagnostics || [])])$('global-diagnostics').append(node('p',diagnosticText(value),'diagnostic'));
    const visible=(next.panels || []).filter(panel=>panel.tab===section);
    const ids=new Set(visible.map(panel=>panel.id));
    for(const [id,record]of panels){if(!ids.has(id)){dispose(record.candidate);dispose(record.display);record.wrapper.remove();panels.delete(id);}}
    let previous;
    let index=0;for(const panel of visible){let record=panels.get(panel.id);if(!record){record=makePanel(panel);panels.set(panel.id,record);if(booted)enter(record,index);}index++;record.latest=panel;showProblems(record);stage(record,panel);const expected=previous?previous.nextElementSibling:$('panels').firstElementChild;if(expected!==record.wrapper)$('panels').insertBefore(record.wrapper,expected);previous=record.wrapper;}
    $('empty').hidden=!!visible.length;
    if(!visible.length){$('empty').replaceChildren();if(!current&&section){$('empty').append(node('h2','This section is unavailable'),node('p','It may have been moved or removed.'));if(tabs[0]){const link=node('a','Open '+tabs[0].title);link.href=tabHref(tabs[0].id);$('empty').append(link);}}else{$('empty').append(node('h2',current?'No content yet':'Create your first section'),node('p','Publish an HTML, Markdown or text panel in a section folder. LCARS will discover it automatically.'));const link=node('a','Read the authoring guide');link.href='/agent-guide';$('empty').append(link);}}
    updateTimes();
    finishBoot();
  }
  // First load: the frame draws itself while state arrives; the loading grid
  // crossfades out as panels rise in. Panels are never hidden, because browsers
  // pause hidden cross-origin frames and they would not be ready when shown.
  let booted=false;
  function enter(record,index){if(!motionOn())return;record.wrapper.classList.add('enter');record.wrapper.style.animationDelay=(index*60)+'ms';}
  function endBoot(){
    if(booted)return;booted=true;
    document.body.classList.remove('booting');
    [...panels.values()].forEach((record,index)=>enter(record,index));
    const boot=$('boot');if(!boot)return;
    if(motionOn()){boot.classList.add('done');setTimeout(()=>boot.remove(),220);}else boot.remove();
  }
  function finishBoot(){endBoot();}
  function navigate(path,push=true){
    if(push)history.pushState(null,'',path);
    section=sectionFromPath();
    $('menu').setAttribute('aria-expanded','false');document.querySelector('.shell').classList.remove('menu-open');
    if(state)apply(state);
    scrollTo(0,0);$('main').focus({preventScroll:true});
  }
  document.addEventListener('click',event=>{
    const link=event.target.closest?.('a[href]');
    if(!link||event.defaultPrevented||event.button!==0||event.metaKey||event.ctrlKey||event.shiftKey||event.altKey||link.target||capturing)return;
    const url=new URL(link.href,location.href);
    if(url.origin!==location.origin||!url.pathname.startsWith('/s/'))return;
    event.preventDefault();
    if(url.pathname!==location.pathname)navigate(url.pathname);
  });
  window.addEventListener('popstate',()=>navigate(location.pathname,false));
  async function poll() {
    clearTimeout(timer);
    if(latched||document.hidden)return;
    if(polling){pendingCheck=true;return;}
    polling=true;pendingCheck=false;
    let succeeded=false;
    try{
      const response=await fetch('/api/state',{headers:etag?{'If-None-Match':etag}:{},cache:'no-store',signal:AbortSignal.timeout(10000)});
      if(response.status!==304&&!response.ok)throw new Error('State request failed: '+response.status);
      if(response.status!==304){const next=await response.json();if(!latched){etag=response.headers.get('ETag')||'';apply(next);}}
      lastCheck=new Date();$('connection').textContent='Connected';$('connection').classList.remove('disconnected');$('last-check').textContent=lastCheck.toLocaleTimeString();backoff=2000;succeeded=true;
    }catch(error){$('connection').textContent='Disconnected · retaining displayed content';$('connection').classList.add('disconnected');backoff=Math.min(backoff*2,30000);}
    finally{
      polling=false;
      if(!latched&&!document.hidden){
        if(pendingCheck&&succeeded){pendingCheck=false;void poll();}
        else if(!eventsHealthy||!succeeded)timer=setTimeout(poll,backoff);
      }
    }
  }
  function closeEvents(){
    eventStream?.close();eventStream=undefined;eventsHealthy=false;
  }
  function connectEvents(){
    if(latched||document.hidden||!window.EventSource)return;
    if(eventStream&&eventStream.readyState!==EventSource.CLOSED)return;
    closeEvents();
    eventStream=new EventSource('/api/events');
    eventStream.onopen=()=>{eventsHealthy=true;eventFailures=0;clearTimeout(timer);void poll();};
    eventStream.addEventListener('state',event=>{
      if(event.data!==state?.revision)void poll();
    });
    eventStream.onerror=()=>{
      eventsHealthy=false;
      if(++eventFailures>=2)void poll();
      else {clearTimeout(timer);timer=setTimeout(poll,2000);}
    };
  }
  window.addEventListener('pagehide',()=>{closeEvents();clearTimeout(timer);});
  window.addEventListener('pageshow',()=>{connectEvents();void poll();});
  window.addEventListener('message',event=>{
    const message=event.data;
    if(!message||message.v!==1||typeof message.nonce!=='string'||message.nonce.length>100)return;
    const instance=instances.get(message.nonce);
    if(!instance||event.source!==instance.frame.contentWindow||instance.failed)return;
    if(!['hello','height','ready','dataAck','error','captured','modal'].includes(message.type))return;
    if(message.type==='modal'){
      const id=typeof message.id==='string'?message.id.slice(0,40):'';
      const spec=modalSpec(message.modal);
      // Only the displayed frame the user is interacting with may open a modal.
      if(!id||!spec||capturing||instance.record.display!==instance||document.activeElement!==instance.frame||$('modal').open){if(id)post(instance,'modalClosed',{id,opened:false});return;}
      showModal(spec,palette[instance.panel.accent],()=>{if(instances.has(instance.nonce)){instance.frame.focus();post(instance,'modalClosed',{id,opened:true});}});
      return;
    }
    if(message.type==='hello'){if(instance.initialized)return;instance.initialized=true;post(instance,'init',{data:instance.desired,state:frameState()});}
    else if(message.type==='height'){
      if(typeof message.height!=='number'||!Number.isFinite(message.height)||message.height<80||message.height>2400)return;
      instance.frame.style.height=message.height+'px';
      if(instance.record.display===instance)sizeSlot(instance.record,message.height);
      if(Array.isArray(message.overflow))instance.overflow=message.overflow.filter(value=>typeof value==='string').slice(0,20);
      if(Array.isArray(message.diagnostics)&&message.diagnostics.length){instance.record.runtime=message.diagnostics.filter(value=>typeof value==='string').slice(0,10).map(value=>value.slice(0,1500));showProblems(instance.record);}
    }else if(message.type==='dataAck'){
      if(typeof message.hash!=='string'||!instance.dataRevisions.has(message.hash))return;
      instance.ackData=instance.dataRevisions.get(message.hash);
      instance.dataRevisions=new Map([[instance.ackData.hash,instance.ackData],[instance.desired.hash,instance.desired]]);
      if(instance.ackData.hash===instance.desired.hash&&instance.ready){clearTimeout(instance.deadline);instance.deadline=undefined;}
      if(instance.record.candidate===instance)promote(instance);else updateTimes();
    }else if(message.type==='ready'){
      if(message.hash!==instance.ackData?.hash)return;
      instance.ready=true;
      if(Array.isArray(message.overflow))instance.overflow=message.overflow.filter(value=>typeof value==='string').slice(0,20);
      if(instance.ackData.hash!==instance.desired.hash){post(instance,'data',{data:instance.desired});}else promote(instance);
    }else if(message.type==='error'){if(typeof message.error==='string')fail(instance,message.error.slice(0,1500));}
    else if(message.type==='captured'){
      if(message.captureId!==instance.captureId||message.hash!==instance.ackData?.hash)return;
      if(Array.isArray(message.overflow))instance.overflow=message.overflow.filter(value=>typeof value==='string').slice(0,20);
      instance.captureDone=true;
    }
  });
  $('menu').addEventListener('click',()=>{const open=$('menu').getAttribute('aria-expanded')!=='true';$('menu').setAttribute('aria-expanded',String(open));document.querySelector('.shell').classList.toggle('menu-open',open);});
  function motionUI(){ $('motion').setAttribute('aria-pressed',String(storedMotion));applyMotion();sendLifecycle(); }
  $('motion').addEventListener('click',()=>{storedMotion=!storedMotion;try{localStorage.setItem('lcars-motion-off',String(storedMotion));}catch{}motionUI();});
  reduced.addEventListener('change',()=>{applyMotion();sendLifecycle();});
  document.addEventListener('visibilitychange',()=>{sendLifecycle();if(document.hidden){clearTimeout(timer);clearTimeout(timesTimer);closeEvents();}else{updateTimes();connectEvents();void poll();}});
  // Components get a fresh display clock on each wall-clock minute.
  (function minuteTick(){setTimeout(()=>{if(!document.hidden&&!frozenClock)sendLifecycle();minuteTick();},60050-Date.now()%60000);})();
  async function waitFor(predicate,deadline){while(!predicate()){if(captureError)throw new Error(captureError);if(performance.now()>deadline)throw new Error('Capture timed out waiting for expected displayed hashes and readiness');await new Promise(resolve=>setTimeout(resolve,40));}}
  window.LCARSCapture=async(options={})=>{
    if(capturing)throw new Error('Capture already in progress');
    if((options.expectPanelHash||options.expectDataHash)&&!options.panel)throw new Error('Expected hashes require a panel');
    capturing=true;frozenClock=new Date().toISOString();if($('modal').open)$('modal').close();applyMotion();endBoot();clearTimeout(timesTimer);sendLifecycle();
    const deadline=performance.now()+10500;
    try{
      await waitFor(()=>{
        if(!state)return false;
        for(const record of panels.values())if((!options.panel||record.latest.id===options.panel)&&(record.display?.failed||record.failedHash))throw new Error(record.runtime.join('; ') || 'Panel rendering failed');
        const accepted=(state.panels || []).filter(panel=>panel.tab===section&&(!options.panel||panel.id===options.panel));
        if(options.panel&&!accepted.length)return false;
        return accepted.every(panel=>(!options.expectPanelHash||panel.docHash===options.expectPanelHash)&&(!options.expectDataHash||dataHash(panel)===options.expectDataHash));
      },deadline);
      latched=true;clearTimeout(timer);closeEvents();
      const selected=[...panels.values()].filter(record=>!options.panel||record.latest.id===options.panel);
      await waitFor(()=>selected.every(record=>{
        const displayed=record.display;
        return displayed&&!displayed.failed&&displayed.ready&&!record.candidate&&displayed.panel.bundleHash===record.latest.bundleHash&&displayed.ackData?.hash===dataHash(record.latest);
      }),deadline);
      for(const record of selected){const instance=record.display;instance.captureId=freshNonce();instance.captureDone=false;post(instance,'capture',{captureId:instance.captureId,state:frameState()});}
      await waitFor(()=>selected.every(record=>record.display.captureDone),deadline);
      await document.fonts.ready;
      await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));
      updateTimes();
      const overflow=[];
      if(document.documentElement.scrollWidth>innerWidth+1)overflow.push('Shell has horizontal document overflow');
      for(const record of selected)for(const issue of record.display.overflow || [])overflow.push(record.latest.id+': '+issue);
      const errors=[
        ...(state.diagnostics || []).map(diagnosticText),
        ...(state.tabs || []).filter(tab=>tab.id===section).flatMap(tab=>(tab.diagnostics || []).map(diagnosticText)),
        ...selected.flatMap(record=>[...(record.latest.diagnostics || []).map(value=>record.latest.id+': '+diagnosticText(value)),...(record.display.failed?record.runtime:[])]),
        ...overflow
      ].filter(value=>!/^Warning:/.test(value));
      const report={captureTime:frozenClock,viewport:{width:innerWidth,height:innerHeight,deviceScale:devicePixelRatio},revision:state.revision,panels:selected.map(record=>({id:record.latest.id,docHash:record.display.panel.docHash,bundleHash:record.display.panel.bundleHash,dataHash:record.display.ackData.hash,updatedAt:updatedFor(record.display.panel,record.display.ackData)})),errors,overflow};
      window.__lcarsCaptureErrors=errors;return report;
    }catch(error){
      const errors=[errorText(error)];
      const selected=[...panels.values()].filter(record=>!options.panel||record.latest.id===options.panel);
      window.__lcarsCaptureErrors=errors;
      window.__lcarsCaptureReport={captureTime:frozenClock,viewport:{width:innerWidth,height:innerHeight,deviceScale:devicePixelRatio},revision:state?.revision,panels:selected.map(record=>({id:record.latest.id,docHash:record.display?.panel.docHash || '',bundleHash:record.display?.panel.bundleHash || '',dataHash:record.display?.ackData?.hash || '',updatedAt:record.display ? updatedFor(record.display.panel,record.display.ackData) : '',verified:!!record.display&&!record.display.failed})),errors,overflow:[]};
      throw error;
    }
  };
  motionUI();connectEvents();void poll();
})();
