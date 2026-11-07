import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { JSDOM, VirtualConsole } from 'jsdom';
import FakeTimers from '@sinonjs/fake-timers';

const html = readFileSync(new URL('../shell.html', import.meta.url), 'utf8');
// Build the shell exactly as web.ShellScript does: shell/*.js in filename order
// inside one strict-mode wrapper, so the tests run what the server sends.
const shellDir = new URL('../toolkit/shell/', import.meta.url);
const script = '/* LCARS shell, bridge and capture contract v1. Built from web/toolkit/shell/*.js in filename order. */\n'
  + "(() => {\n'use strict';\n"
  + readdirSync(shellDir).filter(name => name.endsWith('.js')).sort().map(name => {
    const part = readFileSync(new URL(name, shellDir), 'utf8');
    return '// --- shell/' + name + ' ---\n' + part + (part.endsWith('\n') ? '' : '\n');
  }).join('')
  + '\n})();\n';
export const NOW = Date.parse('2026-11-07T12:00:00Z');
export const ago = ms => new Date(NOW - ms).toISOString();
export const panel = (overrides = {}) => ({
  id: 'overview/status', tab: 'overview', title: 'Status', kind: 'html',
  bundleHash: 'bundle-1', docHash: 'doc-1', url: '/component/status',
  updatedAt: ago(120_000), data: { hash: 'data-1', updatedAt: ago(60_000), envelope: {} },
  ...overrides,
});
export const state = (panels = [panel()], overrides = {}) => ({
  revision: 'revision-1', tabs: [{ id: 'overview', title: 'Overview' }], panels, ...overrides,
});

// Evaluate the shipped script and HTML without exposing its private functions.
// Only browser boundaries (network, media preference, clock) are substituted.
export async function shell(t, { initial = state(), path = '/s/overview', events = true } = {}) {
  const errors = [];
  const console = new VirtualConsole();
  console.on('jsdomError', error => errors.push(error));
  const dom = new JSDOM(html, {
    url: 'https://lcars.test' + path, runScripts: 'outside-only',
    pretendToBeVisual: true, virtualConsole: console,
  });
  const { window } = dom;
  // Let jsdom's initial load/pageshow finish before starting the shell.
  await new Promise(resolve => window.addEventListener('load', resolve, { once: true }));
  await new Promise(resolve => setTimeout(resolve, 0));
  const clock = FakeTimers.withGlobal(window).install({
    now: NOW, toFake: ['Date', 'setTimeout', 'clearTimeout', 'performance', 'requestAnimationFrame', 'cancelAnimationFrame'],
  });
  t.after(() => {
    clock.uninstall();
    dom.window.close();
    assert.deepEqual(errors, [], 'uncaught browser errors');
  });
  window.matchMedia = () => ({ matches: true, addEventListener() {} });
  window.scrollTo = () => {};
  const requests = [], responses = [], streams = [];
  const response = (value, etag = '"' + value.revision + '"') => ({
    status: 200, ok: true, headers: new Headers({ ETag: etag }), json: async () => value,
  });
  responses.push(response(initial));
  window.fetch = async (url, options) => {
    requests.push({ url, ...options });
    const next = responses.shift();
    if (next instanceof Error) throw next;
    return typeof next === 'function' ? next() : next || { status: 304 };
  };
  class EventSource extends window.EventTarget {
    static CLOSED = 2;
    readyState = 0;
    constructor(url) { super(); this.url = url; streams.push(this); }
    close() { this.readyState = EventSource.CLOSED; }
    open() { this.readyState = 1; this.onopen(); }
    fail() { this.onerror(); }
    notify(revision) { this.dispatchEvent(new window.MessageEvent('state', { data: revision })); }
  }
  window.EventSource = events ? EventSource : undefined;
  window.eval(script);
  await clock.tickAsync(0);
  const $ = selector => window.document.querySelector(selector);
  const all = selector => [...window.document.querySelectorAll(selector)];
  const message = (frame, values, source = frame.contentWindow) => {
    const nonce = new URL(frame.src).searchParams.get('nonce');
    window.dispatchEvent(new window.MessageEvent('message', { source, data: { v: 1, nonce, ...values } }));
  };
  return {
    window, clock, $, all, requests, responses, streams, response, message,
    acknowledge(frame, hash = 'data-1') {
      message(frame, { type: 'dataAck', hash });
      message(frame, { type: 'ready', hash });
    },
    async update(next) {
      responses.push(response(next));
      window.dispatchEvent(new window.Event('pageshow'));
      await clock.tickAsync(0);
      assert.equal(responses.length, 0, 'state update was fetched');
    },
    click(link, options = {}) {
      link.dispatchEvent(new window.MouseEvent('click', { bubbles: true, cancelable: true, button: 0, ...options }));
    },
  };
}
