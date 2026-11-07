// Keyboard shortcuts, boot sequence, in-page navigate(), link interception and popstate.
// Keyboard: 1-9 open sections, [ and ] step through the current section's tabs.
document.addEventListener('keydown', event => {
  if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey || capturing || lifted) return;
  if (event.target.closest?.('input, textarea, select, [contenteditable]')) return;
  const top = [...$('navigation').children], tabs = [...$('subnav').children];
  if (/^[1-9]$/.test(event.key) && top[Number(event.key) - 1]) { event.preventDefault(); flash([top[Number(event.key) - 1]]); navigate(top[Number(event.key) - 1].pathname); return; }
  if ((event.key === '[' || event.key === ']') && tabs.length > 1) {
    event.preventDefault();
    const at = Math.max(0, tabs.findIndex(link => link.hasAttribute('aria-current')));
    const next = tabs[(at + (event.key === ']' ? 1 : tabs.length - 1)) % tabs.length];
    flash([next]); navigate(next.pathname);
  }
});
// First load: the frame draws itself while state arrives; the loading grid
// crossfades out as panels rise in, each after its own stable random delay.
let booted=false;
function enter(record){
  if(!motionOn())return;
  const wrapper=record.wrapper;
  wrapper.classList.add('enter');wrapper.style.animationDelay=entryDelay('panel:'+record.section+':'+record.latest.id)+'ms';
  wrapper.addEventListener('animationend',event=>{if(event.target===wrapper)wrapper.classList.remove('enter');},{once:true});
}
function endBoot(){
  if(booted)return;booted=true;
  document.body.classList.remove('booting');
  currentRecords().forEach(enter);
  for(const [seed,tile]of pool.get(section)?.tiles || [])if(motionOn())enterTile(tile,seed);
  const boot=$('boot');if(!boot)return;
  if(motionOn()){boot.classList.add('done');setTimeout(()=>boot.remove(),220);}else boot.remove();
}
function finishBoot(){endBoot();}
// Every navigation (links, history, keys, frames) comes through here. With
// motion on, the old section's frames morph into the new section's.
function navigate(path,push=true){
  if(push)history.pushState(null,'',path);
  closeExpand(true);
  const from=section;
  const animate=motionOn()&&booted&&!!state&&typeof Element.prototype.animate==='function';
  const before=animate?morphSources():[];
  endMorph(false);
  section=sectionFromPath();
  $('menu').setAttribute('aria-expanded','false');document.querySelector('.shell').classList.remove('menu-open');
  if(state){navigating=true;try{apply(state);}finally{navigating=false;}}
  scrollTo(0,0);$('main').focus({preventScroll:true});
  sendLifecycle();
  if(animate&&section!==from&&pool.has(section))startMorph(before,pool.get(section));
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
