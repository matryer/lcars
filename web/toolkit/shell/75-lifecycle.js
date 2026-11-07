// Menu and motion toggles, visibility lifecycle and the minute tick.
$('menu').addEventListener('click',()=>{const open=$('menu').getAttribute('aria-expanded')!=='true';$('menu').setAttribute('aria-expanded',String(open));document.querySelector('.shell').classList.toggle('menu-open',open);});
// Motion follows the OS reduced-motion setting (and capture); there is no on-page toggle any more.
// A preference saved by the old Motion button could never be undone, so it is dropped at load.
storedMotion=false;try{localStorage.removeItem('lcars-motion-off');}catch{}
function motionUI(){ applyMotion();sendLifecycle(); }
reduced.addEventListener('change',()=>{applyMotion();sendLifecycle();});
document.addEventListener('visibilitychange',()=>{sendLifecycle();if(document.hidden){clearTimeout(timer);clearTimeout(timesTimer);closeEvents();}else{updateTimes();connectEvents();void poll();}});
// Components get a fresh display clock on each wall-clock minute.
(function minuteTick(){setTimeout(()=>{if(!document.hidden&&!frozenClock)sendLifecycle();minuteTick();},60050-Date.now()%60000);})();
