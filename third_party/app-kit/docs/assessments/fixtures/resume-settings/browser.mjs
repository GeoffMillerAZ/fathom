import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import crypto from 'node:crypto'
import { pathToFileURL } from 'node:url'
import { spawnSync } from 'node:child_process'
const [origin, outputDir, playwrightEntry, root] = process.argv.slice(2)
if (!origin || new URL(origin).hostname !== '127.0.0.1') throw new Error('exact loopback origin required')
if (!outputDir || !playwrightEntry || !root) throw new Error('usage: browser.mjs ORIGIN EMPTY_SYSTEM_TEMP_DIR PLAYWRIGHT_ENTRY RAC_ROOT')
const out = fs.realpathSync(outputDir)
const tempRoot = fs.realpathSync(os.tmpdir()) + path.sep
if (!out.startsWith(tempRoot) || fs.readdirSync(out).length !== 0) throw new Error('output must be an existing empty real system-temp child')
fs.chmodSync(out, 0o700)
const { chromium } = await import(pathToFileURL(fs.realpathSync(playwrightEntry)).href)
const checks = []
const failures = []
const requests = []
const responses = []
const errors = []
const screenshots = []
const layoutDiagnostics = []
const expectedNavigation = [
  ['/today', 'Today'],
  ['/pipeline', 'Pipeline'],
  ['/discovery', 'Discovery'],
  ['/people', 'People'],
  ['/evidence', 'Evidence'],
  ['/editor', 'Editor'],
  ['/agent', 'Agent'],
  ['/settings', 'Settings'],
]
const probeClientID = `probe-${crypto.randomBytes(8).toString('hex')}.apps.googleusercontent.com`
const probeSecret = `probe-${crypto.randomBytes(24).toString('base64url')}`
const staleClientID = `stale-${crypto.randomBytes(8).toString('hex')}.apps.googleusercontent.com`
const staleSecret = `stale-${crypto.randomBytes(24).toString('base64url')}`
const check = (name, ok, detail = undefined) => {
  checks.push({ name, ok: Boolean(ok), ...(detail === undefined ? {} : { detail }) })
  if (!ok) failures.push(name)
}
const allowed = new Set([
  'GET /settings', 'HEAD /settings',
  'GET /assets/app.css', 'HEAD /assets/app.css',
  'GET /assets/palette.js', 'HEAD /assets/palette.js',
  'POST /settings/google/credentials',
])
const sha = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex')
const processIdentity = async browser => {
  const cdp = await browser.newBrowserCDPSession()
  try {
    const { processInfo } = await cdp.send('SystemInfo.getProcessInfo')
    const browsers = processInfo.filter(process => process.type === 'browser' && Number.isInteger(process.id) && process.id > 1)
    if (browsers.length !== 1) throw new Error(`expected one browser process, got ${browsers.length}`)
    const pid = browsers[0].id
    const command = spawnSync('ps', ['-ww', '-p', String(pid), '-o', 'command='], { encoding: 'utf8', maxBuffer: 1024 * 1024, timeout: 2000 })
    if (command.error || command.status !== 0 || !command.stdout.trim()) throw new Error(`cannot read browser command line ${pid}`)
    const commandLine = command.stdout.trim()
    if (/[a-z][a-z0-9+.-]*:\/\/[^/\s@]+@/i.test(commandLine)) throw new Error('refusing browser command line containing URL credentials')
    const userDataMatches = [...commandLine.matchAll(/(?:^|\s)--user-data-dir=([^\s]+)/g)]
    if (userDataMatches.length !== 1) throw new Error(`expected one browser user-data-dir, got ${userDataMatches.length}`)
    const rawUserData = userDataMatches[0][1]
    const processTmpRoot = fs.realpathSync(os.tmpdir())
    const userDataDir = fs.realpathSync(rawUserData)
    if (!userDataDir.startsWith(processTmpRoot + path.sep)) throw new Error(`browser user-data-dir is outside process tmp root: ${userDataDir}`)

    const ps = spawnSync('ps', ['-p', String(pid), '-o', 'lstart=,command='], { encoding: 'utf8', maxBuffer: 1024 * 1024, timeout: 2000 })
    const group = spawnSync('ps', ['-p', String(pid), '-o', 'pgid='], { encoding: 'utf8', maxBuffer: 1024 * 1024, timeout: 2000 })
    if (ps.error || ps.status !== 0 || !ps.stdout.trim()) throw new Error(`cannot identify browser process ${pid}`)
    if (group.error || group.status !== 0 || !/^\d+$/.test(group.stdout.trim())) throw new Error(`cannot identify browser process group ${pid}`)
    const pgid = Number(group.stdout.trim())
    if (pgid !== pid) throw new Error(`browser process group is not owned: pid=${pid} pgid=${pgid}`)
    return { pid, pgid, userDataDir, commandLine, psIdentity: ps.stdout.trim() }
  } finally {
    await cdp.detach().catch(() => {})
  }
}
const snap = async (page, name) => {
  const file = path.join(out, name)
  await page.screenshot({ path: file, fullPage: true })
  screenshots.push({ name, size: fs.statSync(file).size, sha256: sha(file) })
}
const installBoundary = async (context, tag, { blockStylesheet = false } = {}) => {
  await context.route('**/*', async route => {
    const req = route.request()
    const u = new URL(req.url())
    const key = `${req.method()} ${u.pathname}`
    if (u.origin !== origin || !allowed.has(key)) {
      failures.push(`${tag}: unexpected request`)
      return route.abort('blockedbyclient')
    }
    requests.push({ tag, method: req.method(), path: u.pathname })
    if (blockStylesheet && key === 'GET /assets/app.css') return route.abort('blockedbyclient')
    return route.continue()
  })
}
const submit = async (page, tag) => {
  const post = page.waitForResponse(r => r.request().method() === 'POST' && new URL(r.url()).pathname === '/settings/google/credentials')
  const navigation = page.waitForNavigation({ waitUntil: 'domcontentloaded' })
  await page.locator('#google-client-secret').press('Enter')
  const response = await post
  const finalResponse = await navigation
  check(`${tag}: POST uses 303`, response.status() === 303, response.status())
  check(`${tag}: redirected settings is 200`, finalResponse?.status() === 200, finalResponse?.status())
  return response
}
const errorFocus = async page => {
  const immediate = await page.locator('#credentials-error').evaluate(el => document.activeElement === el)
  if (immediate) return { immediate, eventual: true }
  const deadline = Date.now() + 1000
  let eventual = false
  while (Date.now() < deadline) {
    eventual = await page.locator('#credentials-error').evaluate(el => document.activeElement === el)
    if (eventual) break
    await new Promise(resolve => setTimeout(resolve, 25))
  }
  const active = await page.evaluate(() => ({ id: document.activeElement?.id || '', tag: document.activeElement?.tagName || '' }))
  return { immediate, eventual, active }
}
const auditLayout = async (page, tag) => {
  const layout = await page.evaluate(() => {
    const controls = [...document.querySelectorAll('#google-credentials-form input:not([type=hidden]), #google-credentials-form button')]
    return {
      minControlHeight: Math.min(...controls.map(el => el.getBoundingClientRect().height)),
      horizontalOverflow: Math.max(0, document.documentElement.scrollWidth - innerWidth),
    }
  })
  check(`${tag}: credential controls at least 44px`, layout.minControlHeight >= 44, layout.minControlHeight)
  check(`${tag}: no document horizontal overflow`, layout.horizontalOverflow === 0, layout.horizontalOverflow)
  return layout
}
const shellReadiness = async page => {
  let loadCompleted = false
  try {
    await page.waitForLoadState('load', { timeout: 2500 })
    loadCompleted = true
  } catch { /* The readiness check fails; do not retry until layout looks good. */ }
  const state = await page.evaluate(() => {
    const sheets = [...document.querySelectorAll('link[rel="stylesheet"]')].map(link => {
      let rules = null
      try { rules = link.sheet?.cssRules.length ?? null } catch { /* Not readable. */ }
      return { path: new URL(link.href).pathname, rules, disabled: link.disabled,
        mediaMatches: !link.media || matchMedia(link.media).matches }
    })
    const shell = document.querySelector('body > div.flex.min-h-screen')
    return { readyState: document.readyState, sheets, shellDisplay: shell ? getComputedStyle(shell).display : null }
  })
  return { ...state, loadCompleted, ready: loadCompleted && state.readyState === 'complete' &&
    state.sheets.length === 1 && state.sheets[0].path === '/assets/app.css' &&
    state.sheets[0].rules > 0 && !state.sheets[0].disabled && state.sheets[0].mediaMatches && state.shellDisplay === 'flex' }
}
const readShell = ({ expected, diagnose }) => {
    const describeNav = label => {
      const nav = document.querySelector(`nav[aria-label="${label}"]`)
      return nav ? {
        links: [...nav.querySelectorAll(':scope a')].map(link => [link.getAttribute('href'), link.querySelector('span:last-child')?.textContent.trim() || '']),
        current: [...nav.querySelectorAll(':scope a[aria-current="page"]')].map(link => link.getAttribute('href')),
      } : null
    }
    const skip = document.querySelector('body > a[href="#content"]')
    const main = document.querySelector('main#content')
    // Read the original oracle before diagnostic style/geometry reads can settle layout.
    const horizontalOverflow = Math.max(0, document.documentElement.scrollWidth - innerWidth)
    const diagnostic = diagnose || horizontalOverflow > 0 ? {
      at: performance.now(), readyState: document.readyState,
      viewport: { width: innerWidth, height: innerHeight, scrollX, scrollY,
        visualWidth: visualViewport?.width, visualOffsetLeft: visualViewport?.offsetLeft },
      documentWidth: document.documentElement.scrollWidth, bodyWidth: document.body.scrollWidth,
      active: { tag: document.activeElement?.tagName, id: document.activeElement?.id },
      stylesheets: [...document.querySelectorAll('link[rel="stylesheet"]')].map(link => {
        let rules = null
        try { rules = link.sheet?.cssRules.length ?? null } catch { rules = 'unavailable' }
        return { path: new URL(link.href).pathname, sheetPresent: Boolean(link.sheet), rules }
      }),
      offenders: [...document.querySelectorAll('body *')].flatMap(el => {
        const rect = el.getBoundingClientRect()
        if (!el.getClientRects().length || (rect.right <= innerWidth && rect.left >= 0 && el.scrollWidth <= el.clientWidth + 1)) return []
        const style = getComputedStyle(el)
        return [{ tag: el.tagName, id: el.id, classes: el.getAttribute('class'),
          left: rect.left, right: rect.right, width: rect.width, clientWidth: el.clientWidth, scrollWidth: el.scrollWidth,
          display: style.display, position: style.position, overflowX: style.overflowX,
          minWidth: style.minWidth, maxWidth: style.maxWidth }]
      }).slice(0, 24),
    } : null
    return {
      desktop: describeNav('Primary'),
      mobile: describeNav('Primary (mobile)'),
      expected,
      skip: skip ? { text: skip.textContent.trim(), href: skip.getAttribute('href') } : null,
      main: main ? { id: main.id, tabindex: main.getAttribute('tabindex') } : null,
      horizontalOverflow,
      width: innerWidth,
      diagnostic,
    }
}
const auditShell = async (page, tag) => {
  const readiness = await shellReadiness(page)
  check(`${tag}: shell stylesheet loaded before layout`, readiness.ready, readiness)
  const targetedDiagnostic = tag === 'nojs-390-invalid'
  const { diagnostic, ...shell } = await page.evaluate(readShell, { expected: expectedNavigation, diagnose: targetedDiagnostic })
  check(`${tag}: exact app-owned shell routes`, shell.desktop && shell.mobile &&
    JSON.stringify(shell.desktop.links) === JSON.stringify(expectedNavigation) &&
    JSON.stringify(shell.mobile.links) === JSON.stringify(expectedNavigation), shell)
  check(`${tag}: Settings is current in both navigation copies`,
    JSON.stringify(shell.desktop?.current) === JSON.stringify(['/settings']) &&
    JSON.stringify(shell.mobile?.current) === JSON.stringify(['/settings']), shell)
  check(`${tag}: shared skip and main identity`, shell.skip?.text === 'Skip to content' && shell.skip?.href === '#content' && shell.main?.id === 'content' && shell.main?.tabindex === '-1', shell)
  check(`${tag}: shell has no horizontal overflow`, shell.horizontalOverflow === 0, shell.horizontalOverflow)

  const diagnosticRun = diagnostic ? { tag, readiness, stages: [{ stage: 'layout-oracle', overflow: shell.horizontalOverflow, ...diagnostic }] } : null
  const recordDiagnostic = async stage => {
    const sample = await page.evaluate(readShell, { expected: expectedNavigation, diagnose: true })
    diagnosticRun.stages.push({ stage, overflow: sample.horizontalOverflow, ...sample.diagnostic })
    fs.writeFileSync(path.join(out, `${tag}-layout-diagnostic.json`), JSON.stringify(diagnosticRun, null, 2) + '\n', { mode: 0o600 })
  }
  if (diagnosticRun) {
    layoutDiagnostics.push(diagnosticRun)
    fs.writeFileSync(path.join(out, `${tag}-layout-diagnostic.json`), JSON.stringify(diagnosticRun, null, 2) + '\n', { flag: 'wx', mode: 0o600 })
    // Screenshots may await fonts/paint: the original geometry above, not this later
    // image, owns the failing instant. None of these observations replaces its result.
    await snap(page, `${tag}-shell-layout-diagnostic.png`)
    await recordDiagnostic('after-screenshot-before-input')
    await page.waitForTimeout(100)
    await recordDiagnostic('after-fixed-100ms-before-input')
  }

  if (shell.width >= 768) return
  const startingURL = page.url()
  const visited = []
  let summaryFocused = false
  for (let i = 0; i < 24; i++) {
    await page.keyboard.press('Tab')
    if (i === 0 && diagnosticRun) await recordDiagnostic('after-first-native-Tab')
    const active = await page.evaluate(() => ({
      tag: document.activeElement?.tagName || '',
      text: document.activeElement?.querySelector?.('span:last-child')?.textContent?.trim() || document.activeElement?.textContent?.trim() || '',
      href: document.activeElement?.getAttribute?.('href') || '',
    }))
    visited.push(active)
    if (active.tag === 'SUMMARY' && active.text === 'Menu') {
      summaryFocused = true
      break
    }
  }
  check(`${tag}: native Tab reaches mobile Menu without navigation`, summaryFocused && page.url() === startingURL, { visited, url: page.url() })
  if (!summaryFocused) return
  const summary = page.locator('details.md\\:hidden > summary')
  const summaryPaint = await summary.evaluate(el => {
    const rect = el.getBoundingClientRect()
    const style = getComputedStyle(el)
    return { height: rect.height, width: rect.width, visible: Boolean(el.getClientRects().length) && rect.top >= 0 && rect.bottom <= innerHeight,
      outlineStyle: style.outlineStyle, outlineWidth: parseFloat(style.outlineWidth), outlineColor: style.outlineColor }
  })
  check(`${tag}: focused mobile Menu is visible and 44px`, summaryPaint.visible && summaryPaint.height >= 44 && summaryPaint.width > 0 &&
    summaryPaint.outlineStyle !== 'none' && summaryPaint.outlineWidth >= 2 && !['transparent', 'rgba(0, 0, 0, 0)'].includes(summaryPaint.outlineColor), summaryPaint)
  await page.keyboard.press('Enter')
  check(`${tag}: native Enter opens mobile Menu`, await page.locator('details.md\\:hidden').getAttribute('open') !== null)
  const openOverflow = await page.evaluate(() => Math.max(0, document.documentElement.scrollWidth - innerWidth))
  check(`${tag}: open mobile Menu has no horizontal overflow`, openOverflow === 0, openOverflow)

  let settingsFocused = false
  const mobileVisited = []
  for (let i = 0; i < expectedNavigation.length; i++) {
    await page.keyboard.press('Tab')
    const active = await page.evaluate(() => ({
      tag: document.activeElement?.tagName || '',
      text: document.activeElement?.querySelector?.('span:last-child')?.textContent?.trim() || document.activeElement?.textContent?.trim() || '',
      href: document.activeElement?.getAttribute?.('href') || '',
    }))
    mobileVisited.push(active)
    if (active.tag === 'A' && active.href === '/settings' && active.text === 'Settings') settingsFocused = true
    if (page.url() !== startingURL) break
  }
  check(`${tag}: native Tab reaches Settings without activation`, settingsFocused && page.url() === startingURL, { visited: mobileVisited, url: page.url() })
  if (settingsFocused) {
    const settingsPaint = await page.locator('nav[aria-label="Primary (mobile)"] a[href="/settings"]').evaluate(el => {
      const rect = el.getBoundingClientRect()
      const style = getComputedStyle(el)
      return { active: document.activeElement === el, height: rect.height, width: rect.width,
        visible: Boolean(el.getClientRects().length) && rect.top >= 0 && rect.bottom <= innerHeight,
        outlineStyle: style.outlineStyle, outlineWidth: parseFloat(style.outlineWidth), outlineColor: style.outlineColor }
    })
    check(`${tag}: focused Settings link is visible and 44px`, settingsPaint.active && settingsPaint.visible && settingsPaint.height >= 44 && settingsPaint.width > 0 &&
      settingsPaint.outlineStyle !== 'none' && settingsPaint.outlineWidth >= 2 && !['transparent', 'rgba(0, 0, 0, 0)'].includes(settingsPaint.outlineColor), settingsPaint)
    for (let i = 0; i < expectedNavigation.length; i++) await page.keyboard.press('Shift+Tab')
    const returned = await summary.evaluate(el => document.activeElement === el)
    check(`${tag}: reverse native Tab returns to mobile Menu`, returned && page.url() === startingURL, page.url())
    if (returned) {
      await page.keyboard.press('Enter')
      check(`${tag}: native Enter closes mobile Menu`, await page.locator('details.md\\:hidden').getAttribute('open') === null)
    }
  }
}

