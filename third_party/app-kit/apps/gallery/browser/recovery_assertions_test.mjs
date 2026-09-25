import test from 'node:test';
import assert from 'node:assert/strict';
import {recoveryVerdict, settingsInputRecoveryVerdict, settingsRecoveryValue, typingRecoveryVerdict} from './recovery_assertions.mjs';

function valid(stage = 'after-acceptance') {
  const accepted = stage === 'after-acceptance';
  return {stage, forwardedHeldPosts: accepted ? 1 : 0, heldStatus: accepted ? 303 : null,
    posts: accepted ? 3 : 2, sent: 'Submitted Avery', revised: 'Reviewed Avery',
    initial: {value: 'Initial Avery', version: '9007199254740993'},
    retained: {value: 'Submitted Avery', version: '9007199254740993'},
    saved: {value: accepted ? 'Submitted Avery' : 'Initial Avery', version: accepted ? '9007199254740994' : '9007199254740993'},
    final: {value: 'Reviewed Avery', version: accepted ? '9007199254740995' : '9007199254740994'},
    sawConflict: accepted};
}

test('both exact recovery histories pass without floating-point version rounding', () => {
  assert.deepEqual(recoveryVerdict(valid()), []);
  assert.deepEqual(recoveryVerdict(valid('before-dispatch')), []);
});

function validTyping(stage = 'after-acceptance') {
  const observation = valid(stage);
  return {stage: observation.stage, forwardedHeldPosts: observation.forwardedHeldPosts,
    heldStatus: observation.heldStatus, posts: observation.posts, sent: observation.sent,
    edited: `${observation.sent} typed`, revised: observation.initial.value,
    initial: observation.initial, retained: {value: `${observation.sent} typed`, version: observation.initial.version},
    saved: observation.saved, final: {value: observation.initial.value, version: observation.final.version},
    sawConflict: observation.sawConflict};
}

test('typing histories retain the observed edit and explicitly save the reverted baseline', () => {
  assert.deepEqual(typingRecoveryVerdict(validTyping()), []);
  assert.deepEqual(typingRecoveryVerdict(validTyping('before-dispatch')), []);
});

const settingsBaseline = {timezone: 'America/Phoenix', start_page: 'home', digest: true};
function validSettingsInput(journey, stage = 'after-acceptance') {
  const accepted = stage === 'after-acceptance';
  const initial = settingsRecoveryValue(settingsBaseline);
  const sentFields = ['settings-selections', 'settings-start-page'].includes(journey) ? {timezone: 'UTC', start_page: 'activity', digest: false} :
    {timezone: 'America/Phoenix', start_page: 'home', digest: false};
  const editedFields = journey === 'settings-selections' ? {...sentFields, timezone: 'Europe/London'} :
    journey === 'settings-start-page' ? {...sentFields, start_page: 'home'} : settingsBaseline;
  const sent = settingsRecoveryValue(sentFields), edited = settingsRecoveryValue(editedFields);
  return {journey, stage, forwardedHeldPosts: accepted ? 1 : 0, heldStatus: accepted ? 303 : null,
    posts: accepted ? 3 : 2, initial: {value: initial, version: '9007199254740993'}, sent, edited, revised: initial,
    retained: {value: edited, version: '9007199254740993'},
    saved: {value: accepted ? sent : initial, version: accepted ? '9007199254740994' : '9007199254740993'},
    final: {value: initial, version: accepted ? '9007199254740995' : '9007199254740994'}, sawConflict: accepted};
}

test('Settings input histories retain exactly one control edit and explicitly return to baseline', () => {
  for (const journey of ['settings-selections', 'settings-start-page', 'settings-checkbox']) {
    assert.deepEqual(settingsInputRecoveryVerdict(validSettingsInput(journey)), [], `${journey} accepted`);
    assert.deepEqual(settingsInputRecoveryVerdict(validSettingsInput(journey, 'before-dispatch')), [], `${journey} pre-dispatch`);
  }
});

for (const [name, journey, mutate, reason] of [
  ['missing journey', 'settings-selections', o => { delete o.journey; }, 'incomplete Settings input observation'],
  ['extra normalized field', 'settings-selections', o => { o.normalized = true; }, 'incomplete Settings input observation'],
  ['noncanonical snapshot', 'settings-selections', o => { o.sent = '["UTC", "activity",false]'; }, 'invalid Settings snapshot'],
  ['untyped false checkbox', 'settings-checkbox', o => { o.edited = '["America/Phoenix","home","false"]'; }, 'invalid Settings snapshot'],
  ['unknown journey', 'settings-selections', o => { o.journey = 'settings-unknown'; }, 'unknown Settings input journey'],
  ['selection loses timezone edit', 'settings-selections', o => { o.edited = settingsRecoveryValue({timezone: 'UTC', start_page: 'activity', digest: false}); o.retained.value = o.edited; }, 'wrong edited Settings control'],
  ['selection changes another control', 'settings-selections', o => { o.edited = settingsRecoveryValue({timezone: 'Europe/London', start_page: 'home', digest: false}); o.retained.value = o.edited; }, 'wrong edited Settings control'],
  ['start-page retains submitted value', 'settings-start-page', o => { o.edited = o.sent; o.retained.value = o.sent; }, 'wrong edited Settings control'],
  ['start-page changes another control', 'settings-start-page', o => { o.edited = settingsRecoveryValue({timezone: 'Europe/London', start_page: 'home', digest: false}); o.retained.value = o.edited; }, 'wrong edited Settings control'],
  ['checkbox remains false', 'settings-checkbox', o => { o.edited = o.sent; o.retained.value = o.sent; }, 'wrong edited Settings control'],
  ['stale retained version', 'settings-selections', o => { o.retained.version = o.saved.version; }, 'lost draft or original version'],
  ['automatic repost', 'settings-checkbox', o => { o.posts++; }, 'unexpected POST count'],
  ['skipped conflict', 'settings-selections', o => { o.sawConflict = false; }, 'wrong retry conflict outcome'],
]) {
  test(`Settings input verdict rejects ${name}`, () => {
    const observation = validSettingsInput(journey);
    mutate(observation);
    assert.ok(settingsInputRecoveryVerdict(observation).includes(reason));
  });
}

