/* LCARS component bridge, contract v1. The only host action is a user-initiated, text-only modal. */
(() => {
  'use strict';
  const nonce = new URL(location.href).searchParams.get('nonce');
  const dataListeners = new Set(), stateListeners = new Set(), captureListeners = new Set();
  let initialized = false, stopped = false, ready = false, currentData, pendingData, pumping = false;
  let state = {visible: false, motion: false, capture: false, clock: new Date().toISOString()};
  let domReady = false, deferrals = 0, finishDeferrals = [];
  const modalWaiters = new Map(); let modalCount = 0;
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
    for (const callback of [...stateListeners]) { try { callback({...state}); } catch (error) { fail(error); } }
    for (const element of document.querySelectorAll('lcars-time')) element.render?.();
  }
  const clip = (value, limit) => String(value).slice(0, limit);
  function modalSpec(options) {
    if (!options || typeof options !== 'object') throw new TypeError('openModal needs an options object');
    const title = typeof options.title === 'string' ? options.title.trim() : '';
    if (!title) throw new TypeError('openModal needs a title');
    if (options.body !== undefined && typeof options.body !== 'string') throw new TypeError('openModal body must be a string');
    if (options.fields !== undefined && !Array.isArray(options.fields)) throw new TypeError('openModal fields must be an array');
    const fields = (options.fields || []).slice(0, 40).map(field => {
      if (!field || typeof field.label !== 'string' || !field.label.trim()) throw new TypeError('Each modal field needs a label');
      if (field.time !== undefined) {
        if (!Number.isFinite(Date.parse(field.time))) throw new TypeError(`Modal field "${field.label}" has an invalid time`);
        return {label: clip(field.label, 100), time: new Date(field.time).toISOString()};
      }
      if (typeof field.value !== 'string' && typeof field.value !== 'number') throw new TypeError(`Modal field "${field.label}" needs a value or time`);
      return {label: clip(field.label, 100), value: clip(field.value, 500)};
    });
    return {title: clip(title, 200), body: clip(options.body || '', 4000), fields};
  }
  function openModal(options) {
    const modal = modalSpec(options);
    if (!initialized || stopped || (navigator.userActivation && !navigator.userActivation.isActive)) return Promise.resolve(false);
    const id = String(++modalCount), returnFocus = document.activeElement;
    post('modal', {id, modal});
    return new Promise(resolve => modalWaiters.set(id, opened => {
      if (opened && returnFocus && returnFocus !== document.body) returnFocus.focus?.({preventScroll: true});
      resolve(opened);
    }));
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
        button = document.createElement('button'); button.type = 'button'; button.setAttribute('aria-haspopup', 'dialog');
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
      openModal({title: label, fields: [{label, time: value}]}).catch(fail);
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
    openModal,
    deferReady() {
      if (ready) throw new Error('deferReady must be called during initialization');
      deferrals++; let done = false;
      return () => { if (!done) { done = true; deferrals--; deferredDone(); } };
    }
  });
  let measuredHeight = 0, scheduled = false, changes = [], oscillationSent = false;
  function overflow() {
    const issues = [];
    if (document.documentElement.scrollWidth > innerWidth + 1) issues.push('Component has horizontal document overflow');
    return issues;
  }
  function measure() {
    scheduled = false;
    const content = document.querySelector('.lcars-content') || document.body;
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
    if (!['init', 'data', 'state', 'capture', 'modalClosed'].includes(message.type)) return;
    if (message.type === 'modalClosed') {
      const done = modalWaiters.get(message.id);
      if (done) { modalWaiters.delete(message.id); done(message.opened !== false); }
      return;
    }
    if (message.type === 'init' || message.type === 'data') {
      const value = message.data;
      if (!value || typeof value.hash !== 'string' || value.hash.length > 64 || JSON.stringify(value).length > 300000) return;
      if (message.type === 'init') { if (domReady) start(value, message.state); }
      else if (initialized) { pendingData = value; if (ready) pump(); }
    } else if (message.type === 'state') setState(message.state);
    else if (message.type === 'capture' && ready && !pumping && !pendingData && typeof message.captureId === 'string' && message.captureId.length < 100) {
      try {
        setState(message.state);
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
    const content = document.querySelector('.lcars-content') || document.body;
    new ResizeObserver(scheduleMeasure).observe(content);
    new MutationObserver(scheduleMeasure).observe(content, {childList: true, subtree: true, attributes: true, characterData: true});
    post('hello'); scheduleMeasure();
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', mounted, {once: true}); else mounted();
})();