let browser
let browserVersion = ''
let retained
let stale
try {
  browser = await chromium.launch({ channel: 'chrome', headless: true })
  browserVersion = browser.version()
  const browserProcess = await processIdentity(browser)
  fs.writeFileSync(path.join(out, 'browser-process.json'), JSON.stringify(browserProcess, null, 2) + '\n', { flag: 'wx', mode: 0o600 })
  const staleContext = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 390, height: 844 }, colorScheme: 'light' })
  await installBoundary(staleContext, 'stale-nojs')
  const stalePage = await staleContext.newPage()
  stalePage.on('console', m => { if (m.type() === 'error' || m.type() === 'warning') errors.push({ type: `console-${m.type()}`, tag: 'stale-nojs' }) })
  stalePage.on('pageerror', () => errors.push({ type: 'pageerror', tag: 'stale-nojs' }))
  stalePage.on('requestfailed', r => { if (r.failure()?.errorText !== 'net::ERR_BLOCKED_BY_CLIENT') errors.push({ type: 'requestfailed', tag: 'stale-nojs', path: new URL(r.url()).pathname }) })
  const staleInitial = await stalePage.goto(`${origin}/settings`, { waitUntil: 'domcontentloaded' })
  check('stale-nojs: pre-opened form is 200', staleInitial?.status() === 200, staleInitial?.status())
  stale = { context: staleContext, page: stalePage }

  const widths = [390, 768, 1440]
  for (const js of [false, true]) {
    for (const width of widths) {
      const tag = `${js ? 'js' : 'nojs'}-${width}`
      const context = await browser.newContext({ javaScriptEnabled: js, viewport: { width, height: 844 }, colorScheme: width === 768 ? 'dark' : 'light' })
      await installBoundary(context, tag)
      const page = await context.newPage()
      page.on('console', m => { if (m.type() === 'error' || m.type() === 'warning') errors.push({ type: `console-${m.type()}`, tag }) })
      page.on('pageerror', () => errors.push({ type: 'pageerror', tag }))
      page.on('requestfailed', r => { if (r.failure()?.errorText !== 'net::ERR_BLOCKED_BY_CLIENT') errors.push({ type: 'requestfailed', tag, path: new URL(r.url()).pathname }) })
      page.on('response', r => { const u = new URL(r.url()); if (u.origin === origin) responses.push({ tag, method: r.request().method(), path: u.pathname, status: r.status() }) })
      const initial = await page.goto(`${origin}/settings`, { waitUntil: 'domcontentloaded' })
      check(`${tag}: initial settings 200`, initial?.status() === 200, initial?.status())
      check(`${tag}: same-origin referrer policy`, initial?.headers()['referrer-policy'] === 'same-origin', initial?.headers()['referrer-policy'])
      check(`${tag}: private response no-store`, initial?.headers()['cache-control'] === 'no-store', initial?.headers()['cache-control'])
      check(`${tag}: native form method`, await page.locator('#google-credentials-form').getAttribute('method') === 'post')
      check(`${tag}: native form action`, await page.locator('#google-credentials-form').getAttribute('action') === '/settings/google/credentials')
      check(`${tag}: client label`, await page.locator('label[for="google-client-id"]').innerText() === 'Client ID (required)')
      check(`${tag}: secret label`, await page.locator('label[for="google-client-secret"]').innerText() === 'Client secret (required)')
      check(`${tag}: client help association`, await page.locator('#google-client-id').getAttribute('aria-describedby') === 'google-client-id-help')
      check(`${tag}: secret help association`, await page.locator('#google-client-secret').getAttribute('aria-describedby') === 'google-client-secret-help')
      check(`${tag}: initially valid`, await page.locator('[aria-invalid="true"]').count() === 0)
      await auditLayout(page, tag)
      await auditShell(page, `${tag}-initial`)
      await page.locator('#google-client-id').fill(probeClientID)
      await page.locator('#google-client-secret').fill('   ')
      const postCountBefore = requests.filter(r => r.tag === tag && r.method === 'POST').length
      await submit(page, tag)
      check(`${tag}: invalid redirect uses only fixed error fragment`, page.url() === `${origin}/settings#credentials-error`)
      const focus = await errorFocus(page)
      check(`${tag}: error alert eventually focused`, focus.eventual, focus)
      check(`${tag}: retained client ID`, (await page.locator('#google-client-id').inputValue()).endsWith('.apps.googleusercontent.com'))
      check(`${tag}: secret blank after rejection`, await page.locator('#google-client-secret').inputValue() === '')
      check(`${tag}: only secret invalid`, await page.locator('#google-client-id').getAttribute('aria-invalid') === null && await page.locator('#google-client-secret').getAttribute('aria-invalid') === 'true')
      check(`${tag}: invalid secret help and error associated`, await page.locator('#google-client-secret').getAttribute('aria-describedby') === 'google-client-secret-help google-client-secret-error')
      check(`${tag}: inline secret error present`, await page.locator('#google-client-secret-error').count() === 1)
      await snap(page, `${tag}-invalid-focus.png`)
      await page.keyboard.press('Tab')
      const errorLink = await page.evaluate(() => ({ id: document.activeElement?.id || '', tag: document.activeElement?.tagName || '', href: document.activeElement?.getAttribute?.('href') || '' }))
      check(`${tag}: Tab reaches first error link`, errorLink.tag === 'A' && errorLink.href === '#google-client-secret', errorLink)
      await page.keyboard.press('Enter')
      check(`${tag}: error link natively focuses secret`, await page.locator('#google-client-secret').evaluate(el => document.activeElement === el))
      await page.reload({ waitUntil: 'domcontentloaded' })
      check(`${tag}: reload did not resubmit`, requests.filter(r => r.tag === tag && r.method === 'POST').length === postCountBefore + 1)
      check(`${tag}: reload retains only activated field fragment`, page.url() === `${origin}/settings#google-client-secret`)
      await auditShell(page, `${tag}-invalid`)
      if (!js && width === 390) retained = { context, page, tag }
      else await context.close()
    }
  }

  const rp = retained.page
  await rp.locator('#google-client-id').fill(probeClientID)
  await rp.locator('#google-client-secret').fill(probeSecret)
  await submit(rp, 'nojs-390-recovery')
  const saved = new URL(rp.url())
  check('nojs recovery: fixed saved marker only', saved.pathname === '/settings' && saved.search === '?credentials=saved')
  check('nojs recovery: no submitted field in URL', !saved.searchParams.has('clientId') && !saved.searchParams.has('clientSecret'))
  const savedBody = await rp.locator('body').innerText()
  check('nojs recovery: saved and not-connected truth', savedBody.includes('Credentials saved locally.') && savedBody.includes('Gmail is not connected yet.'))
  check('nojs recovery: secret not reflected', !savedBody.includes(probeSecret))
  await snap(rp, 'nojs-390-saved.png')
  await auditShell(rp, 'nojs-390-recovery-shell')

  const sp = stale.page
  await sp.locator('#google-client-id').fill(staleClientID)
  await sp.locator('#google-client-secret').fill(staleSecret)
  await submit(sp, 'stale-nojs')
  const already = new URL(sp.url())
  check('stale form: fixed already-saved marker only', already.pathname === '/settings' && already.search === '?credentials=already-saved')
  check('stale form: no submitted field in URL', !already.searchParams.has('clientId') && !already.searchParams.has('clientSecret'))
  const alreadyBody = await sp.locator('body').innerText()
  check('stale form: existing credentials kept status', alreadyBody.includes('Credentials already existed.') && alreadyBody.includes('existing credentials were kept.') && alreadyBody.includes('Gmail is not connected yet.'))
  check('stale form: secret not reflected', !alreadyBody.includes(staleSecret))
  await snap(sp, 'stale-nojs-already-saved.png')
  await auditShell(sp, 'stale-nojs-already-saved-shell')

  // Independent synthetic controls prove that readiness cannot bless absent CSS
  // or waive a real, styled horizontal overflow. No credential POSTs in these lanes.
  const missingCSS = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 390, height: 844 } })
  try {
    await installBoundary(missingCSS, 'missing-css-control', { blockStylesheet: true })
    const page = await missingCSS.newPage()
    const response = await page.goto(`${origin}/settings`, { waitUntil: 'domcontentloaded' })
    check('missing CSS control: real Settings document is 200', response?.status() === 200)
    const readiness = await shellReadiness(page)
    check('missing CSS control: requested stylesheet was blocked', requests.some(r => r.tag === 'missing-css-control' && r.path === '/assets/app.css'))
    check('missing CSS control: readiness refuses missing CSS', !readiness.ready && readiness.sheets.length === 1 && readiness.sheets[0].rules === null, readiness)
  } finally { await missingCSS.close() }
  const overflowing = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 390, height: 844 } })
  try {
    await installBoundary(overflowing, 'styled-overflow-control')
    const page = await overflowing.newPage()
    await page.goto(`${origin}/settings`, { waitUntil: 'domcontentloaded' })
    const readiness = await shellReadiness(page)
    check('styled overflow control: real shell CSS is ready', readiness.ready, readiness)
    await page.evaluate(() => {
      const fixture = document.createElement('div')
      fixture.id = 'synthetic-overflow-control'
      fixture.style.width = `${innerWidth + 128}px`
      fixture.style.height = '1px'
      document.body.append(fixture)
    })
    const sample = await page.evaluate(readShell, { expected: expectedNavigation, diagnose: true })
    check('styled overflow control: unchanged zero-overflow oracle refuses fixture', sample.horizontalOverflow >= 128, sample.horizontalOverflow)
  } finally { await overflowing.close() }

  check('boundary: no unexpected network', failures.every(f => !f.includes('unexpected request')))
  check('browser: no console, page, or request errors', errors.length === 0, errors.length)
} finally {
  await retained?.context.close().catch(() => {})
  await stale?.context.close().catch(() => {})
  await browser?.close()
}