for (const [name, mutate, reason] of [
  ['missing edited draft', o => { delete o.edited; }, 'incomplete typing observation'],
  ['extra observed field', o => { o.normalized = true; }, 'incomplete typing observation'],
  ['same retained and submitted draft', o => { o.edited = o.sent; o.retained.value = o.sent; }, 'typing did not retain a distinct edited draft'],
  ['retry does not use baseline', o => { o.revised = 'different retry'; o.final.value = o.revised; }, 'retry did not use the reverted baseline'],
  ['lost typed draft', o => { o.retained.value = o.sent; }, 'lost draft or original version'],
  ['automatic typing-lane repost', o => { o.posts++; }, 'unexpected POST count'],
]) {
  test(`typing verdict rejects ${name}`, () => {
    const observation = validTyping();
    mutate(observation);
    assert.ok(typingRecoveryVerdict(observation).includes(reason));
  });
}

for (const [name, mutate, reason] of [
  ['wrong dispatch', o => { o.forwardedHeldPosts = 0; }, 'wrong dispatch or acceptance'],
  ['missing acceptance', o => { o.heldStatus = null; }, 'wrong dispatch or acceptance'],
  ['automatic repost', o => { o.posts++; }, 'unexpected POST count'],
  ['lost draft', o => { o.retained.value = o.initial.value; }, 'lost draft or original version'],
  ['silently advanced draft version', o => { o.retained.version = o.saved.version; }, 'lost draft or original version'],
  ['extra acceptance', o => { o.saved.version = '9007199254740995'; }, 'wrong interruption version increment'],
  ['extra final write', o => { o.final.version = '9007199254740996'; }, 'wrong final version increment'],
  ['wrong final value', o => { o.final.value = o.sent; }, 'wrong final saved value'],
  ['skipped conflict', o => { o.sawConflict = false; }, 'wrong retry conflict outcome'],
  ['rounded numeric version', o => { o.saved.version = 9007199254740994; }, 'invalid version observation'],
]) {
  test(`rejects ${name} canary`, () => {
    const observation = valid();
    mutate(observation);
    assert.ok(recoveryVerdict(observation).includes(reason));
  });
}

test('pre-dispatch lane refuses an accepted write and an unknown stage', () => {
  const observation = valid('before-dispatch');
  observation.saved = {value: observation.sent, version: '9007199254740994'};
  assert.deepEqual(recoveryVerdict(observation), ['wrong saved value after interruption', 'wrong interruption version increment']);
  assert.deepEqual(recoveryVerdict({...observation, stage: 'unspecified'}), ['unknown stage']);
});

test('Settings snapshots distinguish checked, unchecked and absent controls', () => {
  const fields = {timezone: 'UTC', start_page: 'activity', digest: false};
  assert.equal(settingsRecoveryValue(fields), '["UTC","activity",false]');
  assert.equal(settingsRecoveryValue({digest: false, start_page: 'activity', timezone: 'UTC'}), settingsRecoveryValue(fields));
  assert.notEqual(settingsRecoveryValue({...fields, digest: true}), settingsRecoveryValue(fields));
  for (const invalid of [null, {}, {timezone: 'UTC', start_page: 'activity'},
    {...fields, digest: ''}, {...fields, digest: 'false'}, {...fields, start_page: undefined},
    {...fields, extra: 'unexpected'}]) assert.throws(() => settingsRecoveryValue(invalid));
});

test('Settings recovery refuses losing any one control, including a false checkbox', () => {
  const initial = {timezone: 'America/Phoenix', start_page: 'home', digest: true};
  const sent = {timezone: 'UTC', start_page: 'activity', digest: false};
  const revised = {timezone: 'Europe/London', start_page: 'home', digest: true};
  for (const stage of ['before-dispatch', 'after-acceptance']) {
    const observation = valid(stage);
    observation.initial.value = settingsRecoveryValue(initial);
    observation.sent = observation.retained.value = settingsRecoveryValue(sent);
    observation.saved.value = stage === 'after-acceptance' ? observation.sent : observation.initial.value;
    observation.revised = observation.final.value = settingsRecoveryValue(revised);
    assert.deepEqual(recoveryVerdict(observation), []);
    for (const field of ['timezone', 'start_page', 'digest']) {
      const corrupted = structuredClone(observation);
      corrupted.retained.value = settingsRecoveryValue({...sent, [field]: initial[field]});
      assert.ok(recoveryVerdict(corrupted).includes('lost draft or original version'), field);
    }
  }
});
