const assert = require('node:assert/strict');
const { argumentsFor } = require('../../bench.d/apps-r2-vitest-json.cjs');
const env = { BENCH_REPORT_DIR: '/work/reports', BENCH_REPORT_STAGE: '1' };
const argv = ['node', '/work/source/node_modules/.bin/vitest', 'run'];
const args = argumentsFor(argv, env, '/work/source/package');
assert.equal(args[0], '--reporter=json');
assert.match(args[1], /^--outputFile=\/work\/reports\/1\/[0-9a-f]{20}\.json$/);
assert.deepEqual(argumentsFor(argv, { ...env, VITEST: 'true' }, '/work/source'), []);
assert.deepEqual(argumentsFor(['node', '/work/source/tsc', '--noEmit'], env, '/work/source'), []);
assert.deepEqual(argumentsFor(['node', '/work/source/app.mjs'], env, '/work/source'), []);
assert.deepEqual(argumentsFor(argv, {}, '/work/source'), []);
assert.deepEqual(argumentsFor([...argv, '--version'], env, '/work/source'), []);
assert.notEqual(argumentsFor(argv, { ...env, NITRO_BUILDER: 'rollup' }, '/work/source')[1],
                argumentsFor(argv, { ...env, NITRO_BUILDER: 'rolldown' }, '/work/source')[1]);
assert.deepEqual(argumentsFor(argv, env, '/work/tmp/nitro-example-vitest-fixture'), []);
assert.deepEqual(argumentsFor([...argv, '--outputFile=/work/results.json'], env, '/work/source'), []);
assert.deepEqual(argumentsFor([...argv, '--outputFile', '/work/results.json'], env, '/work/source'), []);
assert.deepEqual(argv, ['node', '/work/source/node_modules/.bin/vitest', 'run']);
console.log('PASS: native Vitest reporting; type checks, application CLIs and nested Vitest runs unaffected; builder reports distinct');
