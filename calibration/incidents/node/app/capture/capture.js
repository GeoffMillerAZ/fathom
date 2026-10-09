'use strict';

// TL-7's capture runner (docs/plans/telemetry-links.md): runs every
// fault-injected case, reads the truth from V8's own structured call sites
// (the innermost frame inside the app), and writes one JSON line per case
// carrying the raw stack and the payload shapes a notifier, a tracer, the
// Lambda runtime and an APM agent would send. The app root is rewritten to
// /app (a container's deploy root; /var/task for Lambda), as production
// traces carry the deploy root, not the repository's.

const path = require('path');
const { Invoice } = require('../src/billing/invoice');
const { Shipment } = require('../src/shipping/shipment');
const { Product } = require('../src/catalog/product');
const { Search } = require('../src/catalog/search');
const { Order } = require('../src/orders/order');
const { Cart } = require('../src/orders/cart');
const { User } = require('../src/accounts/user');
const { Auth } = require('../src/accounts/auth');

const appRoot = path.resolve(__dirname, '..');

// V8 formats err.stack through prepareStackTrace: keep the call sites and
// render the default text (the header, then "    at " + each call site).
Error.prepareStackTrace = (err, sites) => {
  err.sites = sites;
  const head = err.message ? `${err.name}: ${err.message}` : err.name;
  return [head, ...sites.map((s) => `    at ${s}`)].join('\n');
};

const cases = [
  ['billing-total-div0', () => new Invoice([100, 200], 'USD').total(0)],
  ['billing-tax-undefined', () => new Invoice([100], 'USD').applyTax('US')],
  ['billing-parse-amount', () => new Invoice([1], 'USD').parseAndAdd('12a')],
  ['billing-ctor-currency', () => new Invoice([], 'EURO')],
  ['billing-refund-static', () => Invoice.refund(new Invoice([10], 'USD'), 9999)],
  ['shipping-round-div0', () => new Shipment(1200, 'DE').packed(0)],
  ['shipping-band-bounds', () => new Shipment(5000, 'DE').band()],
  ['shipping-validate', () => new Shipment(10, '').validate()],
  ['shipping-track-async', () => new Shipment(10, 'FR').track({})],
  ['catalog-validate', () => new Product('bad', '1').validate()],
  ['catalog-walk-cycle', () => new Search({ a: 'b', b: 'a' }).depth('a')],
  ['catalog-rank-missing', () => new Search({}).rank(['A'], 'Z')],
  ['catalog-price-parse', () => new Product('ABC-1', 'x').price()],
  ['orders-add-line', () => new Order(5).addLine(0)],
  ['orders-coupon-undefined', () => new Cart(new Order(5), {}).applyCoupon('NOPE')],
  ['orders-checkout', () => new Cart(new Order(5), {}).checkout(99)],
  ['orders-summarize-callback', () => {
    const o = new Order(5);
    o.addLine(3);
    o.summarize(0);
  }],
  ['orders-deep-chain', () => new Order(1).deep(5)],
  ['accounts-rename', () => new User('a', 'a@b.c').rename('Single')],
  ['accounts-verify', () => new Auth('s3cret').verify('token')],
  ['accounts-hash-algorithm', () => new Auth('s').hashPassword('p', 'NOPE-256')],
  ['accounts-domain', () => new User('a', 'nope').domain()],
];

const inApp = (file) => file && file.startsWith(appRoot + path.sep) && !file.startsWith(path.join(appRoot, 'capture') + path.sep);
const rel = (file) => path.relative(appRoot, file).split(path.sep).join('/');
const deployed = (text, root) => text.split(appRoot + path.sep).join(root + '/');

// The bare name the index gives the injected function: a class
// constructor is declared `constructor`, though V8 names the class.
function bareName(site) {
  if (site.isConstructor()) {
    return 'constructor';
  }
  const fn = site.getFunctionName() || site.getMethodName() || '';
  return fn.split('.').pop();
}

function bugsnag(err) {
  const stacktrace = err.sites.map((s) => ({
    file: s.getFileName() && s.getFileName().startsWith(appRoot) ? rel(s.getFileName()) : s.getFileName(),
    lineNumber: s.getLineNumber(),
    columnNumber: s.getColumnNumber(),
    method: s.getFunctionName() ? (s.getTypeName() && !s.isToplevel() && !s.getFunctionName().includes('.') ? `${s.getTypeName()}.${s.getFunctionName()}` : s.getFunctionName()) : null,
    inProject: Boolean(s.getFileName() && s.getFileName().startsWith(appRoot) && !s.getFileName().includes('node_modules')),
  }));
  return {
    apiKey: '00000000000000000000000000000000',
    events: [{
      exceptions: [{ errorClass: err.name, message: err.message, stacktrace }],
      breadcrumbs: [{ name: 'checkout clicked', type: 'user' }],
      user: { id: 'u-17' },
      metaData: { request: { path: '/checkout' } },
    }],
  };
}

const attr = (key, v) => ({ key, value: typeof v === 'number' ? { intValue: String(v) } : { stringValue: String(v) } });

// An OTLP JSON log record from the logging bridge: the exception
// attributes, and code.* (new semantic conventions) naming the call site
// that logged it.
function otlp(err, stack, logSite) {
  return {
    resourceLogs: [{
      resource: { attributes: [attr('service.name', 'shop-node')] },
      scopeLogs: [{
        logRecords: [{
          severityText: 'ERROR',
          body: { stringValue: err.message },
          attributes: [
            attr('exception.type', err.name),
            attr('exception.message', err.message),
            attr('exception.stacktrace', stack),
            attr('code.file.path', deployed(logSite.file, '/app')),
            attr('code.line.number', logSite.line),
            attr('code.function.name', logSite.func),
          ],
        }],
      }],
    }],
  };
}

function lambda(err) {
  const trace = deployed(err.stack, '/var/task').split('\n');
  return { errorType: err.name, errorMessage: err.message, trace };
}

function newrelic(err, stack) {
  return { results: [{ events: [{ 'error.class': err.name, 'error.message': err.message, 'error.stack_trace': stack, appName: 'shop-node' }] }] };
}

function logSiteHere() {
  const holder = {};
  Error.captureStackTrace(holder, logSiteHere);
  void holder.stack;
  const s = holder.sites[0];
  return { file: s.getFileName(), line: s.getLineNumber(), func: s.getFunctionName() || 'record' };
}

function record(id, err) {
  void err.stack; // formats once, keeping the call sites
  const site = err.sites.find((s) => inApp(s.getFileName()));
  const stack = deployed(err.stack, '/app');
  const out = {
    case: id,
    lang: 'node',
    truth: { file: rel(site.getFileName()), line: site.getLineNumber(), func: site.getFunctionName(), name: bareName(site) },
    payloads: {
      raw: stack,
      bugsnag: JSON.stringify(bugsnag(err)),
      otlp: JSON.stringify(otlp(err, stack, logSiteHere())),
      lambda: JSON.stringify(lambda(err)),
      newrelic: JSON.stringify(newrelic(err, stack)),
    },
  };
  process.stdout.write(JSON.stringify(out) + '\n');
}

async function main() {
  for (const [id, fn] of cases) {
    try {
      await fn();
    } catch (err) {
      record(id, err);
      continue;
    }
    process.stderr.write(`case ${id} did not throw\n`);
    process.exit(1);
  }
}

main();
