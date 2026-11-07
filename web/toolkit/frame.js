/* LCARS component bridge, contract v1. Detail views render inside this frame; section navigation is user-initiated only. */
(() => {
  'use strict';
  const nonce = new URL(location.href).searchParams.get('nonce');
  const dataListeners = new Set(), stateListeners = new Set(), captureListeners = new Set();
  let initialized = false, stopped = false, ready = false, currentData, pendingData, pumping = false;
  let state = {visible: false, motion: false, capture: false, clock: new Date().toISOString()};
  let domReady = false, deferrals = 0, finishDeferrals = [];
  const navigateWaiters = new Map(); let navigateCount = 0;
  const post = (type, values = {}) => parent.postMessage({v: 1, nonce, type, ...values}, '*');
  const textError = error => String(error?.message || error || 'Unknown rendering error').slice(0, 1500);
  function fail(error) { if (stopped) return; stopped = true; post('error', {error: textError(error)}); }
  function deferredDone() { if (!deferrals) { finishDeferrals.forEach(done => done()); finishDeferrals = []; } }
  function delivery(value) { return {...(value.envelope || {}), data: value.envelope?.data ?? null, missing: !!value.missing}; }
  async function callData(value) { for (const callback of [...dataListeners]) await callback(delivery(value)); }
  async function pump() {
    if (pumping || stopped) return;
    pumping = true;
    try {
      while (pendingData && !stopped) {
        const value = pendingData; pendingData = undefined;
        await callData(value);
        currentData = value;
        post('dataAck', {hash: value.hash});
      }
    } catch (error) { fail(error); }
    finally { pumping = false; }
  }
  async function imagesReady() {
    await Promise.all([...document.images].map(image => {
      if (image.complete) { if (!image.naturalWidth) throw new Error('Image failed: ' + image.getAttribute('src')); return; }
      return new Promise((resolve, reject) => {
        image.addEventListener('load', resolve, {once: true});
        image.addEventListener('error', () => reject(new Error('Image failed: ' + image.getAttribute('src'))), {once: true});
      });
    }));
  }
  const frame = () => new Promise(resolve => { const fallback = setTimeout(resolve, 80); requestAnimationFrame(() => { clearTimeout(fallback); resolve(); }); });
  async function settle() { await document.fonts.ready; await imagesReady(); await frame(); await frame(); }
  async function start(value, nextState) {
    if (initialized || stopped) return;
    initialized = true;
    setState(nextState);
    try {
      await callData(value);
      currentData = value;
      post('dataAck', {hash: value.hash});
      if (deferrals) await new Promise(resolve => finishDeferrals.push(resolve));
      await settle();
      if (stopped) return;
      ready = true;
      measure();
      post('ready', {hash: value.hash, overflow: overflow()});
      if (pendingData) pump();
    } catch (error) { fail(error); }
  }
  function setState(next) {
    if (!next || typeof next !== 'object') return;
    state = {visible: !!next.visible, motion: !!next.motion, capture: !!next.capture, clock: typeof next.clock === 'string' ? next.clock : state.clock};
    document.documentElement.dataset.motion = state.motion ? 'on' : 'off';
    // Parked and hidden panels pause optional animation such as .lcars-scan.
    document.documentElement.dataset.visible = state.visible ? 'true' : 'false';
    // The page theme recolours the palette variables; warning and critical stay fixed.
    if (next.theme && typeof next.theme === 'object') for (const key of ['apricot', 'lilac', 'blue', 'peach']) {
      const value = next.theme[key];
      if (typeof value === 'string' && /^#[0-9a-f]{6}$/i.test(value)) document.documentElement.style.setProperty('--' + key, value);
    }
    for (const callback of [...stateListeners]) { try { callback({...state}); } catch (error) { fail(error); } }
    for (const element of document.querySelectorAll('lcars-time')) element.render?.();
    if (detail) refreshDetailTimes();
  }
  const clip = (value, limit) => String(value).slice(0, limit);
  function detailSpec(options) {
    if (!options || typeof options !== 'object') throw new TypeError('openDetail needs an options object');
    const title = typeof options.title === 'string' ? options.title.trim() : '';
    if (!title) throw new TypeError('openDetail needs a title');
    if (options.body !== undefined && typeof options.body !== 'string') throw new TypeError('openDetail body must be a string');
    if (options.fields !== undefined && !Array.isArray(options.fields)) throw new TypeError('openDetail fields must be an array');
    const fields = (options.fields || []).slice(0, 40).map(field => {
      if (!field || typeof field.label !== 'string' || !field.label.trim()) throw new TypeError('Each detail field needs a label');
      if (field.time !== undefined) {
        if (!Number.isFinite(Date.parse(field.time))) throw new TypeError(`Detail field "${field.label}" has an invalid time`);
        return {label: clip(field.label, 100), time: new Date(field.time).toISOString()};
      }
      if (typeof field.value !== 'string' && typeof field.value !== 'number') throw new TypeError(`Detail field "${field.label}" needs a value or time`);
      return {label: clip(field.label, 100), value: clip(field.value, 500)};
    });
    return {title: clip(title, 200), body: clip(options.body || '', 4000), fields};
  }
  // Detail views replace the panel's content in place. The content is hidden,
  // not removed, so its state, scroll positions and listeners survive Back.
  // The view lives in a shadow root so panel styles cannot restyle it.
  const detailStyle = `:host{display:block;padding:2px;color:var(--lcars-text);font:16px/1.5 var(--font-body);overflow-wrap:anywhere}
*,*::before,*::after{box-sizing:border-box}
.bar{display:flex;align-items:center;gap:8px;margin:0 0 16px}
.back{display:inline-flex;align-items:center;gap:8px;min-height:44px;margin:0;padding:0 20px 0 16px;border:0;border-radius:22px 0 0 22px;background:var(--lcars-apricot);color:var(--lcars-ink);font:500 18px/1 var(--lcars-label-font);text-transform:uppercase;letter-spacing:.06em;cursor:pointer}
.back:hover{filter:brightness(1.12)}
.back:active{transform:translateX(-2px)}
.rule{flex:1 1 auto;min-width:24px;height:12px;background:var(--lcars-lilac);border-radius:0 6px 6px 0}
h2{margin:0 0 16px;font:500 28px/1.2 var(--lcars-label-font);color:var(--lcars-apricot);text-transform:uppercase;letter-spacing:.02em}
h2:focus{outline:none}
p{margin:0 0 16px}
dl{display:grid;grid-template-columns:minmax(0,max-content) minmax(0,1fr);gap:10px 24px;margin:0}
dt{font:500 18px/1.35 var(--lcars-label-font);text-transform:uppercase;letter-spacing:.04em;color:var(--lcars-lilac)}
dd{margin:0;display:grid;gap:2px;min-width:0}
.ago{color:var(--lcars-muted);font-size:14px}
:focus-visible{outline:3px solid var(--lcars-blue);outline-offset:4px}
@media(max-width:400px){dl{grid-template-columns:minmax(0,1fr);gap:2px}dd{margin-bottom:10px}}
@media(prefers-reduced-motion:reduce){.back{transition:none}}`;
  let detail, lastActivated;
  // Safari does not focus buttons on click, so remember what was last pressed.
  document.addEventListener('pointerdown', event => { lastActivated = event.target?.closest?.('button,a,[tabindex],input,select,summary,lcars-time') || undefined; }, true);
  const contentElement = () => document.querySelector('.lcars-content') || document.body;
  function renderDetail(view, spec) {
    const root = view.shadowRoot;
    const style = document.createElement('style'); style.textContent = detailStyle;
    const bar = document.createElement('div'); bar.className = 'bar';
    const back = document.createElement('button'); back.type = 'button'; back.className = 'back';
    const arrow = document.createElement('span'); arrow.setAttribute('aria-hidden', 'true'); arrow.textContent = '◄';
    back.append(arrow, 'Back'); back.addEventListener('click', () => closeDetail());
    const rule = document.createElement('span'); rule.className = 'rule'; rule.setAttribute('aria-hidden', 'true');
    bar.append(back, rule);
    const heading = document.createElement('h2'); heading.id = 'title'; heading.tabIndex = -1; heading.textContent = spec.title;
    const parts = [style, bar, heading];
    for (const paragraph of spec.body.split(/\n\s*\n/).map(text => text.trim()).filter(Boolean)) {
      const p = document.createElement('p'); p.textContent = paragraph; parts.push(p);
    }
    if (spec.fields.length) {
      const list = document.createElement('dl');
      for (const field of spec.fields) {
        const term = document.createElement('dt'); term.textContent = field.label;
        const value = document.createElement('dd');
        if (field.time) {
          const time = document.createElement('time'); time.dateTime = field.time; time.textContent = exactFormat.format(new Date(field.time));
          const ago = document.createElement('span'); ago.className = 'ago'; ago.dataset.time = field.time;
          value.append(time, ago);
        } else value.textContent = field.value;
        list.append(term, value);
      }
      parts.push(list);
    }
    root.replaceChildren(...parts);
    refreshDetailTimes();
    return heading;
  }
  function refreshDetailTimes() {
    for (const element of detail?.view.shadowRoot.querySelectorAll('[data-time]') || []) {
      const text = relativeParts(Date.parse(element.dataset.time)).long;
      if (element.textContent !== text) element.textContent = text;
    }
  }
  function openDetail(options) {
    const spec = detailSpec(options);
    if (!initialized || stopped || state.capture) return Promise.resolve(false);
    if (detail) { const replaced = detail.resolve; detail.resolve = undefined; replaced?.(false); }
    else {
      const content = contentElement(), active = document.activeElement;
      const opener = active && active !== document.body && content.contains(active) ? active : lastActivated?.isConnected && content.contains(lastActivated) ? lastActivated : undefined;
      const view = document.createElement('div'); view.className = 'lcars-detail'; view.setAttribute('role', 'region'); view.attachShadow({mode: 'open'});
      detail = {view, opener, openerId: opener?.id || '', scroll: scrollY};
      content.inert = true;
      document.documentElement.dataset.lcarsDetail = 'open';
      document.body.append(view);
      resizeObserver?.observe(view);
    }
    const heading = renderDetail(detail.view, spec);
    detail.view.setAttribute('aria-label', spec.title);
    scrollTo(0, 0);
    heading.focus({preventScroll: true});
    measure();
    post('detail', {title: spec.title});
    return new Promise(resolve => { detail.resolve = resolve; });
  }
  function closeDetail() {
    if (!detail) return;
    const {view, opener, openerId, scroll, resolve} = detail;
    detail = undefined;
    resizeObserver?.unobserve(view);
    view.remove();
    const content = contentElement();
    content.inert = false;
    delete document.documentElement.dataset.lcarsDetail;
    scrollTo(0, scroll);
    // Return focus to the opener, or to its replacement if the panel re-rendered it with the same id.
    const target = opener?.isConnected ? opener : openerId ? document.getElementById(openerId) : null;
    if (target) target.focus({preventScroll: true});
    else { if (!content.hasAttribute('tabindex')) { content.tabIndex = -1; content.addEventListener('blur', () => content.removeAttribute('tabindex'), {once: true}); } content.focus({preventScroll: true}); }
    measure();
    post('detail', {title: null});
    resolve?.(true);
  }
  document.addEventListener('keydown', event => {
    if (detail && event.key === 'Escape' && !event.defaultPrevented) { event.preventDefault(); closeDetail(); }
  });
  function navigate(section) {
    if (typeof section !== 'string' || !/^[a-z0-9-]{1,64}(\/[a-z0-9-]{1,64})?$/.test(section)) throw new TypeError('navigate needs a section ID such as "systems" or "systems/network"');
    if (!initialized || stopped || (navigator.userActivation && !navigator.userActivation.isActive)) return Promise.resolve(false);
    const id = String(++navigateCount);
    post('navigate', {id, section});
    return new Promise(resolve => navigateWaiters.set(id, resolve));
  }
  const relativeFormat = new Intl.RelativeTimeFormat(undefined, {numeric: 'auto'});
  const exactFormat = new Intl.DateTimeFormat(undefined, {dateStyle: 'full', timeStyle: 'long'});
  function relativeParts(ms) {
    const delta = ms - Date.parse(state.clock), size = Math.abs(delta);
    const [unit, step, letter] = size < 3600000 ? ['minute', 60000, 'm'] : size < 86400000 ? ['hour', 3600000, 'h'] : ['day', 86400000, 'd'];
    const count = Math.max(1, Math.floor(size / step));
    return {short: delta > 0 ? `in ${count}${letter}` : `${count}${letter}`, long: relativeFormat.format(delta > 0 ? count : -count, unit)};
  }
  class RelativeTime extends HTMLElement {
    static get observedAttributes() { return ['datetime', 'label']; }
    connectedCallback() { this.render(); }
    attributeChangedCallback() { if (this.isConnected) this.render(); }
    render() {
      let button = this.firstElementChild;
      if (!button || button.localName !== 'button' || this.childElementCount !== 1) {
        button = document.createElement('button'); button.type = 'button';
        button.addEventListener('click', () => this.open());
        this.replaceChildren(button);
      }
      const ms = Date.parse(this.getAttribute('datetime'));
      button.disabled = !Number.isFinite(ms);
      if (button.disabled) { button.textContent = '—'; button.removeAttribute('title'); button.setAttribute('aria-label', 'Time unknown'); return; }
      const parts = relativeParts(ms), exact = exactFormat.format(new Date(ms)), label = this.getAttribute('label');
      if (button.textContent !== parts.short) button.textContent = parts.short;
      button.title = exact;
      button.setAttribute('aria-label', `${label ? label + ': ' : ''}${parts.long}. Show exact time`);
    }
    open() {
      const value = this.getAttribute('datetime');
      if (!Number.isFinite(Date.parse(value))) return;
      const label = this.getAttribute('label') || 'Time';
      try { openDetail({title: label, fields: [{label, time: value}]}); } catch (error) { fail(error); }
    }
  }
  if (!customElements.get('lcars-time')) customElements.define('lcars-time', RelativeTime);
  window.LCARS = Object.freeze({
    version: 1,
    onData(callback) {
      if (typeof callback !== 'function') throw new TypeError('onData needs a callback');
      dataListeners.add(callback);
      if (currentData && ready) Promise.resolve().then(() => callback(delivery(currentData))).catch(fail);
      return () => dataListeners.delete(callback);
    },
    onState(callback) {
      if (typeof callback !== 'function') throw new TypeError('onState needs a callback');
      stateListeners.add(callback);
      if (initialized) callback({...state});
      return () => stateListeners.delete(callback);
    },
    onCapture(callback) {
      if (typeof callback !== 'function') throw new TypeError('onCapture needs a callback');
      captureListeners.add(callback);
      return () => captureListeners.delete(callback);
    },
    openDetail,
    // Contract v1 alias: openModal now opens the same in-panel detail view.
    openModal: openDetail,
    navigate,
    deferReady() {
      if (ready) throw new Error('deferReady must be called during initialization');
      deferrals++; let done = false;
      return () => { if (!done) { done = true; deferrals--; deferredDone(); } };
    }
  });
  let measuredHeight = 0, scheduled = false, changes = [], oscillationSent = false, resizeObserver;
  function overflow() {
    const issues = [];
    if (document.documentElement.scrollWidth > innerWidth + 1) issues.push('Component has horizontal document overflow');
    return issues;
  }
  function measure() {
    scheduled = false;
    const content = detail ? detail.view : contentElement();
    if (!content) return;
    const height = Math.ceil(Math.max(content.getBoundingClientRect().height, content.scrollHeight));
    if (!Number.isFinite(height) || Math.abs(height - measuredHeight) < 1) return;
    measuredHeight = height;
    const now = performance.now(); changes = changes.filter(change => now - change < 1000); changes.push(now);
    const diagnostics = [];
    if (height > 2400) diagnostics.push('Content exceeds 2400px; scroll inside this panel for more');
    if (changes.length > 20 && !oscillationSent) { diagnostics.push('Possible component resize oscillation'); oscillationSent = true; }
    post('height', {height: Math.max(80, Math.min(2400, height)), diagnostics, overflow: overflow()});
  }
  function scheduleMeasure() { if (!scheduled) { scheduled = true; requestAnimationFrame(measure); } }
  window.addEventListener('message', async event => {
    const message = event.data;
    if (event.source !== parent || !message || message.v !== 1 || message.nonce !== nonce || stopped) return;
    if (!['init', 'data', 'state', 'capture', 'back', 'navigated'].includes(message.type)) return;
    if (message.type === 'navigated') {
      const done = navigateWaiters.get(message.id);
      if (done) { navigateWaiters.delete(message.id); done(message.ok === true); }
      return;
    }
    if (message.type === 'back') { closeDetail(); return; }
    if (message.type === 'init' || message.type === 'data') {
      const value = message.data;
      if (!value || typeof value.hash !== 'string' || value.hash.length > 64 || JSON.stringify(value).length > 300000) return;
      if (message.type === 'init') { if (domReady) start(value, message.state); }
      else if (initialized) { pendingData = value; if (ready) pump(); }
    } else if (message.type === 'state') setState(message.state);
    else if (message.type === 'capture' && ready && !pumping && !pendingData && typeof message.captureId === 'string' && message.captureId.length < 100) {
      try {
        setState(message.state);
        closeDetail(); // captures show the panel itself, never a detail view
        for (const callback of [...captureListeners]) await callback({...state});
        await settle(); measure();
        post('captured', {captureId: message.captureId, hash: currentData.hash, overflow: overflow()});
      } catch (error) { fail(error); }
    }
  });
  window.addEventListener('error', event => {
    if (event.target !== window) fail('Resource failed: ' + (event.target.src || event.target.href || event.target.tagName));
    else fail(event.error || event.message);
  }, true);
  window.addEventListener('unhandledrejection', event => fail(event.reason));
  document.addEventListener('securitypolicyviolation', event => fail('Resource blocked by component policy: ' + event.blockedURI));
  function mounted() {
    domReady = true;
    const content = contentElement();
    resizeObserver = new ResizeObserver(scheduleMeasure);
    resizeObserver.observe(content);
    new MutationObserver(scheduleMeasure).observe(content, {childList: true, subtree: true, attributes: true, characterData: true});
    post('hello'); scheduleMeasure();
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', mounted, {once: true}); else mounted();
})();
