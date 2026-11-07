import assert from 'node:assert/strict';
import { test } from 'node:test';
import { shell, panel, state, ago } from './shell-harness.mjs';

const displayed = 'iframe:not(.frame-candidate)';
const candidate = 'iframe.frame-candidate';

test('a panel becomes visible only after acknowledging data and readiness', async t => {
  const s = await shell(t);
  const frame = s.$(candidate);
  assert.ok(frame);
  assert.equal(s.$(displayed), null);
  assert.equal(frame.getAttribute('aria-hidden'), 'true');
  s.message(frame, { type: 'ready', hash: 'data-1' });
  assert.equal(s.$(displayed), null, 'ready before acknowledgement is ignored');
  s.message(frame, { type: 'dataAck', hash: 'unknown' });
  s.message(frame, { type: 'ready', hash: 'unknown' });
  assert.equal(s.$(displayed), null, 'unknown data cannot be promoted');
  s.message(frame, { type: 'dataAck', hash: 'data-1' });
  assert.equal(s.$(displayed), null, 'acknowledgement alone is insufficient');
  s.message(frame, { type: 'ready', hash: 'data-1' });
  assert.equal(s.$(displayed), frame);
  assert.equal(frame.hasAttribute('aria-hidden'), false);
  assert.equal(frame.inert, false);
});

test('a replacement retains the working frame and heading until verified', async t => {
  const s = await shell(t);
  const old = s.$(candidate);
  s.acknowledge(old);
  await s.update(state([panel({ bundleHash: 'bundle-2', title: 'New status' })]));
  const next = s.$(candidate);
  assert.notEqual(next, old);
  assert.equal(s.$(displayed), old);
  assert.equal(s.$('.panel h2').textContent, 'Status');
  s.message(next, { type: 'dataAck', hash: 'data-1' });
  assert.equal(s.$(displayed), old);
  s.message(next, { type: 'ready', hash: 'data-1' });
  assert.equal(s.$(displayed), next);
  assert.equal(old.isConnected, false);
  assert.equal(s.$('.panel h2').textContent, 'New status');
});

for (const failure of ['error', 'timeout']) {
  test(`a replacement ${failure} keeps the last verified display and allows retry`, async t => {
    const s = await shell(t);
    const old = s.$(candidate);
    s.acknowledge(old);
    const nextState = state([panel({ bundleHash: 'bundle-2' })]);
    await s.update(nextState);
    const failed = s.$(candidate);
    if (failure === 'error') s.message(failed, { type: 'error', error: 'Renderer failed' });
    else {
      await s.clock.tickAsync(9_999);
      assert.equal(s.$(candidate), failed);
      await s.clock.tickAsync(1);
    }
    assert.equal(s.$(displayed), old);
    assert.equal(failed.isConnected, false);
    assert.match(s.$('.diagnostic').textContent, failure === 'error' ? /Renderer failed/ : /exceeded 10 seconds/);
    await s.update(nextState);
    assert.equal(s.$(candidate), null, 'failed bundle is not retried on every poll');
    s.click(s.all('.diagnostic button')[0]);
    const retried = s.$(candidate);
    assert.ok(retried);
    assert.notEqual(retried, failed);
    s.acknowledge(retried);
    assert.equal(s.$(displayed), retried);
    assert.equal(s.$('.diagnostic'), null);
  });
}

test('live data updates reuse the frame and publish timestamps only after acknowledgement', async t => {
  const s = await shell(t);
  const frame = s.$(candidate);
  s.acknowledge(frame);
  const messages = [];
  frame.contentWindow.postMessage = message => messages.push(message);
  const time = s.$('.panel-time').textContent;
  await s.update(state([panel({ data: { hash: 'data-2', updatedAt: ago(7_200_000), envelope: {} } })]));
  assert.equal(s.$(displayed), frame);
  assert.equal(s.$(candidate), null);
  assert.equal(messages.at(-1).type, 'data');
  assert.equal(messages.at(-1).data.hash, 'data-2');
  assert.equal(s.$('.panel-time').textContent, time);
  s.message(frame, { type: 'dataAck', hash: 'never-sent' });
  assert.equal(s.$('.panel-time').textContent, time);
  s.message(frame, { type: 'dataAck', hash: 'data-2' });
  assert.equal(s.$('.panel-time').textContent, '2m', 'publication time is the newest document/data time');
  await s.clock.tickAsync(10_000);
  assert.equal(s.$('.diagnostic'), null, 'acknowledgement cancels the data timeout');
});

