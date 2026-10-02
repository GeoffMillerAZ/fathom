// node --test calibration/oracles/parliament/iam_cdk_oracle.test.js, with
// IAM_ORACLE_MODULES=<node_modules holding aws-cdk-lib and jsonc-parser>.
'use strict';

const test = require('node:test');
const assert = require('node:assert');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { analyze } = require('./iam_cdk_oracle.js');

const mods = process.env.IAM_ORACLE_MODULES;

function subject(files) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'iam-oracle-'));
  for (const [rel, body] of Object.entries(files)) {
    fs.mkdirSync(path.dirname(path.join(dir, rel)), { recursive: true });
    fs.writeFileSync(path.join(dir, rel), body);
  }
  return dir;
}

test('AWS policy model decides documents, statements and elements', { skip: !mods }, () => {
  const iam = require(path.join(mods, 'aws-cdk-lib', 'aws-iam'));
  const jsonc = require(path.join(mods, 'jsonc-parser'));
  const dir = subject({
    'p/identity.json': `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "Read",
      "Effect": "Allow",
      "Action": ["s3:GetObject", "s3:ListBucket"],
      "Resource": "arn:aws:s3:::example-bucket/*"
    },
    {
      "Effect": "Deny",
      "NotAction": "iam:*",
      "NotResource": ["*"]
    }
  ]
}
`,
    'p/trust.json': `{
  "Version": "2012-10-17",
  "Statement": {
    "Effect": "Allow",
    "Principal": {"Service": "ec2.amazonaws.com", "AWS": ["arn:aws:iam::111122223333:root"]},
    "Action": "sts:AssumeRole"
  }
}
`,
    'p/bad.json': `{
  "Version": "2012-10-17",
  "Statement": [
    {"Effect": "Allow", "Action": "not an action", "Resource": "*"},
    {"Sid": "Ok", "Effect": "Allow", "Action": "s3:*", "Resource": "*"}
  ]
}
`,
    'p/other.json': '{"Version": "2012-10-17", "Name": "x"}\n',
  });
  const r = analyze(dir, iam, jsonc);
  const defs = r.defs.map((d) => `${d.path}|${d.name}|${d.kind}@${d.line}`);
  assert.deepStrictEqual(defs, [
    'p/bad.json|Ok|rule@5',
    'p/identity.json|identity.json|policy@1',
    'p/identity.json|Read|rule@4',
    'p/identity.json|1|rule@10',
    'p/trust.json|trust.json|policy@1',
    'p/trust.json|0|rule@3',
  ]);
  const refs = r.refs.map((x) => `${x.path}|${x.from}->${x.to}`).sort();
  assert.deepStrictEqual(refs, [
    'p/bad.json|Ok->*',
    'p/bad.json|Ok->s3:*',
    'p/identity.json|1->*',
    'p/identity.json|1->iam:*',
    'p/identity.json|Read->arn:aws:s3:::example-bucket/*',
    'p/identity.json|Read->s3:GetObject',
    'p/identity.json|Read->s3:ListBucket',
    'p/trust.json|0->arn:aws:iam::111122223333:root',
    'p/trust.json|0->ec2.amazonaws.com',
    'p/trust.json|0->sts:AssumeRole',
  ]);
  assert.strictEqual(r.stats.statements_refused, 1);
  assert.strictEqual(r.stats.documents_refused, 1);
  assert.strictEqual(r.stats.single_object_statement, 1);
  assert.strictEqual(r.stats.not_policy, 1);
});
