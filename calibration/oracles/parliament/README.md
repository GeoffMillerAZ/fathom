# aws-iam-policy oracles (tier A: aws-cdk-lib; tier B: parliament)

The language-breadth harness's oracles for the `aws-iam-policy` doc-engine dialect (LB-Q-aws-iam-policy,
LB-Q-aws-iam-policy-rest; design `docs/design/language-breadth.md` §1.3, §1.5 D2). Bench-only, never
imported by fathom, never a product dependency. Both read `.json` only and name the same entities: a
`policy` per document (a JSON object with a `Statement` and a `Version` of 2012-10-17 or 2008-10-17, named
by the file, at its root object's line), a `rule` per statement (its `Sid`, else its position from 0, at the
statement object's line), and as references every value of a statement's `Action`, `NotAction`,
`Resource`, `NotResource`, `Principal` and `NotPrincipal` (a principal object's values, one list per
principal type), as written.

## iam_cdk_oracle.js (tier A, scored since wave ox)

```
npm install --prefix <scratch> aws-cdk-lib@2.272.0 constructs@10 jsonc-parser@3.3.1
node calibration/oracles/parliament/iam_cdk_oracle.js -subject <checkout> -modules <scratch>/node_modules \
    -out <scratch>/iam-<subject>.json -files-out <txt> -summary-out <json>
indexbench lang-oracle -dialect aws-iam-policy -subject <checkout> -subject-name <name> \
    -tool-oracle <scratch>/iam-<subject>.json -out docs/bench/readings/lang/aws-iam-policy.json
IAM_ORACLE_MODULES=<scratch>/node_modules node --test calibration/oracles/parliament/iam_cdk_oracle.test.js
```

AWS's own policy model decides: aws-cdk-lib's `aws_iam` (Apache-2.0, maintained by AWS), run offline. A
document is an entity when `PolicyDocument.fromJson` accepts it; a statement when `PolicyStatement.fromJson`
accepts it (field types, the action grammar, at least one Action or NotAction); its name is the Sid the
parsed statement carries; its references are the parsed statement's actions, notActions, resources,
notResources and principals. IAM also accepts a `Statement` written as one object, which
`PolicyDocument.fromJson` refuses ("Statement must be an array"), so such a document is accepted when its one
statement is (27 of camp's documents). Values come from `JSON.parse`, the parser aws-cdk-lib is handed;
lines from jsonc-parser (Microsoft, MIT), an independent reader with offsets.

**Why tier A.** §1.3's A rung is the format's reference parser or the tool's own evaluation. IAM's own
parser runs inside the service (IAM Access Analyzer's `validate-policy` is online), so no offline copy of it
exists; aws-cdk-lib is AWS's first-party, offline implementation of the policy document model, the code AWS
ships for writing and reading IAM policies, not a third party's reading of the format (which would be B, as
parliament is below). The relabel from the tier-C reading cites that definition and its limit: aws-cdk-lib's
action grammar (`^(\*|[a-zA-Z0-9-]+:[a-zA-Z0-9*]+)$`) is stricter than the service's. On tenchi-security/camp
it refuses 8 AWS managed policy versions IAM itself published (an action with a space, `amplifybackend: *`,
`lambda:ListFunctions `; an action with a hyphen, `config:select-resource-config`): those 8 documents and
statements yield no entity and stay in the measured set, so fathom's 16 definitions there count against it.
Every other definition and reference agrees with parliament's (38,553 of 38,569 definitions, 543,829 of
546,159 references; the difference is exactly the refused statements).

## parliament_oracle.py (tier B since wave ox; the cross-check)

```
python3 -m venv <scratch>/venv && <scratch>/venv/bin/pip install parliament==1.6.4 'setuptools<81'
<scratch>/venv/bin/python calibration/oracles/parliament/parliament_oracle.py -subject <checkout> -out <json> \
    [-files-out <txt>] [-summary-out <json>] [-no-analyze]
cd calibration/oracles/parliament && <scratch>/venv/bin/python -m unittest test_parliament_oracle
```

duo-labs/parliament 1.6.4 (BSD-3-Clause): its `jsoncfg` reader and its own `make_list` and `Statement`
classes. **Relabelled C to B (wave ox, the decision LB-Q-aws-iam-policy-rest asked for):** parliament is the
widely used third-party IAM policy parser and linter, and its `Statement` model lists each statement's
elements; design §1.5 D2 places a widely used third-party parser's dump at tier B (JavaParser,
nikic/php-parser and luaparse are the precedents), which proves L1 excellent and L2 at most good. It is not
the format's reference parser, which is why the scored oracle is the tier-A one above.
