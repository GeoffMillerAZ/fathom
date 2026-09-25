// Actual native-form navigation through an isolated loopback fault transport.
// No app hooks, server patches, real accounts or automatic replay.
import fs from 'node:fs';
import {bootstrap} from './config.mjs';
import {recoveryTransport} from './recovery_transport.mjs';
import {recoveryVerdict, settingsRecoveryValue, typingRecoveryVerdict, settingsInputRecoveryVerdict} from './recovery_assertions.mjs';

const {base: target, variant, root, chromium, executablePath} = await bootstrap();
const input = process.env.APPKIT_RECOVERY_INPUT || 'browser-stop';
if (!['browser-stop', 'inline-resume', 'typing', 'settings-input'].includes(input)) throw new Error('APPKIT_RECOVERY_INPUT must be browser-stop, inline-resume, typing or settings-input');
const editInput = input === 'typing' || input === 'settings-input';
const onlyCase = process.env.APPKIT_RECOVERY_CASE;
let base;
const report = {
  target, variant, input, onlyCase, startedAt: new Date().toISOString(), cases: [], passes: [], failures: [],
  blocked: [], pageErrors: [], console: [], dialogs: [], expectedDialogs: [],
  limits: [
    'Synthetic Gallery sessions and native Chrome forms only; not consumer, production, native-host or full accessibility proof.',
    'A loopback proxy withholds a native submission before forwarding, or forwards it once and withholds its accepted response. This does not model every network/browser failure.',
    input === 'browser-stop' ? 'CDP Page.stopLoading invokes browser-level Stop before keyboard recovery. This does not prove inline cancellation or physical keyboard/browser-toolbar operation.' :
      input === 'inline-resume' ? 'CDP Input key events activate inline Resume while navigation is pending; no Page.stopLoading or app-handler invocation is used. This is automated browser input, not physical keyboard/native-host proof.' :
        input === 'typing' ? 'CDP inserts one synthetic ASCII suffix into the focused Profile text control while its native submission is held. It is automated text-input evidence only, not physical keyboard, IME, Settings or native-host proof.' :
          'CDP Shift+Tab reaches a Settings select or checkbox while its native submission is held; native letter type-ahead or Space edits that control. Automated keyboard input only, not physical keyboard, IME, pointer/menu, other-engine or native-host proof.',
    input === 'typing' ? 'Typing cases are Profile text input only. The initial submission uses Enter while the field retains focus; later explicit retries use the ordinary Save control.' :
      input === 'settings-input' ? 'Settings input cases use native Save then keyboard control edits; later explicit retries use Save. Select type-ahead edits are separated by two seconds, not a throughput/performance claim. Both selections and checkbox serialization are checked independently.' :
        'Keyboard traversal activates Resume editing for Profile, all Settings controls, and a checkbox-only draft; fixture filling and other submissions use pointer/form controls.',
    'No automatic repost means none during these completed journeys, supported separately by source and VM tests.',
  ],
};
const check = (ok, name, detail) => {
  (ok ? report.passes : report.failures).push({name, ...(detail === undefined ? {} : {detail})});
  if (!ok) throw new Error(name);
};
async function bounded(promise, label) {
  let timer;
  try {
    return await Promise.race([promise, new Promise((_, reject) => {
      timer = setTimeout(() => reject(new Error(`Timed out: ${label}`)), 8000);
    })]);
  } finally { clearTimeout(timer); }
}
function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  promise.catch(() => {});
  return {promise, resolve, reject};
}
const browser = await chromium.launch({headless: true, executablePath});
report.browser = browser.version();
const allJourneys = [
  {id: 'profile', path: '/shell/profile', focus: 'shell-display_name'},
  {id: 'settings-selections', path: '/shell/settings', focus: 'shell-timezone'},
  {id: 'settings-checkbox', path: '/shell/settings', focus: 'shell-digest'},
];
const journeys = input === 'typing' ? [allJourneys[0]] : input === 'settings-input' ?
  [allJourneys[1], {id:'settings-start-page', path:'/shell/settings', focus:'shell-start_page'}, allJourneys[2]] : allJourneys;
