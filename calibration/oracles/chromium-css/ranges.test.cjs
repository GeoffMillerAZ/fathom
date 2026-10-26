'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const { rangeCompanion } = require('./ranges.cjs');

test('invalid native range budgets fail before scratch or browser profile creation', async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'css-range-budget-'));
  const prior = process.env.CSS_BROWSER_TIMEOUT_MS;
  try {
    for (const value of ['0', '-1', '1200001']) {
      const scratch = path.join(root, `scratch-${value}`);
      process.env.CSS_BROWSER_TIMEOUT_MS = value;
      await assert.rejects(rangeCompanion('/unused/browser', {}, '/unused/subject', {}, scratch),
        /CSS_BROWSER_TIMEOUT_MS must be a positive integer no greater than 1200000/);
      assert.equal(fs.existsSync(scratch), false, `budget ${value} created a ranges profile or scratch directory`);
    }
  } finally {
    if (prior === undefined) delete process.env.CSS_BROWSER_TIMEOUT_MS;
    else process.env.CSS_BROWSER_TIMEOUT_MS = prior;
    fs.rmSync(root, { recursive: true, force: true });
  }
});
