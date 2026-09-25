import assert from 'node:assert/strict';
import test from 'node:test';
import {disposeOwnedContext} from './owned_context.mjs';

function fakeGuard(responses) {
  const calls = [];
  return {
    calls,
    coverage: {failed: false, closed: false},
    async send(method, params) {
      calls.push({method, params});
      const next = responses.shift();
      if (next instanceof Error) throw next;
      return structuredClone(next);
    },
  };
}

const happyResponses = () => [
  {browserContextIds: ['owned', 'unrelated']},
  {targetInfos: [
    {targetId: 'owned-page', browserContextId: 'owned'},
    {targetId: 'other-page', browserContextId: 'unrelated'},
  ]},
  {},
  {browserContextIds: ['unrelated']},
  {targetInfos: [{targetId: 'other-page', browserContextId: 'unrelated'}]},
];

test('disposes exactly one owned context in five commands', async () => {
  const guard = fakeGuard(happyResponses()), events = [];
  const result = await disposeOwnedContext({guard, contextId: 'owned', events});
  assert.deepEqual(guard.calls, [
    {method: 'Target.getBrowserContexts', params: {}},
    {method: 'Target.getTargets', params: {}},
    {method: 'Target.disposeBrowserContext', params: {browserContextId: 'owned'}},
    {method: 'Target.getBrowserContexts', params: {}},
    {method: 'Target.getTargets', params: {}},
  ]);
  assert.deepEqual(result, {contextId: 'owned', contextClosed: true, targetIds: ['owned-page'], targetsClosed: true});
  assert.equal(events.length, 10);
  assert.ok(events.every(event => typeof event.at === 'string'));
  assert.deepEqual(events.map(event => event.kind), ['command', 'response', 'command', 'response', 'command', 'response', 'command', 'response', 'command', 'response']);
});

test('refuses invalid identity, event storage, and guard state before sending', async t => {
  for (const [name, input] of [
    ['missing identity', {contextId: ''}],
    ['non-string identity', {contextId: 1}],
    ['missing events', {contextId: 'owned', events: null}],
    ['missing send', {contextId: 'owned', events: [], guard: {coverage: {failed: false, closed: false}}}],
    ['failed guard', {contextId: 'owned', events: [], guard: {send() {}, coverage: {failed: true, closed: false}}}],
    ['closed guard', {contextId: 'owned', events: [], guard: {send() {}, coverage: {failed: false, closed: true}}}],
  ]) await t.test(name, async () => {
    const guard = input.guard ?? fakeGuard([]);
    await assert.rejects(disposeOwnedContext({guard, events: [], ...input}));
    assert.deepEqual(guard.calls ?? [], []);
  });
});

test('refuses malformed preflight responses without disposal', async t => {
  for (const [name, responses] of [
    ['malformed contexts', [{browserContextIds: null}]],
    ['missing owned context', [{browserContextIds: ['unrelated']}]],
    ['malformed targets', [{browserContextIds: ['owned']}, {targetInfos: [{targetId: ''}]}]],
  ]) await t.test(name, async () => {
    const guard = fakeGuard(responses), events = [];
    await assert.rejects(disposeOwnedContext({guard, contextId: 'owned', events}));
    assert.equal(guard.calls.some(call => call.method === 'Target.disposeBrowserContext'), false);
  });
});

test('records and rethrows a disposal protocol error', async () => {
  const failure = new Error('protocol refusal');
  const guard = fakeGuard([happyResponses()[0], happyResponses()[1], failure]), events = [];
  await assert.rejects(disposeOwnedContext({guard, contextId: 'owned', events}), failure);
  assert.equal(guard.calls.length, 3);
  assert.equal(events.at(-1).kind, 'error');
  assert.match(events.at(-1).error, /protocol refusal/);
});

test('refuses a false pass when context remains', async () => {
  const responses = happyResponses();
  responses[3] = {browserContextIds: ['owned', 'unrelated']};
  const guard = fakeGuard(responses);
  await assert.rejects(disposeOwnedContext({guard, contextId: 'owned', events: []}), /context remains/);
  assert.equal(guard.calls.length, 4);
});

test('refuses a false pass when an owned target remains', async () => {
  const responses = happyResponses();
  responses[4] = {targetInfos: [{targetId: 'owned-page'}]};
  const guard = fakeGuard(responses);
  await assert.rejects(disposeOwnedContext({guard, contextId: 'owned', events: []}), /target remains/);
  assert.equal(guard.calls.length, 5);
});

test('refuses success if an unrelated context disappears', async () => {
  const responses = happyResponses();
  responses[3] = {browserContextIds: []};
  const guard = fakeGuard(responses);
  await assert.rejects(disposeOwnedContext({guard, contextId: 'owned', events: []}), /unrelated browser context/);
});

test('guard failure during preflight prevents disposal', async () => {
  const guard = fakeGuard(happyResponses()), original = guard.send;
  guard.send = async function(method, params) {
    const result = await original.call(this, method, params);
    if (method === 'Target.getTargets') this.coverage.failed = true;
    return result;
  };
  await assert.rejects(disposeOwnedContext({guard, contextId: 'owned', events: []}), /healthy and open/);
  assert.equal(guard.calls.length, 2);
});

test('malformed post-disposal inventories never become closure evidence', async t => {
  for (const [name, index, value] of [
    ['contexts', 3, {}], ['targets', 4, {targetInfos: null}],
    ['duplicate contexts', 3, {browserContextIds: ['unrelated', 'unrelated']}],
    ['disposal response', 2, null],
  ]) await t.test(name, async () => {
    const responses = happyResponses(); responses[index] = value;
    await assert.rejects(disposeOwnedContext({guard: fakeGuard(responses), contextId: 'owned', events: []}));
  });
});
