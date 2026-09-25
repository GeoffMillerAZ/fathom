import fs from 'node:fs';
import {bootstrap} from './config.mjs';

const {base, variant, root, chromium, executablePath} = await bootstrap();
const shots = `${root}/shell-forms-${variant}-screenshots`;
fs.mkdirSync(shots, {recursive: true});
const report = {base, variant, startedAt: new Date().toISOString(), passes: [], failures: [], requests: [], blocked: [], console: [], pageErrors: [], dialogs: [], focus: [], limits: ['Synthetic process-memory sessions only; no production account, native client, human task timing, full keyboard, contrast or 200% browser zoom certification.', 'Native selectOption/fill set fixture values; only separately named Tab/Enter paths are keyboard traversal.', 'Expected HTTP error responses are recorded separately from JavaScript/CSP errors.']};
const check = (ok, name, detail = '') => (ok ? report.passes : report.failures).push({name, detail});
const browser = await chromium.launch({headless: true, executablePath});
report.browser = browser.version();
const contexts = [];
async function newContext(javaScriptEnabled = true) {
  const context = await browser.newContext({javaScriptEnabled, viewport:{width:1440,height:1000}, serviceWorkers:'block'});
  context.setDefaultTimeout(5000); contexts.push(context);
  await context.route('**/*', async route => {
    const req=route.request(), url=new URL(req.url());
    const allowed = url.origin===base && ((req.method()==='GET' && (['/shell','/shell/home','/shell/profile','/shell/settings','/shell/help','/shell/about'].includes(url.pathname) || ['/static/app.css','/static/shell-demo.js'].includes(url.pathname))) || (req.method()==='POST' && url.pathname==='/shell/action'));
    if (!allowed) { report.blocked.push({method:req.method(),url:req.url()}); return route.abort(); }
    if (req.method()==='POST') report.requests.push({path:url.pathname,method:'POST',origin:req.headers().origin});
    return route.continue();
  });
  await context.routeWebSocket('**/*', ws=>{report.blocked.push({websocket:ws.url()});ws.close();});
  context.on('page', page=>{
    page.on('pageerror',e=>report.pageErrors.push(String(e)));
    page.on('console',m=>{if(m.type()==='error') report.console.push({text:m.text(),url:m.location().url});});
    // This flow submits dirty forms or visits clean ones; no dialog is expected.
    page.on('dialog', async d=>{report.dialogs.push({type:d.type(),message:d.message(),decision:'dismiss unexpected dialog'});await d.dismiss();});
  });
  return context;
}
async function go(page, route, expected=200) {
  const response=await page.goto(base+route,{waitUntil:'load'});
  check(response.status()===expected, `GET ${route}`, response.status());
  check(response.headers()['referrer-policy']==='same-origin',`native form privacy policy ${route}`,response.headers()['referrer-policy']);
  return response;
}
async function post(page, button, expected, label) {
  const [response] = await Promise.all([page.waitForResponse(r=>r.request().method()==='POST' && new URL(r.url()).pathname==='/shell/action'),page.waitForNavigation({waitUntil:'load'}),button.click()]);
  check(response.status()===expected,label,`status=${response.status()}`);
  if (response.status()!==expected) throw new Error(`${label}: expected ${expected}, got ${response.status()}`);
  report.focus.push({label,id:await page.evaluate(()=>document.activeElement?.id||document.activeElement?.tagName)});
}
const field = page=>page.locator('#shell-display_name');
const save = page=>page.locator('[data-shell-save]');
async function fixture(page,{access='editor',availability='ready',fail=false}={}) {
  await page.getByText('Laboratory controls and evidence limits',{exact:true}).click();
  await page.locator('#shell-access').selectOption(access);
  await page.locator('#shell-availability').selectOption(availability);
  await page.locator('[name=fail_next_save]').setChecked(fail);
  await post(page,page.getByRole('button',{name:'Apply fixture',exact:true}),303,'fixture applied');
}
async function shot(page,name) {await page.screenshot({path:`${shots}/${name}.png`,fullPage:true});}
async function inspectAccountMenu(page,width,theme,mode) {
  // Pointer opens the native disclosure; Tab inspects every shared account
  // destination. This is not an exclusively keyboard or screen-reader audit.
  await page.locator('header details > summary').click();
  const accountLinks=page.locator('nav[aria-label="Account destinations"] a');
  check(await accountLinks.count()===4,`account destinations complete ${mode} ${width}/${theme}`);
  for(const [i,destination] of ['profile','settings','help','about'].entries()) {
    await page.keyboard.press('Tab');
    const link=accountLinks.nth(i);
    const observed=await link.evaluate(n=>{const r=n.getBoundingClientRect();return {href:n.getAttribute('href'),focused:n===document.activeElement,current:n.getAttribute('aria-current'),height:r.height,left:r.left,right:r.right,viewport:innerWidth};});
    check(observed.href===`/shell/${destination}` && observed.focused && observed.current===(destination==='settings'?'page':null),`account keyboard route ${destination} ${mode} ${width}/${theme}`,observed);
    check(observed.height>=44 && observed.left>=0 && observed.right<=observed.viewport,`account target fits ${destination} ${mode} ${width}/${theme}`,observed);
  }
  await page.screenshot({path:`${shots}/account-${mode}-${width}-${theme}.png`});
  await page.locator('header details > summary').click();
}
async function tabTo(page, selector, limit=35) {
  for(let i=0;i<limit;i++) {
    await page.keyboard.press('Tab');
    if(await page.locator(selector).evaluate(n=>n===document.activeElement)) return i+1;
  }
  throw new Error(`Tab traversal never reached ${selector}`);
}
try {
  const context=await newContext(), page=await context.newPage();
  // Actual keyboard skip-link activation, then a keyboard profile edit/save.
  await go(page,'/shell/profile');
  await page.keyboard.press('Tab');
  check(await page.evaluate(()=>document.activeElement?.getAttribute('href'))==='#shell-content','first Tab reaches skip link');
  await page.keyboard.press('Enter');
  check(await page.evaluate(()=>document.activeElement?.id)==='shell-content','Enter skip link focuses main');
  check(await save(page).isDisabled(),'unchanged form save disabled');
  const tabs=await tabTo(page,'#shell-display_name');
  await page.keyboard.press('Meta+A'); await page.keyboard.type('Keyboard Avery');
  check(await field(page).inputValue()==='Keyboard Avery','keyboard input replaces display name',`Tab count from main=${tabs}`);
  check(await page.locator('[data-shell-draft-status]').innerText()==='Unsaved changes','dirty status labels draft');
  await tabTo(page,'[data-shell-save]');
  const [keyboardResponse]=await Promise.all([page.waitForResponse(r=>r.request().method()==='POST'),page.waitForNavigation({waitUntil:'load'}),page.keyboard.press('Enter')]);
  check(keyboardResponse.status()===303,'keyboard native save',keyboardResponse.status());
  if(keyboardResponse.status()!==303) throw new Error('First native save refused');
  check(await field(page).inputValue()==='Keyboard Avery','saved profile survives redirect');
  check(await save(page).isDisabled(),'saved draft becomes clean');

  // Native HTML required validation, then server validation without script/DOM bypass.
  await field(page).fill(''); await save(page).click();
  check(await field(page).evaluate(n=>n.validity.valueMissing && n===document.activeElement),'native required validation focuses empty field');
  await field(page).fill('X'.repeat(81));
  await post(page,save(page),422,'server profile length validation');
  check(await field(page).inputValue()==='X'.repeat(81),'422 draft retained');
  check(await field(page).getAttribute('aria-invalid')==='true','422 input error association');
  await page.locator('[aria-label="Fields to correct"] a').click();
  check(await page.evaluate(()=>document.activeElement?.id)==='shell-display_name','linked error focuses input');
  await shot(page,'validation-1440-light');
  await field(page).fill('Validated Avery'); await post(page,save(page),303,'corrected validation saved');

  // Two genuine same-session tabs, stale form versions and explicit review.
  const other=await context.newPage(); await go(other,'/shell/profile');
  await field(page).fill('Retained conflict draft');
  await field(other).fill('Newer saved value'); await post(other,save(other),303,'second tab save');
  await post(page,save(page),409,'stale profile conflict');
  check(await field(page).inputValue()==='Retained conflict draft','conflict draft retained');
  check((await page.locator('[aria-labelledby=shell-conflict-title]').innerText()).includes('Newer saved value'),'conflict displays current saved value');
  check(await save(page).innerText()==='Apply reviewed draft','conflict requires explicit reviewed action');
  await page.setViewportSize({width:390,height:844}); await shot(page,'conflict-390-light');
  await post(page,save(page),303,'reviewed profile saves');

  // One-shot server save failure configured only from another synthetic tab.
  await go(other,'/shell/profile'); await fixture(other,{fail:true});
  await go(page,'/shell/profile'); await field(page).fill('Retry Avery');
  await post(page,save(page),503,'one-shot profile save failure');
  check(await field(page).inputValue()==='Retry Avery' && await save(page).isEnabled(),'failed profile retains editable retry');
  await shot(page,'save-failure-390-light');
  await post(page,save(page),303,'profile retry saves once');

  // Settings save and two-tab optimistic conflict preserve all draft fields.
  await go(page,'/shell/settings');
  await page.locator('#shell-timezone').selectOption('UTC');
  await page.locator('#shell-start_page').selectOption('activity');
  await page.locator('#shell-digest').uncheck();
  await post(page,save(page),303,'settings save');
  check(await page.locator('#shell-timezone').inputValue()==='UTC' && await page.locator('#shell-start_page').inputValue()==='activity' && !await page.locator('#shell-digest').isChecked(),'settings persisted');
  await go(other,'/shell/settings');
  await page.locator('#shell-timezone').selectOption('Europe/London');
  await page.locator('#shell-digest').check();
  await other.locator('#shell-timezone').selectOption('America/Phoenix'); await post(other,save(other),303,'second settings tab save');
  await post(page,save(page),409,'settings conflict');
  check(await page.locator('#shell-timezone').inputValue()==='Europe/London' && await page.locator('#shell-digest').isChecked(),'settings conflict retains draft');
  await post(page,save(page),303,'reviewed settings save');

  // Appearance is browser-local, independent of saved account version.
  const savedVersion=await page.locator('form[data-shell-edit] [name=version]').inputValue();
  const themeColors={};
  for(const theme of ['dark','light']) {
    await page.locator('[data-appkit-appearance-select]').selectOption(theme);
    check(await page.locator('[data-appkit-appearance-select]').isEnabled() && await page.locator('[data-appkit-appearance-status]').innerText()==='Saved on this device.',`mounted appearance replaces fallback ${theme}`);
    check(await page.locator('html').getAttribute('data-theme')===theme,`appearance applies ${theme}`);
    themeColors[theme]=await page.locator('body').evaluate(n=>getComputedStyle(n).backgroundColor);
    check(await page.locator('form[data-shell-edit] [name=version]').inputValue()===savedVersion,`appearance doesn't mutate account ${theme}`);
    for(const width of [390,1440]) {
      await page.setViewportSize({width,height:1000});
      const geometry=await page.evaluate(()=>({client:document.documentElement.clientWidth,scroll:document.documentElement.scrollWidth,main:document.querySelectorAll('main').length}));
      check(geometry.scroll<=geometry.client,`settings ${width}/${theme} no page overflow`,geometry);
      check(geometry.main===1,`settings ${width}/${theme} one main`,geometry.main);
      await shot(page,`settings-${width}-${theme}`);
      await inspectAccountMenu(page,width,theme,'scripts');
    }
    await go(page,'/shell/profile'); check(await page.locator('html').getAttribute('data-theme')===theme,`appearance retained on route ${theme}`);
    await go(page,'/shell/settings');
  }
  await page.locator('[data-appkit-appearance-select]').selectOption('system');
  await page.emulateMedia({colorScheme:'dark'});
  await page.waitForFunction(()=>window.AppKitAppearance.get().effective==='dark');
  check(await page.locator('html').getAttribute('data-theme')===null && await page.locator('body').evaluate(n=>getComputedStyle(n).backgroundColor)===themeColors.dark,'system CSS follows dark device without override');
  await page.emulateMedia({colorScheme:'light'});
  await page.waitForFunction(()=>window.AppKitAppearance.get().effective==='light');
  check(await page.locator('html').getAttribute('data-theme')===null && await page.locator('body').evaluate(n=>getComputedStyle(n).backgroundColor)===themeColors.light,'system CSS follows light device without override');
  check(themeColors.dark!==themeColors.light,'appearance schemes have distinct computed backgrounds',themeColors);

  // Permission revoked after loading a valid form. No tampering with disabled fields.
  await go(page,'/shell/profile'); await field(page).fill('Private unsubmitted draft');
  await go(other,'/shell/profile'); await fixture(other,{access:'viewer'});
  await post(page,save(page),403,'permission rechecked on stale form');
  check((await page.locator('#shell-retained-draft').inputValue()).includes('Private unsubmitted draft'),'denied draft available to copy');
  check(await field(page).isDisabled(),'viewer cannot edit'); await shot(page,'viewer-draft-1440-light');
  await go(other,'/shell/profile'); await fixture(other);

  for(const state of ['loading','error']) {
    await go(other,'/shell/profile'); await fixture(other,{availability:state});
    await go(page,'/shell/profile',state==='error'?503:200);
    check(await field(page).count()===0,`${state} omits editable account form`);
    await shot(page,`${state}-1440-light`);
    await post(page,page.getByRole('button',{name:state==='loading'?'Finish simulated load':'Retry loading account',exact:true}),303,`${state} explicit recovery`);
    check(await field(page).inputValue()==='Retry Avery',`${state} preserves saved profile`);
  }

  // Sign out another tab while a draft exists, then explicitly recover.
  await go(page,'/shell/profile'); await field(page).fill('Signed-out draft');
  await go(other,'/shell/profile'); await other.getByText('My account',{exact:true}).click();
  await post(other,other.getByRole('button',{name:'Sign out of demo',exact:true}),303,'signout');
  await post(page,save(page),401,'signed-out stale form');
  check((await page.locator('#shell-retained-draft').inputValue()).includes('Signed-out draft'),'signed-out draft retained');
  await post(page,page.locator('#shell-content').getByRole('button',{name:'Sign in to demo',exact:true}),303,'explicit sign-in recovery');
  check(await field(page).inputValue()==='Retry Avery','signin reloads saved not unsubmitted data');
  await go(page,'/shell/about');
  const diagnostic=await page.locator('#shell-diagnostics').inputValue();
  check(/App-kit shell laboratory/.test(diagnostic) && !/Retry Avery|alex@example|csrf|\/Users\/|[0-9a-f]{64}/.test(diagnostic),'safe diagnostic text omits account/token/path');
  await go(page,'/shell/help');
  const disclosure=page.getByText('Report a problem without exposing account data',{exact:true});
  await disclosure.click(); await page.getByRole('link',{name:'View safe build information',exact:true}).click(); await page.waitForLoadState('load');
  check(new URL(page.url()).pathname==='/shell/about','help recovery destination works');

  // Independent no-script browser session; no hand-crafted network submission.
  const nojs=await newContext(false), plain=await nojs.newPage();
  await go(plain,'/shell/profile'); await field(plain).fill('No Script Avery');
  await post(plain,save(plain),303,'no-script native profile save');
  check(await field(plain).inputValue()==='No Script Avery','no-script profile persisted');
  await field(plain).fill('Z'.repeat(81)); await post(plain,save(plain),422,'no-script validation');
  check(await field(plain).inputValue()==='Z'.repeat(81),'no-script rejected draft retained');
  await go(plain,'/shell/settings'); await plain.locator('#shell-timezone').selectOption('UTC');
  await post(plain,save(plain),303,'no-script settings save');
  check(await plain.locator('#shell-timezone').inputValue()==='UTC','no-script settings persisted');
  check(await plain.locator('[data-appkit-appearance-select]').isDisabled(),'no-script appearance honestly unavailable');
  for (const theme of ['light','dark']) {
    await plain.emulateMedia({colorScheme:theme});
    for (const width of [1440,390]) {
      await plain.setViewportSize({width,height:1000});
      await inspectAccountMenu(plain,width,theme,'no-script');
      const appearance=plain.locator('[data-appkit-appearance]');
      check(await appearance.locator('[data-appkit-appearance-status]').count()===1 && await appearance.locator('[data-appkit-appearance-status]').innerText()==='Appearance controls require JavaScript.',`one truthful no-script appearance status ${width}/${theme}`);
      check(!/loading|scripts are disabled/i.test(await appearance.innerText()),`no conflicting no-script explanation ${width}/${theme}`);
      check(await appearance.locator('select').isDisabled() && !await appearance.locator('[data-appkit-appearance-retry]').isVisible(),`no-script controls stay honestly unavailable ${width}/${theme}`);
      await appearance.evaluate(element=>element.scrollIntoView({block:'center',behavior:'instant'}));
      const bounds=await appearance.boundingBox();
      check(bounds && bounds.x>=0 && bounds.x+bounds.width<=width+1,`no-script appearance fits width ${width}/${theme}`);
      await plain.screenshot({path:`${shots}/settings-no-script-${width}-${theme}-viewport.png`});
    }
  }
} catch(error) {check(false,'harness execution',String(error.stack||error));}
finally {
  check(report.blocked.length===0,'no outside request or WebSocket',report.blocked);
  check(report.pageErrors.length===0,'no JavaScript page errors',report.pageErrors);
  const expectedError=entry=> /^Failed to load resource: the server responded with a status of (401|403|409|422|503) \(/.test(entry.text) && ['/shell/action','/shell/profile','/shell/settings'].some(route=>entry.url===base+route);
  report.expectedConsole=report.console.filter(expectedError);
  const unexpectedConsole=report.console.filter(entry=>!expectedError(entry));
  check(unexpectedConsole.length===0,'no unexpected console or CSP errors',unexpectedConsole);
  const errorKeys=report.expectedConsole.map(entry=>`${new URL(entry.url).pathname}:${entry.text.match(/status of (\d+)/)[1]}`).sort();
  const expectedKeys=['/shell/action:422','/shell/action:409','/shell/action:503','/shell/action:409','/shell/action:403','/shell/profile:503','/shell/profile:503','/shell/profile:401','/shell/action:401','/shell/action:422'].sort();
  check(JSON.stringify(errorKeys)===JSON.stringify(expectedKeys),'exact intentional HTTP error console multiset',errorKeys);
  check(report.dialogs.length===0,'no unexpected browser dialogs',report.dialogs);
  report.finishedAt=new Date().toISOString();
  fs.writeFileSync(`${root}/shell-forms-${variant}-report.json`,JSON.stringify(report,null,2)+'\n');
  await browser.close();
}
console.log(JSON.stringify({passes:report.passes.length,failures:report.failures,report:`${root}/shell-forms-${variant}-report.json`},null,2));
process.exitCode=report.failures.length?1:0;
