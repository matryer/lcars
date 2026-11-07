// The window 'message' listener for component frames.
window.addEventListener('message',event=>{
  const message=event.data;
  if(!message||message.v!==1||typeof message.nonce!=='string'||message.nonce.length>100)return;
  const instance=instances.get(message.nonce);
  if(!instance||event.source!==instance.frame.contentWindow||instance.failed)return;
  if(!['hello','height','ready','dataAck','error','captured','detail','navigate'].includes(message.type))return;
  if(message.type==='navigate'){
    const id=typeof message.id==='string'?message.id.slice(0,40):'';
    const target=typeof message.section==='string'?message.section:'';
    // Only the displayed frame the user is interacting with, to a section that exists.
    const ok=!!id&&!capturing&&instance.record.display===instance&&document.activeElement===instance.frame&&(state?.tabs || []).some(tab=>tab.id===target);
    if(id)post(instance,'navigated',{id,ok});
    if(ok)navigate(tabHref(target));
    return;
  }
  // A frame opened ({title}) or left ({title:null}) its in-panel detail view.
  if(message.type==='detail'){frameDetail(instance,message.title);return;}
  if(message.type==='hello'){if(instance.initialized)return;instance.initialized=true;prepareTimes(instance.record);post(instance,'init',{data:instance.desired,state:frameState()});}
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
    if(instance.record.candidate===instance){promote(instance);syncDetail(instance.record);}else updateTimes();
  }else if(message.type==='ready'){
    if(message.hash!==instance.ackData?.hash)return;
    instance.ready=true;
    if(Array.isArray(message.overflow))instance.overflow=message.overflow.filter(value=>typeof value==='string').slice(0,20);
    if(instance.ackData.hash!==instance.desired.hash){post(instance,'data',{data:instance.desired});}else{promote(instance);syncDetail(instance.record);}
  }else if(message.type==='error'){if(typeof message.error==='string'){fail(instance,message.error.slice(0,1500));syncDetail(instance.record);}}
  else if(message.type==='captured'){
    if(message.captureId!==instance.captureId||message.hash!==instance.ackData?.hash)return;
    if(Array.isArray(message.overflow))instance.overflow=message.overflow.filter(value=>typeof value==='string').slice(0,20);
    instance.captureDone=true;
  }
});
