// Native-key control only. No claim about application recovery or physical input.
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import { inspectZoom } from './zoom-contract.mjs';

const html = '<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Native select control</title></head><body><label>Scheme<select name="scheme"><option value="light">Light</option><option value="dark">Dark</option></select></label><button id="after">After selection</button></body></html>';
const bounded = async work => {
  let timer;
  try { return await Promise.race([work(), new Promise((_, reject) => {
    timer = setTimeout(() => reject(Error('native-select operation deadline')), 10000);
  })]); } finally { clearTimeout(timer); }
};

export async function runProbe({ context, origin, report }) {
  assert.equal(report.contract.workload.mode, 'native-select-control-v1');
  assert.equal(new URL(origin).hostname, '127.0.0.1');
  const page = await context.newPage(), cdp = await context.newCDPSession(page);
  const traces = [], blocked = [], errors = [];
  report.select_control = { html, html_sha256: crypto.createHash('sha256').update(html).digest('hex'),
    traces, blocked, errors, limits: ['Two-option browser control, not app navigation or native physical-key proof.'] };
  const check = (name, pass, detail) => {
    report.checks.push({ name, pass: Boolean(pass), detail }); assert.ok(pass, name);
  };
  async function state(key) {
    const dom = await bounded(() => page.evaluate(() => {
      const select = document.querySelector('select');
      return { value: select.value, selectedIndex: select.selectedIndex,
        options: [...select.options].map(o => ({ value: o.value, selected: o.selected })),
        active: { tag: document.activeElement.tagName, name: document.activeElement.getAttribute('name'), id: document.activeElement.id },
        scripts: document.scripts.length, dpr: devicePixelRatio, inner: innerWidth,
        outer: outerWidth, scale: visualViewport.scale, scrollWidth: document.documentElement.scrollWidth };
    }));
    const tree = await bounded(() => cdp.send('Accessibility.getFullAXTree'));
    const ax = tree.nodes.filter(n => ['combobox', 'option', 'MenuListPopup', 'menuListPopup', 'button'].includes(n.role?.value))
      .map(n => ({ nodeId: n.nodeId, role: n.role?.value, name: n.name?.value, value: n.value?.value,
        ignored: n.ignored, properties: n.properties }));
    return { key, dom, ax, popup_observability: 'Page AX may omit native popup state; selected value and post-Tab focus are the acceptance oracle.' };
  }
  try {
    await page.setViewportSize({ width: 1440, height: 1000 });
    page.setDefaultTimeout(10000); page.setDefaultNavigationTimeout(15000);
    await context.route('**/*', route => {
      if (route.request().method() === 'GET' && route.request().url() === origin + '/native-select-control')
        return route.fulfill({ status: 200, contentType: 'text/html; charset=utf-8', body: html,
          headers: { 'Content-Security-Policy': "default-src 'none'; script-src 'none'" } });
      blocked.push(route.request().url()); return route.abort('blockedbyclient');
    });
    await context.routeWebSocket('**/*', socket => { blocked.push('WebSocket'); socket.close(); });
    page.on('pageerror', e => errors.push(String(e)));
    page.on('requestfailed', r => errors.push(r.url() + ': ' + r.failure()?.errorText));
    await page.goto('chrome://settings/appearance', { waitUntil: 'load' });
    await page.locator('#zoomLevel').selectOption('2');
    check('Chrome Appearance 200 selected', await page.locator('#zoomLevel').inputValue() === '2');
    await page.goto('about:blank', { waitUntil: 'load' });
    await bounded(() => cdp.send('Emulation.setScriptExecutionDisabled', { value: true }));
    for (const [name, keys] of [
      ['historical', ['Home', 'ArrowDown', 'Tab']],
      ['explicit-confirmation', ['Space', 'ArrowDown', 'Enter', 'Tab']],
    ]) {
      await page.goto(origin + '/native-select-control', { waitUntil: 'load' });
      const trace = { name, keys, states: [await state('initial')] }; traces.push(trace);
      const initial = trace.states[0].dom;
      check(`${name} exact initial control with no scripts`, initial.value === 'light' && initial.selectedIndex === 0 &&
        initial.scripts === 0 && initial.active.tag === 'BODY' && JSON.stringify(initial.options) ===
        JSON.stringify([{ value: 'light', selected: true }, { value: 'dark', selected: false }]), initial);
      const metrics = await bounded(() => cdp.send('Page.getLayoutMetrics'));
      check(`${name} actual 200%`, inspectZoom('2', trace.states[0].dom, metrics));
      await page.keyboard.press('Tab'); trace.states.push(await state('Tab to select'));
      check(`${name} native focus reaches select`, trace.states.at(-1).dom.active.name === 'scheme');
      for (const key of keys) { await page.keyboard.press(key); trace.states.push(await state(key)); }
      trace.selected_dark = trace.states.at(-1).dom.value === 'dark';
      trace.focus_left_select = trace.states.at(-1).dom.active.id === 'after';
      if (name === 'explicit-confirmation') {
        check('confirmation commits Dark before leaving select', trace.states.at(-2).dom.value === 'dark', trace.states.at(-2));
        check('Dark survives Tab and focus reaches next control', trace.selected_dark && trace.focus_left_select, trace.states.at(-1));
      }
    }
    check('no blocked requests or page errors', blocked.length === 0 && errors.length === 0, { blocked, errors });
    report.cells.push({ name: 'native-select-control', traces: traces.length });
  } finally {
    await bounded(() => cdp.detach()).catch(e => report.errors.push(String(e)));
    await bounded(() => page.close()).catch(e => report.errors.push(String(e)));
  }
}
