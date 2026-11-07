// window.LCARSCapture, the capture contract. Only the current section is captured;
// parked (preloaded) sections are ignored and no new preloads start while capturing.
async function waitFor(predicate,deadline){while(!predicate()){if(captureError)throw new Error(captureError);if(performance.now()>deadline)throw new Error('Capture timed out waiting for expected displayed hashes and readiness');await new Promise(resolve=>setTimeout(resolve,40));}}
window.LCARSCapture=async(options={})=>{
  if(capturing)throw new Error('Capture already in progress');
  if((options.expectPanelHash||options.expectDataHash)&&!options.panel)throw new Error('Expected hashes require a panel');
  capturing=true;frozenClock=new Date().toISOString();closeTimeDetails();closeExpand(true);endMorph(false);applyMotion();endBoot();clearTimeout(timesTimer);sendLifecycle();
  const deadline=performance.now()+10500;
  try{
    await waitFor(()=>{
      if(!state)return false;
      for(const record of currentRecords())if((!options.panel||record.latest.id===options.panel)&&(record.display?.failed||record.failedHash))throw new Error(record.runtime.join('; ') || 'Panel rendering failed');
      const accepted=(state.panels || []).filter(panel=>panel.tab===section&&(!options.panel||panel.id===options.panel));
      if(options.panel&&!accepted.length)return false;
      return accepted.every(panel=>(!options.expectPanelHash||panel.docHash===options.expectPanelHash)&&(!options.expectDataHash||dataHash(panel)===options.expectDataHash));
    },deadline);
    latched=true;clearTimeout(timer);closeEvents();
    const selected=currentRecords().filter(record=>!options.panel||record.latest.id===options.panel);
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
    const selected=currentRecords().filter(record=>!options.panel||record.latest.id===options.panel);
    window.__lcarsCaptureErrors=errors;
    window.__lcarsCaptureReport={captureTime:frozenClock,viewport:{width:innerWidth,height:innerHeight,deviceScale:devicePixelRatio},revision:state?.revision,panels:selected.map(record=>({id:record.latest.id,docHash:record.display?.panel.docHash || '',bundleHash:record.display?.panel.bundleHash || '',dataHash:record.display?.ackData?.hash || '',updatedAt:record.display ? updatedFor(record.display.panel,record.display.ackData) : '',verified:!!record.display&&!record.display.failed})),errors,overflow:[]};
    throw error;
  }
};
