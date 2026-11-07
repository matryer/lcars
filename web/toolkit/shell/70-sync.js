// State polling, server-sent events and page show/hide listeners.
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
    lastCheck=new Date();$('connection').textContent='Connected';$('connection').classList.remove('disconnected');$('last-check').textContent=lastCheck.toLocaleTimeString([],{hour:'2-digit',minute:'2-digit'});$('last-check').title='Last checked '+lastCheck.toLocaleString();backoff=2000;succeeded=true;
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
