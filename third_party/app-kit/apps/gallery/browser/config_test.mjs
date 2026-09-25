import test from 'node:test';
import assert from 'node:assert/strict';
import {configuration} from './config.mjs';

const valid = {APPKIT_SHELL_BROWSER_ORIGIN:'http://127.0.0.1:18780', APPKIT_PLAYWRIGHT_MODULE:'/tmp/approved-playwright/index.mjs'};
test('explicit local origin and module are required before import or browser launch', () => {
  assert.throws(()=>configuration({}), /explicitly approved/);
  assert.throws(()=>configuration({APPKIT_SHELL_BROWSER_ORIGIN:valid.APPKIT_SHELL_BROWSER_ORIGIN}), /existing approved isolated/);
  assert.throws(()=>configuration({...valid,APPKIT_PLAYWRIGHT_MODULE:'node_modules/playwright/index.mjs'}), /absolute/);
  assert.throws(()=>configuration({...valid,APPKIT_BROWSER_EXECUTABLE:'chrome'}), /absolute/);
});
test('origin refuses remote hosts, DNS names, other schemes and extra URL data', () => {
  for(const origin of ['http://example.com:18780','http://localhost:18780','https://127.0.0.1:18780','http://127.0.0.1','http://127.0.0.1:80','http://127.0.0.1:18780/','http://127.0.0.1:18780/shell','http://127.0.0.1:18780?x=y','http://127.0.0.1:18780#shell','http://user:pass@127.0.0.1:18780','http://127.1:18780']) {
    assert.throws(()=>configuration({...valid,APPKIT_SHELL_BROWSER_ORIGIN:origin}), undefined, origin);
  }
});
test('canonical IPv4 and IPv6 loopback origins and safe labels pass', () => {
  assert.equal(configuration(valid).base, valid.APPKIT_SHELL_BROWSER_ORIGIN);
  assert.equal(configuration({...valid,APPKIT_SHELL_BROWSER_ORIGIN:'http://[::1]:18780'}).base,'http://[::1]:18780');
  assert.equal(configuration({...valid,APPKIT_BROWSER_VARIANT:'before-header-fix'}).variant,'before-header-fix');
  for(const variant of ['../overwrite','A','', 'x'.repeat(33)]) {
    if(variant==='') assert.equal(configuration({...valid,APPKIT_BROWSER_VARIANT:variant}).variant,'candidate');
    else assert.throws(()=>configuration({...valid,APPKIT_BROWSER_VARIANT:variant}), /1–32/);
  }
});