const save = page => page.locator('[data-shell-save]');
const version = page => page.locator('form[data-shell-edit] [name=version]').inputValue();
const status = page => page.locator('[data-shell-draft-status]');
const draftValue = (journey, fields) => journey.id === 'profile' ? fields.display_name : settingsRecoveryValue(fields);
async function readFields(page, journey) {
  if (journey.id === 'profile') return {display_name: await page.locator('#shell-display_name').inputValue()};
  return {timezone: await page.locator('#shell-timezone').inputValue(),
    start_page: await page.locator('#shell-start_page').inputValue(),
    digest: await page.locator('#shell-digest').isChecked()};
}
async function fillFields(page, fields) {
  for (const [name, value] of Object.entries(fields)) {
    const control = page.locator(`#shell-${name}`);
    if (name === 'display_name') await control.fill(value);
    else if (name === 'digest') await control.setChecked(value);
    else await control.selectOption(value);
  }
}
async function readSaved(page, theme, journey) {
  const response = await page.goto(`${base}${journey.path}?theme=${theme}`, {waitUntil: 'load'});
  check(response.status() === 200, 'saved-state GET succeeds', response.status());
  const fields = await readFields(page, journey);
  return {value: draftValue(journey, fields), fields, version: await version(page)};
}
async function submit(page, expected) {
  const [response] = await Promise.all([
    page.waitForResponse(r => r.request().method() === 'POST' && r.url() === `${base}/shell/action`),
    page.waitForNavigation({waitUntil: 'load'}),
    save(page).click(),
  ]);
  check(response.status() === expected, 'explicit submission result', {expected, actual: response.status()});
  return response.status();
}
async function submitFocusedTextWithEnter(cdp) {
  await bounded(cdp.send('Input.dispatchKeyEvent', {type:'keyDown', key:'Enter', code:'Enter', windowsVirtualKeyCode:13, text:'\r', unmodifiedText:'\r'}), 'typing Enter down');
  await bounded(cdp.send('Input.dispatchKeyEvent', {type:'keyUp', key:'Enter', code:'Enter', windowsVirtualKeyCode:13}), 'typing Enter up');
}
async function insertTypingText(cdp, text) {
  if (typeof text !== 'string' || !text) throw new Error('typing text must be a non-empty string');
  await bounded(cdp.send('Input.insertText', {text}), 'typing text insertion');
}
async function settingsKey(cdp, name) {
  const keys = {previous: {key:'Tab', code:'Tab', windowsVirtualKeyCode:9, modifiers:8},
    london: {key:'l', code:'KeyL', windowsVirtualKeyCode:76, text:'l'},
    utc: {key:'c', code:'KeyC', windowsVirtualKeyCode:67, text:'c'},
    overview: {key:'o', code:'KeyO', windowsVirtualKeyCode:79, text:'o'},
    activity: {key:'a', code:'KeyA', windowsVirtualKeyCode:65, text:'a'},
    toggle: {key:' ', code:'Space', windowsVirtualKeyCode:32}};
  const key = keys[name];
  if (!key) throw new Error('Unknown Settings input gesture');
  const {text, ...physical} = key;
  for (const type of ['keyDown', 'keyUp']) await bounded(cdp.send('Input.dispatchKeyEvent', {
    type, ...physical, ...(type === 'keyDown' && text ? {text, unmodifiedText:text} : {}),
  }), `Settings ${name} ${type}`);
}
async function changeSettingsControl(cdp, journey, reverse = false) {
  // Native option-label type-ahead, not DOM value assignment or popup simulation.
  // Separate new label searches; otherwise consecutive letters are one prefix.
  if (reverse && journey.id !== 'settings-checkbox') await new Promise(resolve => setTimeout(resolve, 2000));
  await settingsKey(cdp, journey.id === 'settings-checkbox' ? 'toggle' :
    journey.id === 'settings-start-page' ? (reverse ? 'activity' : 'overview') : (reverse ? 'utc' : 'london'));
}
async function retainedState(page, journey) {
  const state = await page.evaluate(() => {
      const form = document.querySelector('form[data-shell-edit]');
      const save = form?.querySelector('[data-shell-save]');
      const active = document.activeElement;
      const rect = active?.getBoundingClientRect();
      const hit = rect && document.elementFromPoint(rect.x + rect.width / 2, rect.y + rect.height / 2);
      return {url: location.pathname, ready: document.readyState,
        disabled: save?.disabled, busy: form?.getAttribute('aria-busy'),
        status: form?.querySelector('[data-shell-draft-status]')?.textContent,
        resumeHidden: form?.querySelector('[data-shell-resume]')?.hidden,
        activeID: document.activeElement?.id || '',
        focusInViewportAndCenterClear: !!rect && rect.width > 0 && rect.height > 0 &&
          rect.top >= 0 && rect.left >= 0 && rect.bottom <= innerHeight && rect.right <= innerWidth &&
          (hit === active || active?.contains(hit)),
        version: form?.querySelector('[name=version]')?.value,
        resumeFocused: document.activeElement?.matches('[data-shell-resume]') || false};
    });
  const fields = await readFields(page, journey);
  return {...state, fields, value: draftValue(journey, fields)};
}
async function resumeWithKeyboard(page, journey) {
  for (let tabs = 1; tabs <= 60; tabs++) {
    await bounded(page.keyboard.press('Tab'), 'pending keyboard Tab');
    if ((await retainedState(page, journey)).resumeFocused) {
      await bounded(page.keyboard.press('Enter'), 'pending keyboard Enter');
      return tabs;
    }
  }
  throw new Error('Keyboard could not reach Resume editing');
}

