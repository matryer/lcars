/* LCARS component bridge, contract v1. No host actions are exposed. */
(() => {
  'use strict';
  const nonce = new URL(location.href).searchParams.get('nonce');
  const dataListeners = new Set(), stateListeners = new Set(), captureListeners = new Set();
  let initialized = false, stopped = false, ready = false, currentData, pendingData, pumping = false;
  let state = {visible: false, motion: false, capture: false, clock: new Date().toISOString()};
  let domReady = false, deferrals = 0, finishDeferrals = [];
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
  }
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
    if (!['init', 'data', 'state', 'capture'].includes(message.type)) return;
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
