'use strict';

// TL-4's log capture runner (docs/plans/telemetry-links.md): runs every
// logging case through the real libraries (pino, winston and the console;
// pino and winston installed by ../../install-log-deps.sh, found through
// NODE_PATH) and writes one JSON line per case: the rendered log line as an
// operator would paste it, and the truth, the call site V8's own structured
// call sites report at the moment the line is written (pino's destination,
// winston's formats and the console all run on the caller's stack). The
// rendered line never carries the call site. Every case logs a value
// carrying "SNTL", which no localize answer may echo.

const path = require('path');
const util = require('util');

const appRoot = path.resolve(__dirname, '..');
const logModule = path.join(appRoot, 'src', 'log.js');
const rel = (file) => path.relative(appRoot, file).split(path.sep).join('/');

Error.stackTraceLimit = 100;

// The innermost app frame outside the logger module: the logger call.
function site() {
  const prev = Error.prepareStackTrace;
  Error.prepareStackTrace = (_, sites) => sites;
  const holder = {};
  Error.captureStackTrace(holder);
  const sites = holder.stack;
  Error.prepareStackTrace = prev;
  for (const s of sites) {
    const file = s.getFileName();
    if (!file || !file.startsWith(path.join(appRoot, 'src') + path.sep) || file === logModule) {
      continue;
    }
    const fn = s.isConstructor() ? 'constructor' : (s.getFunctionName() || s.getMethodName() || '');
    return { file: rel(file), line: s.getLineNumber(), func: fn, name: fn.split('.').pop() };
  }
  throw new Error('no app frame on the logging stack');
}

const captured = [];
const log = require('../src/log');
log.sink.line = (text, api) => captured.push({ api, line: text, truth: site() });
for (const m of ['error', 'warn', 'info', 'log', 'debug']) {
  console[m] = (...args) => captured.push({ api: 'node.console', line: util.format(...args), truth: site() });
}

const { Gateway } = require('../src/payments/gateway');
const refunds = require('../src/payments/refunds');
const { Warehouse } = require('../src/fulfillment/warehouse');
const audit = require('../src/audit/events');

const gw = new Gateway('SNTL-merchant-01');
const wh = new Warehouse('SNTL-site-02');

const cases = [
  ['payments-authorize-merging-object', () => gw.authorize(1250, '4111111111111111')],
  ['payments-refuse', () => gw.authorize(-5, 'SNTL0003')],
  ['payments-decline', () => gw.decline('SNTL-auth-04', 'SNTL04')],
  ['payments-timeout-template', () => gw.timeout('SNTL-auth-05', 30)],
  ['payments-void-error-first', () => gw.voidAuthorization('SNTL-auth-06')],
  ['payments-capture-concat', () => gw.capture('SNTL-auth-07', 990)],
  ['payments-chargeback-util-format', () => gw.chargeback('SNTL-pay-08', 'fraud')],
  ['refunds-request', () => refunds.requestRefund('SNTL-pay-09', 400, 'damaged')],
  ['refunds-reject', () => refunds.rejectRefund('SNTL-pay-10', 50000)],
  ['refunds-settle-debug', () => refunds.settleRefund('SNTL-pay-11', 'B-7')],
  ['refunds-flag-dynamic', () => refunds.flagRefund('SNTL-pay-12')],
  ['refunds-credit-merging-object', () => refunds.issueCredit('SNTL-refund-13', 400)],
  ['warehouse-pick-splat', () => wh.pick('SNTL-sku-14', 'A-4')],
  ['warehouse-short-pick', () => wh.shortPick('SNTL-sku-15', 6, 2)],
  ['warehouse-restock-template', () => wh.restock('SNTL-sku-16', 40)],
  ['warehouse-quarantine-log-level', () => wh.quarantine('SNTL-lot-17')],
  ['warehouse-recount-debug', () => wh.recount('SNTL-sku-18')],
  ['warehouse-cycle-count', () => wh.cycleCountFailed('SNTL-sku-19', 3)],
  ['audit-login-failed-console-error', () => audit.loginFailed('SNTL-user-20', 5)],
  ['audit-role-granted-console-info', () => audit.roleGranted('SNTL-user-21', 'admin', 'u-1')],
  ['audit-export-denied-console-error-concat', () => audit.exportDenied('SNTL-user-22', 'orders')],
  ['audit-password-reset-console-warn', () => audit.passwordReset('SNTL-user-23')],
  ['audit-access-denied-console-error-template', () => audit.accessDenied('SNTL-user-24', 'reports')],
  ['audit-session-expired-console-log', () => audit.sessionExpired('SNTL-session-25')],
];

let out = '';
for (const [id, fn] of cases) {
  captured.length = 0;
  fn();
  if (captured.length === 0) {
    process.stderr.write(`case ${id} logged nothing\n`);
    process.exit(1);
  }
  const last = captured[captured.length - 1];
  out += `${JSON.stringify({ case: id, lang: 'node', api: last.api, truth: last.truth, payloads: { log: last.line } })}\n`;
}
process.stdout.write(out);