test('unacknowledged live data reports failure while retaining the displayed frame', async t => {
  const s = await shell(t);
  const frame = s.$(candidate);
  s.acknowledge(frame);
  await s.update(state([panel({ data: { hash: 'data-2', envelope: {} } })]));
  await s.clock.tickAsync(10_000);
  assert.equal(s.$(displayed), frame);
  assert.match(s.$('.panel-body').textContent, /Data rendering acknowledgement exceeded 10 seconds/);
  assert.match(s.$('.panel-body').textContent, /last verified render/);
});

test('the bridge rejects invalid versions, nonces, sources, types and heights', async t => {
  const s = await shell(t);
  const frame = s.$(candidate);
  s.acknowledge(frame);
  s.message(frame, { type: 'height', height: 180 });
  assert.equal(frame.style.height, '180px');
  for (const fields of [
    { v: 2 }, { nonce: 'wrong' }, { nonce: 123 }, { nonce: 'x'.repeat(101) },
    { type: 'unsupported' }, ...[79, 2401, NaN, Infinity, '200', null].map(height => ({ height })),
  ]) {
    s.message(frame, { type: 'height', height: 200, ...fields });
    assert.equal(frame.style.height, '180px', JSON.stringify(fields));
  }
  s.message(frame, { type: 'height', height: 200 }, s.window);
  assert.equal(frame.style.height, '180px', 'correct nonce from another window is rejected');
  for (const data of [null, {}, 'hello']) {
    s.window.dispatchEvent(new s.window.MessageEvent('message', { source: frame.contentWindow, data }));
  }
  for (const height of [80, 2400]) {
    s.message(frame, { type: 'height', height });
    assert.equal(frame.style.height, height + 'px');
    assert.equal(s.$('.frame-slot').style.height, height + 'px');
  }
});

test('hello initializes a frame once, with the expected data and lifecycle state', async t => {
  const s = await shell(t);
  const frame = s.$(candidate), messages = [];
  frame.contentWindow.postMessage = message => messages.push(message);
  s.message(frame, { type: 'hello' });
  s.message(frame, { type: 'hello' });
  assert.equal(messages.length, 1);
  assert.equal(messages[0].type, 'init');
  assert.equal(messages[0].data.hash, 'data-1');
  assert.equal(messages[0].state.visible, true);
  assert.equal(messages[0].state.motion, false);
});

test('polling uses ETags and a 304 preserves the existing display', async t => {
  const s = await shell(t, { events: false });
  const frame = s.$(candidate);
  s.acknowledge(frame);
  assert.equal(s.requests[0].url, '/api/state');
  assert.deepEqual(Object.keys(s.requests[0].headers), []);
  await s.clock.tickAsync(1_999);
  assert.equal(s.requests.length, 1);
  await s.clock.tickAsync(1);
  assert.equal(s.requests.length, 2);
  assert.equal(s.requests[1].headers['If-None-Match'], '"revision-1"');
  assert.equal(s.requests[1].cache, 'no-store');
  assert.equal(s.$(displayed), frame);
  assert.equal(s.$('#connection').textContent, 'Connected');
});

test('polling failures back off, retain content, and recover to the normal interval', async t => {
  const s = await shell(t, { events: false });
  const frame = s.$(candidate);
  s.acknowledge(frame);
  s.responses.push(new Error('offline'), { status: 503, ok: false });
  await s.clock.tickAsync(2_000);
  assert.match(s.$('#connection').textContent, /Disconnected/);
  assert.equal(s.$(displayed), frame);
  for (const [wait, count] of [[4_000, 3], [8_000, 4], [2_000, 5]]) {
    await s.clock.tickAsync(wait - 1);
    assert.equal(s.requests.length, count - 1);
    await s.clock.tickAsync(1);
    assert.equal(s.requests.length, count);
  }
  assert.equal(s.$('#connection').textContent, 'Connected');
  assert.equal(s.$('#connection').classList.contains('disconnected'), false);
  assert.equal(s.$(displayed), frame);
});

test('healthy events suppress polling; event failure falls back and reconnect recovers', async t => {
  const s = await shell(t);
  const stream = s.streams[0];
  stream.open();
  await s.clock.tickAsync(0);
  const count = s.requests.length;
  await s.clock.tickAsync(10_000);
  assert.equal(s.requests.length, count);
  stream.notify('revision-1');
  await s.clock.tickAsync(0);
  assert.equal(s.requests.length, count, 'unchanged revisions need no fetch');
  stream.notify('revision-2');
  await s.clock.tickAsync(0);
  assert.equal(s.requests.length, count + 1);
  stream.fail();
  await s.clock.tickAsync(2_000);
  assert.equal(s.requests.length, count + 2);
  stream.open();
  await s.clock.tickAsync(0);
  const recovered = s.requests.length;
  await s.clock.tickAsync(10_000);
  assert.equal(s.requests.length, recovered);
});

