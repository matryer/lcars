/* LCARS shell, bridge and capture contract v1. */
(() => {
  'use strict';
  const $ = id => document.getElementById(id);
  const palette = {apricot:'#FFCC99',orange:'#FF9900',lilac:'#CC99CC',blue:'#9999FF',peach:'#FF9966',critical:'#FF6666'};
  const section = decodeURIComponent(location.pathname.split('/')[2] || '');
  const panels = new Map(), instances = new Map();
  function freshNonce() { return Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2,'0')).join(''); }
  let state, etag = '', navigationKey = '', timer, backoff = 2000, lastCheck, polling = false, capturing = false, latched = false;
  let frozenClock, captureError;
  let eventStream, eventsHealthy = false, pendingCheck = false, eventFailures = 0;
  let storedMotion = false;
  try { storedMotion = localStorage.getItem('lcars-motion-off') === 'true'; } catch {}
  const reduced = matchMedia('(prefers-reduced-motion: reduce)');
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
  function exact(value) { const date = new Date(value); return validTime(value) ? new Intl.DateTimeFormat(undefined,{dateStyle:'full',timeStyle:'long'}).format(date) : 'Update time unknown'; }
  function relative(value) {
    const ms = Date.parse(value); if (!validTime(value)) return 'Update time unknown';
    const age = Date.parse(displayClock())-ms;
    if (age < -1000) return exact(value) + ' (clock skew: future timestamp)';
    if (age < 60000) return 'just now';
    if (age < 3600000) return Math.floor(age/60000) + ' min ago';
    if (age < 86400000) return Math.floor(age/3600000) + ' hr ago';
    return Math.floor(age/86400000) + ' days ago';
  }
  function updateTimes() {
    let latest;
    for (const record of panels.values()) {
      const instance = record.display;
      const updated = instance ? updatedFor(instance.panel,instance.ackData) : '';
      if (updated && (!latest || Date.parse(updated)>Date.parse(latest))) latest = updated;
      record.updated.textContent = updated ? (instance.failed ? 'Last verified update ' : 'Updated ') + relative(updated) : 'Update time unknown';
      record.exact.replaceChildren();
      if (updated) { const time = node('time',exact(updated)); time.dateTime = updated; record.exact.append(time); }
      else record.exact.textContent = 'Update time unknown';
      record.sourceTime.textContent = 'Timestamp source: accepted document, included resources and acknowledged live data file modification times.';
      const observed = observation(instance);
      record.observation.classList.remove('stale');
      if (validTime(observed.date)) {
        const age = Date.parse(displayClock())-Date.parse(observed.date);
        const stale = Number.isFinite(age) && observed.stale > 0 && age > observed.stale*1000;
        record.observation.textContent = (stale ? 'Stale · ' : '') + 'Data observed ' + relative(observed.date);
        record.observation.classList.toggle('stale',stale);
        const time = node('time',exact(observed.date)); time.dateTime = observed.date;
        record.observedExact.replaceChildren(node('span','Observation time: '),time);
      } else { record.observation.textContent = 'Observation time unknown'; record.observedExact.textContent = ''; }
      record.unavailable.hidden = !(instance && instance.panel.kind === 'html' && instance.ackData?.missing);
    }
    $('section-updated').textContent = panels.size ? (latest ? 'Latest content update ' + relative(latest) : 'Latest content update unknown') : 'No content yet';
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
    record.heading.textContent = instance.panel.title;
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
    const headingBox=node('div',undefined,'panel-heading');const heading=node('h2',panel.title);heading.tabIndex=-1;headingBox.append(heading);
    const body=node('div',undefined,'panel-body');const slot=node('div',undefined,'frame-slot');const problems=node('div');
    const meta=node('div',undefined,'panel-meta');const details=node('details');const summary=node('summary');const updated=node('span','Update time unknown');summary.append(updated);const exactTime=node('p'),sourceTime=node('p'),observedExact=node('p');details.append(summary,exactTime,sourceTime,observedExact);
    const observed=node('p','Observation time unknown','observation'),unavailable=node('p','Data unavailable');unavailable.hidden=true;
    const source=node('a','Source','source');source.target='_blank';source.rel='noopener noreferrer';source.hidden=true;
    const sourceLinks=node('div');meta.append(details,observed,unavailable,source,sourceLinks);body.append(slot,problems,meta);wrapper.append(headingBox,body);
    const record={wrapper,heading,slot,problems,updated,exact:exactTime,sourceTime,observedExact,observation:observed,unavailable,source,sourceLinks,runtime:[],latest:panel,failedHash:''};
    return record;
  }
  function apply(next) {
    state=next;
    const tabs=next.tabs || [];
    const navKey=JSON.stringify(tabs.map(tab=>[tab.id,tab.title]));
    if(navKey!==navigationKey){navigationKey=navKey;$('navigation').replaceChildren();for(const tab of tabs){const link=node('a',tab.title || tab.id);link.href='/s/'+encodeURIComponent(tab.id);if(tab.id===section)link.setAttribute('aria-current','page');$('navigation').append(link);}}
    const current=tabs.find(tab=>tab.id===section);
    $('section-title').textContent=current?.title || (section?'Section unavailable':'Your canvas is ready');
    document.title=(current?.title || 'Canvas')+' · LCARS';
    $('global-diagnostics').replaceChildren();
    for(const value of [...(next.diagnostics || []),...(current?.diagnostics || [])])$('global-diagnostics').append(node('p',diagnosticText(value),'diagnostic'));
    const visible=(next.panels || []).filter(panel=>panel.tab===section);
    const ids=new Set(visible.map(panel=>panel.id));
    for(const [id,record]of panels){if(!ids.has(id)){dispose(record.candidate);dispose(record.display);record.wrapper.remove();panels.delete(id);}}
    let previous;
    for(const panel of visible){let record=panels.get(panel.id);if(!record){record=makePanel(panel);panels.set(panel.id,record);}record.latest=panel;showProblems(record);stage(record,panel);const expected=previous?previous.nextElementSibling:$('panels').firstElementChild;if(expected!==record.wrapper)$('panels').insertBefore(record.wrapper,expected);previous=record.wrapper;}
    $('empty').hidden=!!visible.length;
    if(!visible.length){$('empty').replaceChildren();if(!current&&section){$('empty').append(node('h2','This section is unavailable'),node('p','It may have been moved or removed.'));if(tabs[0]){const link=node('a','Open '+tabs[0].title);link.href='/s/'+encodeURIComponent(tabs[0].id);$('empty').append(link);}}else{$('empty').append(node('h2',current?'No content yet':'Create your first section'),node('p','Publish an HTML, Markdown or text panel in a section folder. LCARS will discover it automatically.'));const link=node('a','Read the authoring guide');link.href='/agent-guide';$('empty').append(link);}}
    updateTimes();
  }
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
      lastCheck=new Date();$('connection').textContent='Connected';$('connection').classList.remove('disconnected');$('last-check').textContent='Last successful check '+lastCheck.toLocaleTimeString();backoff=2000;succeeded=true;
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
    if(!['hello','height','ready','dataAck','error','captured'].includes(message.type))return;
    if(message.type==='hello'){if(instance.initialized)return;instance.initialized=true;post(instance,'init',{data:instance.desired,state:frameState()});}
    else if(message.type==='height'){
      if(typeof message.height!=='number'||!Number.isFinite(message.height)||message.height<80||message.height>2400)return;
      instance.frame.style.height=message.height+'px';
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
  function motionUI(){ $('motion').setAttribute('aria-pressed',String(storedMotion));sendLifecycle(); }
  $('motion').addEventListener('click',()=>{storedMotion=!storedMotion;try{localStorage.setItem('lcars-motion-off',String(storedMotion));}catch{}motionUI();});
  reduced.addEventListener('change',sendLifecycle);
  document.addEventListener('visibilitychange',()=>{sendLifecycle();if(document.hidden){clearTimeout(timer);closeEvents();}else{connectEvents();void poll();}});
  setInterval(()=>{updateTimes();sendLifecycle();},60000);
  async function waitFor(predicate,deadline){while(!predicate()){if(captureError)throw new Error(captureError);if(performance.now()>deadline)throw new Error('Capture timed out waiting for expected displayed hashes and readiness');await new Promise(resolve=>setTimeout(resolve,40));}}
  window.LCARSCapture=async(options={})=>{
    if(capturing)throw new Error('Capture already in progress');
    if((options.expectPanelHash||options.expectDataHash)&&!options.panel)throw new Error('Expected hashes require a panel');
    capturing=true;frozenClock=new Date().toISOString();sendLifecycle();
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