const selected = [
  'cmd/serve/settings_browser_probe_test.go', 'cmd/serve/settingsform.go', 'cmd/serve/settingspage.go',
  'cmd/serve/settings.go', 'cmd/serve/auth.go', 'cmd/serve/server.go', 'cmd/serve/shell.go', 'cmd/serve/web/app.css',
  'vendor/github.com/geoffmilleraz/app-kit/appkit/ui/dashboard/nav.go',
  'vendor/github.com/geoffmilleraz/app-kit/appkit/ui/dashboard/shell.go',
].map(rel => ({ path: rel, size: fs.statSync(path.join(root, rel)).size, sha256: sha(path.join(root, rel)) }))
const pwRoot = path.dirname(fs.realpathSync(playwrightEntry))
const receipt = {
  schema: 1,
  generatedAt: new Date().toISOString(),
  boundary: { originHost: new URL(origin).hostname, allowed: [...allowed].sort(), requestBodiesRecorded: false, submittedValuesRecorded: false },
  playwright: { version: JSON.parse(fs.readFileSync(path.join(pwRoot, 'package.json'))).version, entrySha256: sha(path.join(pwRoot, 'index.mjs')), packageSha256: sha(path.join(pwRoot, 'package.json')) },
  browserVersion,
  totals: { checks: checks.length, passed: checks.filter(c => c.ok).length, failed: failures.length, requests: requests.length, responses: responses.length, errors: errors.length },
  checks, failures, layoutDiagnostics, requestSummary: requests.reduce((m, r) => { const k = `${r.method} ${r.path}`; m[k] = (m[k] || 0) + 1; return m }, {}),
  responseStatusSummary: responses.reduce((m, r) => { const k = `${r.method} ${r.path} ${r.status}`; m[k] = (m[k] || 0) + 1; return m }, {}),
  screenshots: screenshots.sort((a,b) => a.name.localeCompare(b.name)), selectedSources: selected.sort((a,b) => a.path.localeCompare(b.path)),
}
const receiptPath = path.join(out, 'receipt.json')
fs.writeFileSync(receiptPath, JSON.stringify(receipt, null, 2) + '\n', { mode: 0o600 })
console.log(JSON.stringify({ outputDir: out, receipt: receiptPath, receiptSha256: sha(receiptPath), totals: receipt.totals, screenshots: receipt.screenshots }, null, 2))
if (failures.length) process.exitCode = 1