try {
  for (const journey of journeys) for (const stage of ['before-dispatch', 'after-acceptance']) {
    for (const theme of ['light', 'dark']) for (const width of [390, 1440]) {
      const name = `${journey.id}-${stage}-${theme}-${width}`;
      if (onlyCase && name !== onlyCase) continue;
      const result = {name, input, journey: journey.id, stage, theme, width, posts: 0, forwardedHeldPosts: 0, heldStatus: null, submissions: [], finished: false};
      report.cases.push(result);
      const transport = await recoveryTransport({target, stage, result});
      base = transport.origin;
      result.origin = base;
      let expectedDeparture;
      const context = await browser.newContext({viewport: {width, height: 1000}, serviceWorkers: 'block', reducedMotion: 'reduce'});
      context.setDefaultTimeout(5000);
      context.on('page', page => {
        page.on('pageerror', e => report.pageErrors.push({name, error: String(e)}));
        page.on('console', m => { if (m.type() === 'error') report.console.push({name, text: m.text(), url: m.location().url}); });
        page.on('dialog', async d => {
          const expected = expectedDeparture;
          if (expected && expected.page === page && !expected.seen && d.type() === 'beforeunload') {
            expected.seen = true;
            const receipt = {name, type: d.type(), dismissed: false};
            try {
              await d.dismiss();
              receipt.dismissed = true;
              report.expectedDialogs.push(receipt);
              expected.resolve(receipt);
            } catch (error) { expected.reject(error); }
            return;
          }
          report.dialogs.push({name, type: d.type()});
          if (expected) expected.reject(new Error(`unexpected dialog during expected departure: ${d.type()}`));
          await d.dismiss();
        });
      });
      await context.routeWebSocket('**/*', ws => {report.blocked.push({name, websocket: ws.url()}); ws.close();});
      await context.route('**/*', async route => {
        const request = route.request(), url = new URL(request.url());
        const get = request.method() === 'GET' && [journey.path, '/static/app.css', '/static/shell-demo.js'].includes(url.pathname);
        const post = request.method() === 'POST' && url.pathname === '/shell/action' && !url.search;
        if (url.origin !== base || (!get && !post)) {
          report.blocked.push({name, method: request.method(), url: request.url()});
          return route.abort();
        }
        if (post) {
          const form = new URLSearchParams(request.postData() || '');
          // Only synthetic non-secret controls; never record cookies or CSRF.
          result.submissions.push({action: form.get('action'), version: form.get('version'),
            display_name: form.getAll('display_name'), timezone: form.getAll('timezone'),
            start_page: form.getAll('start_page'), digest: form.getAll('digest')});
        }
        return route.continue();
      });
      try {
        const page = await context.newPage(), observer = await context.newPage();
        const initial = await readSaved(page, theme, journey);
        result.initial = initial;
        if (input === 'settings-input') check(initial.value === settingsRecoveryValue({timezone:'America/Phoenix',start_page:'home',digest:true}), `${name}: exact initial Settings fixture`);
        check(await page.locator('html').getAttribute('data-theme') === theme, `${name}: requested scheme attribute is present`);
        const cdp = await context.newCDPSession(page);
        await page.bringToFront();
        const sentFields = journey.id === 'profile' ? {display_name: `Submitted ${theme} ${width}`} :
          journey.id === 'settings-checkbox' ? {timezone: 'America/Phoenix', start_page: 'home', digest: false} :
          {timezone: 'UTC', start_page: 'activity', digest: false};
        const revisedFields = editInput ? {...initial.fields} :
          journey.id === 'profile' ? {display_name: `Reviewed ${theme} ${width}`} :
          {timezone: 'Europe/London', start_page: 'home', digest: true};
        const sent = draftValue(journey, sentFields), revised = draftValue(journey, revisedFields);
        const typingSuffix = input === 'typing' ? ` typed ${theme} ${width}` : null;
        const edited = input === 'typing' ? `${sent}${typingSuffix}` : input === 'settings-input' ?
          settingsRecoveryValue(journey.id === 'settings-checkbox' ? {...sentFields, digest:true} :
            journey.id === 'settings-start-page' ? {...sentFields, start_page:'home'} : {...sentFields, timezone:'Europe/London'}) : null;
        result.sent = sentFields;
        result.revised = revisedFields;
        await fillFields(page, sentFields);
        if (input === 'typing') {
          await page.locator('#shell-display_name').focus();
          check(await page.locator('#shell-display_name').evaluate(field => document.activeElement === field), `${name}: Enter submits from the focused Profile text field`);
        }
        transport.arm();
        if (input === 'typing') await submitFocusedTextWithEnter(cdp);
        else await save(page).click({noWaitAfter: true});
        await bounded(transport.reached, `${name} transport hold`);
        check(result.posts === 1, `${name}: exactly one interrupted submission`);
        check(result.forwardedHeldPosts === (stage === 'after-acceptance' ? 1 : 0), `${name}: exact dispatch lane`);
        check(result.heldStatus === (stage === 'after-acceptance' ? 303 : null), `${name}: acceptance observed only when dispatched`);
        if (input === 'typing') {
          const first = result.submissions[0];
          check(result.submissions.length === 1 && first?.action === 'save-profile' &&
            first.version === initial.version && JSON.stringify(first.display_name) === JSON.stringify([sentFields.display_name]),
          `${name}: held POST contains the exact pre-typing Profile draft and original version`, first);
        }
        const pendingShot = await bounded(cdp.send('Page.captureScreenshot', {format: 'png'}), 'capture pending navigation');
        fs.writeFileSync(`${root}/${name}-pending.png`, Buffer.from(pendingShot.data, 'base64'));
        if (input === 'browser-stop') {
          // Chromium deferred Runtime/AX work during held navigation in prior
          // probes. This existing lane deliberately tests browser Stop recovery.
          await bounded(cdp.send('Page.stopLoading'), 'browser-level Stop');
          result.interruption = 'CDP Page.stopLoading';
        } else if (input === 'inline-resume') {
          // Save owns the tab-order position when submission disables it. The
          // preceding now-visible Resume control is reached with Shift+Tab.
          // Raw browser input remains usable without Runtime/AX inspection.
          for (const type of ['keyDown', 'keyUp']) await bounded(cdp.send('Input.dispatchKeyEvent', {type, key:'Tab', code:'Tab', windowsVirtualKeyCode:9, modifiers:8}), 'pending Shift+Tab');
          const focusShot = await bounded(cdp.send('Page.captureScreenshot', {format:'png'}), 'pending keyboard focus screenshot');
          fs.writeFileSync(`${root}/${name}-pending-focus.png`, Buffer.from(focusShot.data,'base64'));
          const disconnectedBeforeActivation = await Promise.race([
            transport.interrupted.then(() => true),
            new Promise(resolve => setTimeout(() => resolve(false), 200)),
          ]);
          check(!disconnectedBeforeActivation, `${name}: tabbing alone leaves the held navigation connected`);
          await bounded(cdp.send('Input.dispatchKeyEvent', {type:'keyDown',key:'Enter',code:'Enter',windowsVirtualKeyCode:13,text:'\r',unmodifiedText:'\r'}), 'pending Enter down');
          await bounded(cdp.send('Input.dispatchKeyEvent', {type:'keyUp',key:'Enter',code:'Enter',windowsVirtualKeyCode:13}), 'pending Enter up');
          result.interruption = 'inline Resume via Shift+Tab/Enter';
        } else {
          // The edited control must itself recover the native navigation.
          // This intentionally does not use Resume, Page.stopLoading or Runtime.
          if (input === 'settings-input') {
            // From the disabled Save position: Resume, Discard, digest, start,
            // timezone. These raw browser keys remain usable during navigation.
            for (let step = 0; step < (journey.id === 'settings-checkbox' ? 3 : journey.id === 'settings-start-page' ? 4 : 5); step++) await settingsKey(cdp, 'previous');
            const focusShot = await bounded(cdp.send('Page.captureScreenshot', {format:'png'}), 'pending Settings focus screenshot');
            fs.writeFileSync(`${root}/${name}-pending-focus.png`, Buffer.from(focusShot.data, 'base64'));
          }
          const disconnectedBeforeTyping = await Promise.race([
            transport.interrupted.then(() => true),
            new Promise(resolve => setTimeout(() => resolve(false), 200)),
          ]);
          check(!disconnectedBeforeTyping, `${name}: held navigation stays connected before typing`);
          if (input === 'typing') await insertTypingText(cdp, typingSuffix);
          else await changeSettingsControl(cdp, journey);
          result.interruption = input === 'typing' ? 'CDP Input.insertText while Profile submission is held' :
            `CDP keyboard ${journey.id === 'settings-checkbox' ? 'Space' : journey.id === 'settings-start-page' ? 'Overview label type-ahead' : 'London label type-ahead'} while Settings submission is held`;
        }
        await bounded(transport.interrupted, `${name} browser disconnects held request`);
        check(result.posts === 1, `${name}: interruption completed without repost`);
        transport.release();
        await bounded(transport.settled, `${name} interrupted request settles`);
        const pending = await retainedState(page, journey);
        if (input === 'browser-stop') {
          check(pending.disabled === true, `${name}: duplicate save disabled while pending`);
          check(pending.busy === 'true', `${name}: pending state exposed`);
          check(pending.status?.startsWith('Saving changes.'), `${name}: pending status is not success`);
        }
        const expectedRetained = editInput ? edited : sent;
        check(pending.value === expectedRetained && pending.version === initial.version, `${name}: pending draft and original version retained`);

        const accepted = stage === 'after-acceptance';
        const saved = await readSaved(observer, theme, journey);
        result.saved = saved;
        check(saved.value === (accepted ? sent : initial.value), `${name}: independent GET proves saved value`);
        check(BigInt(saved.version) === BigInt(initial.version) + (accepted ? 1n : 0n), `${name}: saved version advances only for acceptance`);
        await page.bringToFront();
        if (input === 'browser-stop') result.resumeTabCount = await resumeWithKeyboard(page, journey);
        const recovered = await retainedState(page, journey);
        result.retained = recovered;
        check(recovered.busy === null && recovered.resumeHidden && !recovered.disabled, `${name}: recovery restores controls after ${input}`);
        check(recovered.status?.includes('The server may already have saved.'), `${name}: recovery exposes uncertainty after ${input}`);
        check(recovered.activeID === journey.focus, `${name}: recovery returns focus to the expected field`, recovered.activeID);
        check(recovered.focusInViewportAndCenterClear, `${name}: recovered field is within the viewport with an unobstructed center point`, recovered);
        check(await save(page).isEnabled() && await page.locator('[data-shell-resume]').isHidden(), `${name}: recovery restores controls`);
        check(await page.locator('form[data-shell-edit]').getAttribute('aria-busy') === null, `${name}: no stranded busy state`);
        check((await status(page).innerText()).includes('The server may already have saved.'), `${name}: recovery preserves outcome uncertainty`);
        check(draftValue(journey, await readFields(page, journey)) === expectedRetained && await version(page) === initial.version, `${name}: recovery retains original draft/version`);
        check(result.posts === 1, `${name}: recovery does not repost`);
        await page.screenshot({path: `${root}/${name}-resumed.png`, fullPage: true});

        // Read before retry. A subsequent edit is a new draft, not an automatic
        // replay of the first request, and must still use optimistic concurrency.
        check(JSON.stringify(await readSaved(observer, theme, journey)) === JSON.stringify(saved), `${name}: inspection does not mutate saved state`);
        if (editInput) {
          const secondSuffix = ` again ${theme} ${width}`;
          await page.bringToFront();
          if (input === 'typing') await insertTypingText(cdp, secondSuffix);
          else await changeSettingsControl(cdp, journey, true);
          const afterSecondInput = await retainedState(page, journey);
          check(afterSecondInput.value === (input === 'typing' ? `${edited}${secondSuffix}` : sent), `${name}: second control input retains the exact edited draft`, afterSecondInput.value);
          check(!afterSecondInput.disabled && afterSecondInput.status?.includes('The server may already have saved.'), `${name}: second control input preserves uncertainty and enabled save`);
          check(afterSecondInput.activeID === journey.focus && afterSecondInput.version === initial.version, `${name}: continued input preserves focus and original version`);
          await fillFields(page, revisedFields);
          const reverted = await retainedState(page, journey);
          check(reverted.value === initial.value && reverted.version === initial.version, `${name}: form controls revert to the original baseline without changing version`);
          check(!reverted.disabled && reverted.status?.includes('The server may already have saved.'), `${name}: reverted baseline remains explicitly retryable and outcome-unknown`);
          if (expectedDeparture) throw new Error('a departure receipt is already pending');
          expectedDeparture = {...deferred(), seen: false, page};
          const departureTarget = `${base}${journey.path}?theme=${theme}`;
          const navigation = page.goto(departureTarget, {waitUntil: 'load'}).then(
            () => ({rejected: false, name: null, message: null}),
            error => ({rejected: true, name: error?.name || 'Error', message: String(error?.message || '').slice(0, 1024)}),
          );
          let dialogReceipt, navigationResult;
          try {
            dialogReceipt = await bounded(expectedDeparture.promise, `${name} beforeunload dialog`);
            navigationResult = await bounded(navigation, `${name} canceled form navigation`);
          } finally { expectedDeparture = undefined; }
          const afterDeparture = await retainedState(page, journey);
          result.typing = {edited, second: afterSecondInput.value, reverted: reverted.value,
            departure: {target: departureTarget, type: dialogReceipt.type, dismissed: dialogReceipt.dismissed,
              rejected: navigationResult.rejected, rejection_name: navigationResult.name, rejection_message: navigationResult.message,
              retained: {url: afterDeparture.url, value: afterDeparture.value, version: afterDeparture.version,
                status: afterDeparture.status, disabled: afterDeparture.disabled, busy: afterDeparture.busy,
                resumeHidden: afterDeparture.resumeHidden}}};
          check(dialogReceipt.type === 'beforeunload' && dialogReceipt.dismissed && navigationResult.rejected,
            `${name}: form departure is canceled by the expected beforeunload dialog`, {dialogReceipt, navigationResult});
          check(navigationResult.name === 'Error' && navigationResult.message?.split('\n')[0] === `page.goto: net::ERR_ABORTED at ${departureTarget}`,
            `${name}: rejected navigation identifies beforeunload cancellation, not a timeout or unrelated failure`, navigationResult);
          check(afterDeparture.url === journey.path && afterDeparture.value === initial.value && afterDeparture.version === initial.version,
            `${name}: canceled form departure retains the current document, reverted draft and original version`, afterDeparture);
          check(afterDeparture.busy === null && afterDeparture.resumeHidden && !afterDeparture.disabled &&
            afterDeparture.status?.includes('The server may already have saved.'),
          `${name}: canceled form departure retains uncertainty and enabled save`, afterDeparture);
        } else await fillFields(page, revisedFields);
        const retryStatus = await submit(page, accepted ? 409 : 303);
        check(result.posts === 2, `${name}: only one explicit retry`);
        if (accepted) {
          check(draftValue(journey, await readFields(page, journey)) === revised, `${name}: conflict retains every revised control`);
          const conflict = await page.locator('[aria-labelledby=shell-conflict-title]').innerText();
          const expectedSaved = journey.id === 'profile' ? [sent] : [`${sentFields.timezone} · ${sentFields.start_page} · digest excluded`];
          check(expectedSaved.every(value => conflict.includes(value)), `${name}: conflict identifies accepted saved values`, conflict);
          check(await version(page) === saved.version, `${name}: review receives current version`);
          check(await save(page).innerText() === 'Apply reviewed draft', `${name}: conflict requires explicit review`);
          check(JSON.stringify(await readSaved(observer, theme, journey)) === JSON.stringify(saved), `${name}: rejected retry does not write`);
          await page.screenshot({path: `${root}/${name}-conflict.png`, fullPage: true});
          await submit(page, 303);
        }
        const final = await readSaved(observer, theme, journey);
        result.final = final;
        check(final.value === revised, `${name}: explicit recovery saves intended final value`);
        check(BigInt(final.version) === BigInt(initial.version) + (accepted ? 2n : 1n), `${name}: exact total accepted writes`);
        check(result.posts === (accepted ? 3 : 2), `${name}: no automatic or duplicate submission in completed journey`);
        check(await save(page).isDisabled(), `${name}: confirmed save returns clean controls`);
        check(result.submissions.length === result.posts, `${name}: browser and transport agree on submission count`);
        for (const [index, submission] of result.submissions.entries()) {
          const expected = index === 0 ? sentFields : revisedFields;
          check(submission.version === (index < 2 ? initial.version : saved.version), `${name}: POST ${index + 1} uses the reviewed version`);
          if (journey.id === 'profile') {
            check(submission.action === 'save-profile' && JSON.stringify(submission.display_name) === JSON.stringify([expected.display_name]), `${name}: POST ${index + 1} serializes profile draft`);
          } else {
            check(submission.action === 'save-settings' &&
              JSON.stringify(submission.timezone) === JSON.stringify([expected.timezone]) &&
              JSON.stringify(submission.start_page) === JSON.stringify([expected.start_page]) &&
              JSON.stringify(submission.digest) === JSON.stringify(expected.digest ? ['on'] : []),
            `${name}: POST ${index + 1} serializes both selections and checkbox omission`);
          }
        }
        const editedObservation = {stage, forwardedHeldPosts: result.forwardedHeldPosts,
          heldStatus: result.heldStatus, posts: result.posts, initial: {value: initial.value, version: initial.version}, sent, edited, revised,
          retained: {value: recovered.value, version: recovered.version}, saved: {value: saved.value, version: saved.version},
          final: {value: final.value, version: final.version}, sawConflict: retryStatus === 409};
        result.verdict = input === 'typing' ? typingRecoveryVerdict(editedObservation) : input === 'settings-input' ?
          settingsInputRecoveryVerdict({...editedObservation, journey:journey.id}) :
          recoveryVerdict({...result, initial, sent, revised, saved, final,
            retained: {value: recovered.value, version: recovered.version}, sawConflict: retryStatus === 409});
        check(result.verdict.length === 0, `${name}: shared canary-tested evidence contract`, result.verdict);
        check(result.transportErrors.length === 0, `${name}: no transport errors`, result.transportErrors);
        result.finished = true;
      } finally {
        transport.release();
        const cleanupErrors = [];
        try { await bounded(context.close(), `${name} browser context cleanup`); }
        catch (error) { cleanupErrors.push(error); }
        try { await bounded(transport.close(), `${name} transport cleanup`); }
        catch (error) { cleanupErrors.push(error); }
        if (cleanupErrors.length) throw new AggregateError(cleanupErrors, `${name} cleanup failed`);
      }
    }
  }
  check(report.cases.length === (onlyCase ? 1 : input === 'typing' ? 8 : 24) && report.cases.every(c => c.finished), 'all selected interruption journeys completed');
  check(report.blocked.length === 0, 'no outside request or WebSocket', report.blocked);
  check(report.pageErrors.length === 0, 'no page script errors', report.pageErrors);
  check(report.dialogs.length === 0, 'no unexpected browser dialogs', report.dialogs);
  const finishedTyping = report.cases.filter(c => ['typing', 'settings-input'].includes(c.input) && c.finished).map(c => c.name).sort();
  const expectedTyping = report.expectedDialogs.map(d => d.name).sort();
  check(editInput ? expectedTyping.length === finishedTyping.length &&
    JSON.stringify(expectedTyping) === JSON.stringify(finishedTyping) &&
    report.expectedDialogs.every(d => d.type === 'beforeunload' && d.dismissed) : report.expectedDialogs.length === 0,
  'expected beforeunload dialogs are exact for the selected recovery mode', report.expectedDialogs);
  const expectedConsole = report.console.filter(e => {
    const fixture = report.cases.find(c => c.name === e.name);
    return fixture.stage === 'after-acceptance' && e.url === `${fixture.origin}/shell/action` && /^Failed to load resource: the server responded with a status of 409 \(/.test(e.text);
  });
  const conflicts = report.cases.filter(c => c.stage === 'after-acceptance').length;
  check(expectedConsole.length === conflicts && report.console.length === conflicts, 'exact intentional conflict console responses', report.console);
} catch (error) {
  report.failures.push({name: 'harness execution',detail: String(error.stack || error)});
} finally {
  report.finishedAt = new Date().toISOString();
  fs.writeFileSync(`${root}/shell-recovery-${variant}-report.json`, JSON.stringify(report, null, 2) + '\n');
  await browser.close();
}
console.log(JSON.stringify({passes: report.passes.length, failures: report.failures, report: `${root}/shell-recovery-${variant}-report.json`}, null, 2));
process.exitCode = report.failures.length ? 1 : 0;
