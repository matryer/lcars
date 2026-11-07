// Component bridge: problems, fail, deliver, promote, reveal and stage of frame instances.
function showProblems(record) {
  const values = [...(record.latest.diagnostics || []).map(diagnosticText),...record.runtime];
  record.problems.replaceChildren();
  for (const value of [...new Set(values)]) record.problems.append(node('p',value,'diagnostic'));
  if (record.runtime.length) {
    const retry = node('button','Retry latest'); retry.type='button';
    retry.addEventListener('click',()=>{
      record.failedHash='';record.runtime=[];
      if(record.display?.failed){dispose(record.display);record.display=undefined;}
      stage(record,record.latest);showProblems(record);
    });
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
  sectionProgress();
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
  record.wrapper.style.setProperty('--accent',palette[instance.panel.accent] || palette.lilac);record.wrapper.dataset.accent=instance.panel.accent || 'lilac';
  record.source.hidden = !instance.panel.sourceUrl;
  if (instance.panel.sourceUrl) record.source.href = instance.panel.sourceUrl;
  record.sourceLinks.replaceChildren();
  for (const item of (instance.panel.sources || []).slice(0,100)) {
    if (!item || typeof item.url !== 'string' || !/^https?:\/\//i.test(item.url)) continue;
    const link = node('a',item.title || item.url,'source');link.href=item.url;link.target='_blank';link.rel='noopener noreferrer';record.sourceLinks.append(link);
  }
  record.runtime=[];showProblems(record); updateTimes();
  if (hadFocus) record.heading.focus({preventScroll:true});
  sectionProgress();
}
// The slot grows to the frame's height, then a first display fades in.
function sizeSlot(record, height) {
  if (!Number.isFinite(height)) return;
  record.slot.style.height = height + 'px';
}
// Placeholder bars appear only if a panel in view is still loading after 350ms.
function holdPlaceholder(record) {
  clearTimeout(record.holdTimer);
  if (record.revealed) return;
  record.holdTimer = setTimeout(() => { if (!record.revealed && record.section === section) record.slot.classList.add('pending'); }, 350);
}
function reveal(record, instance) {
  const height = parseFloat(instance.frame.style.height) || 80;
  if (record.revealed) { sizeSlot(record, height); return; }
  record.revealed = true; clearTimeout(record.holdTimer);
  // A panel that was quick (no placeholder shown) takes its height at once, without a visible jump.
  if (!record.slot.classList.contains('pending')) {
    record.slot.classList.add('instant');
    requestAnimationFrame(() => requestAnimationFrame(() => record.slot.classList.remove('instant')));
  }
  record.slot.classList.remove('pending');
  sizeSlot(record, height);
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
