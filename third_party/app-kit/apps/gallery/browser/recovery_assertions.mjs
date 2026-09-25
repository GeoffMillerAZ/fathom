// The same bounded observation contract validates real browser cases and seeded
// false evidence in the unit tests. It does not simulate or perform a save.
export function settingsRecoveryValue(fields) {
  if (!fields || Object.keys(fields).sort().join(',') !== 'digest,start_page,timezone' ||
      typeof fields.timezone !== 'string' || typeof fields.start_page !== 'string' ||
      typeof fields.digest !== 'boolean') throw new Error('Incomplete or untyped Settings observation');
  // Fixed field order; false is an observed checkbox value, not missing data.
  return JSON.stringify([fields.timezone, fields.start_page, fields.digest]);
}

function recoveryHistoryVerdict(observation, retainedValue) {
  const errors = [];
  const o = observation;
  if (!['before-dispatch', 'after-acceptance'].includes(o.stage)) return ['unknown stage'];
  const accepted = o.stage === 'after-acceptance';
  if (o.forwardedHeldPosts !== (accepted ? 1 : 0) || o.heldStatus !== (accepted ? 303 : null)) errors.push('wrong dispatch or acceptance');
  if (o.posts !== (accepted ? 3 : 2)) errors.push('unexpected POST count');
  if (o.retained.value !== retainedValue || o.retained.version !== o.initial.version) errors.push('lost draft or original version');
  if (o.saved.value !== (accepted ? o.sent : o.initial.value)) errors.push('wrong saved value after interruption');
  if (o.final.value !== o.revised) errors.push('wrong final saved value');
  if (o.sawConflict !== accepted) errors.push('wrong retry conflict outcome');
  const versions = [o.initial.version, o.saved.version, o.final.version];
  if (!versions.every(v => typeof v === 'string' && /^(0|[1-9][0-9]*)$/.test(v))) return [...errors, 'invalid version observation'];
  const [initial, saved, final] = versions.map(BigInt);
  if (saved !== initial + (accepted ? 1n : 0n)) errors.push('wrong interruption version increment');
  if (final !== initial + (accepted ? 2n : 1n)) errors.push('wrong final version increment');
  return errors;
}

export function recoveryVerdict(observation) {
  return recoveryHistoryVerdict(observation, observation.sent);
}

function exactKeys(value, keys) {
  return !!value && typeof value === 'object' && !Array.isArray(value) &&
    Object.keys(value).sort().join(',') === [...keys].sort().join(',');
}

// The typing lane observes a distinct retained draft, rather than changing the
// ordinary Resume oracle to pretend that the pre-edit submitted draft remained.
export function typingRecoveryVerdict(observation) {
  const expected = ['edited', 'final', 'forwardedHeldPosts', 'heldStatus', 'initial',
    'posts', 'retained', 'revised', 'saved', 'sawConflict', 'sent', 'stage'];
  if (!exactKeys(observation, expected) ||
      !exactKeys(observation.initial, ['value', 'version']) ||
      !exactKeys(observation.retained, ['value', 'version']) ||
      !exactKeys(observation.saved, ['value', 'version']) ||
      !exactKeys(observation.final, ['value', 'version'])) return ['incomplete typing observation'];
  if (typeof observation.sent !== 'string' || typeof observation.edited !== 'string' ||
      typeof observation.revised !== 'string') return ['invalid typing draft observation'];
  const errors = [];
  if (observation.edited === observation.sent) errors.push('typing did not retain a distinct edited draft');
  if (observation.revised !== observation.initial.value) errors.push('retry did not use the reverted baseline');
  return [...errors, ...recoveryHistoryVerdict(observation, observation.edited)];
}

function canonicalSettingsSnapshot(value) {
  if (typeof value !== 'string') return null;
  try {
    const parsed = JSON.parse(value);
    if (!Array.isArray(parsed) || parsed.length !== 3 || typeof parsed[0] !== 'string' ||
        typeof parsed[1] !== 'string' || typeof parsed[2] !== 'boolean') return null;
    const fields = {timezone: parsed[0], start_page: parsed[1], digest: parsed[2]};
    return settingsRecoveryValue(fields) === value ? fields : null;
  } catch (_) { return null; }
}

// Settings input is a distinct lane: each complete, canonical observation is
// checked against the one permitted control edit; no snapshot is repaired here.
export function settingsInputRecoveryVerdict(observation) {
  const expected = ['edited', 'final', 'forwardedHeldPosts', 'heldStatus', 'initial',
    'journey', 'posts', 'retained', 'revised', 'saved', 'sawConflict', 'sent', 'stage'];
  if (!exactKeys(observation, expected) ||
      !exactKeys(observation.initial, ['value', 'version']) ||
      !exactKeys(observation.retained, ['value', 'version']) ||
      !exactKeys(observation.saved, ['value', 'version']) ||
      !exactKeys(observation.final, ['value', 'version'])) return ['incomplete Settings input observation'];
  const snapshots = Object.fromEntries(['sent', 'edited', 'revised'].map(name => [name, canonicalSettingsSnapshot(observation[name])]).concat([
    ['initial', canonicalSettingsSnapshot(observation.initial.value)],
    ['retained', canonicalSettingsSnapshot(observation.retained.value)],
    ['saved', canonicalSettingsSnapshot(observation.saved.value)],
    ['final', canonicalSettingsSnapshot(observation.final.value)],
  ]));
  if (Object.values(snapshots).some(value => value === null)) return ['invalid Settings snapshot'];
  const baseline = settingsRecoveryValue({timezone: 'America/Phoenix', start_page: 'home', digest: true});
  let sent, edited;
  if (observation.journey === 'settings-selections') {
    sent = settingsRecoveryValue({timezone: 'UTC', start_page: 'activity', digest: false});
    edited = settingsRecoveryValue({timezone: 'Europe/London', start_page: 'activity', digest: false});
  } else if (observation.journey === 'settings-start-page') {
    sent = settingsRecoveryValue({timezone: 'UTC', start_page: 'activity', digest: false});
    edited = settingsRecoveryValue({timezone: 'UTC', start_page: 'home', digest: false});
  } else if (observation.journey === 'settings-checkbox') {
    sent = settingsRecoveryValue({timezone: 'America/Phoenix', start_page: 'home', digest: false});
    edited = baseline;
  } else return ['unknown Settings input journey'];
  const errors = [];
  if (observation.initial.value !== baseline) errors.push('wrong Settings baseline');
  if (observation.sent !== sent) errors.push('wrong submitted Settings draft');
  if (observation.edited !== edited) errors.push('wrong edited Settings control');
  if (observation.revised !== baseline) errors.push('retry did not use the Settings baseline');
  return [...errors, ...recoveryHistoryVerdict(observation, observation.edited)];
}