test('events arriving during a fetch coalesce into one follow-up request', async t => {
  const s = await shell(t);
  let finish;
  s.responses.push(() => new Promise(resolve => { finish = resolve; }));
  const count = s.requests.length;
  for (let i = 0; i < 5; i++) s.streams[0].notify('new-revision');
  assert.equal(s.requests.length, count + 1);
  finish({ status: 304 });
  await s.clock.tickAsync(0);
  assert.equal(s.requests.length, count + 2);
});

const tabs = [
  { id: 'overview', title: 'Overview' }, { id: 'work', title: 'Work' },
  { id: 'work/next steps', title: 'Next steps', parent: 'work' },
  { id: 'work/later', title: 'Later', parent: 'work' },
];

test('navigation opens a first subsection, encodes paths, and handles browser history', async t => {
  const s = await shell(t, { initial: state([
    panel(), panel({ id: 'next', tab: 'work/next steps' }), panel({ id: 'later', tab: 'work/later' }),
  ], { tabs }) });
  const previous = s.$(candidate);
  s.acknowledge(previous);
  s.click(s.$('#navigation [data-tab="work"]'));
  assert.equal(s.window.location.pathname, '/s/work/next%20steps');
  assert.equal(s.$('#navigation [aria-current]').dataset.tab, 'work');
  assert.equal(s.$('#subnav [aria-current]').dataset.tab, 'work/next steps');
  assert.equal(s.$('#subnav [data-tab="work/next steps"]').getAttribute('href'), '/s/work/next%20steps');
  assert.equal(s.$('.panel').dataset.panelId, 'next');
  assert.equal(previous.isConnected, false);
  assert.equal(s.window.document.title, 'Next steps · Work · LCARS');
  s.click(s.$('#subnav [data-tab="work/later"]'));
  assert.equal(s.window.location.pathname, '/s/work/later');
  assert.equal(s.$('.panel').dataset.panelId, 'later');
  s.window.history.replaceState(null, '', '/s/overview');
  s.window.dispatchEvent(new s.window.PopStateEvent('popstate'));
  assert.equal(s.$('#navigation [aria-current]').dataset.tab, 'overview');
  assert.equal(s.$('#subnav').hidden, true);
  assert.equal(s.$('.panel').dataset.panelId, 'overview/status');
  assert.equal(s.requests.length, 1, 'navigation uses the already fetched state');
});

test('a direct nested URL selects its panel and a removed section has a recovery link', async t => {
  const s = await shell(t, {
    path: '/s/work/next%20steps',
    initial: state([panel({ id: 'next', tab: 'work/next steps' })], { tabs }),
  });
  assert.equal(s.$('.panel').dataset.panelId, 'next');
  await s.update(state([], { tabs: tabs.slice(0, 1) }));
  assert.equal(s.$('iframe'), null);
  assert.match(s.$('#empty').textContent, /This section is unavailable/);
  assert.equal(s.$('#empty a').getAttribute('href'), '/s/overview');
});

for (const [name, timestamp, expected] of [
  ['missing', undefined, '—'], ['invalid', 'invalid', '—'], ['zero year', '0001-01-01T00:00:00Z', '—'],
  ['just now', ago(0), '1m'], ['before hour', ago(3_599_999), '59m'],
  ['hour', ago(3_600_000), '1h'], ['before day', ago(86_399_999), '23h'], ['day', ago(86_400_000), '1d'],
]) {
  test(`timestamps: ${name}`, async t => {
    const s = await shell(t, { initial: state([panel({ updatedAt: timestamp, data: { hash: 'data-1', envelope: {} } })]) });
    s.acknowledge(s.$(candidate));
    assert.equal(s.$('.panel-time').textContent, expected);
  });
}

test('future publication times explicitly indicate clock skew', async t => {
  const s = await shell(t, { initial: state([panel({ updatedAt: ago(-120_000), data: { hash: 'data-1', envelope: {} } })]) });
  s.acknowledge(s.$(candidate));
  assert.match(s.$('.panel-time').textContent, /clock skew: future timestamp/);
});

test('observation times take precedence, stale data is marked, and labels refresh on time', async t => {
  const s = await shell(t, { initial: state([panel({ data: {
    hash: 'data-1', updatedAt: ago(60_000), envelope: { updatedAt: ago(3_599_000), staleAfterSeconds: 60 },
  } })]) });
  s.acknowledge(s.$(candidate));
  assert.equal(s.$('.panel-time').textContent, 'Stale · 59m');
  assert.equal(s.$('.panel-time').classList.contains('stale'), true);
  assert.match(s.$('.panel-time').getAttribute('aria-label'), /Observed/);
  assert.equal(s.$('#section-updated').textContent, '1m', 'section time describes publication');
  await s.clock.tickAsync(1_050);
  assert.equal(s.$('.panel-time').textContent, 'Stale · 1h');
});
