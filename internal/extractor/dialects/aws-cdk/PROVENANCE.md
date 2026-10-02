# aws-cdk — provenance and divergences

LB-Q-aws-cdk, design `docs/design/language-breadth.md` §4, §13 (wave iac). A doc-engine dialect
on `cdk.json` (a fusion fast-path key). The app's program rides its own language's dialect
(TypeScript, Python, Go, Java, C#).

## What it models

- The **app** command (`CDKApp`, `task`, signed by the command), each **context** key
  (`CDKContext`, `key`) and **feature flag** (`CDKFeatureFlag`, `key`: a context key named
  `@aws-cdk...`), each **watch** include and exclude glob (`CDKWatch`, `selector`) and every other
  top-level **setting** (`CDKSetting`, `key`).
- **The program the app runs** (L2, a reference of `app`; L3 declared as a `repo-path`): the
  first argument naming a `.ts`, `.js`, `.mjs`, `.cjs`, `.py`, `.go`, `.csproj` or `.fsproj` file
  (`npx tsx bin/app.ts`, `python3 app.py`, `go run app.go`, `dotnet run --project X.csproj`).

## What is not modelled, and why

- An app command that names no file (`mvn -e -q compile exec:java`), and the program's own
  stacks and constructs (its language's dialect; LB-Q-aws-cdk-rest). The entry file is not
  stored as an edge: the doc-xref pass joins only doc-engine files.

## Calibration oracle

`calibration/oracles/aws-cdk/cdk_oracle.js` (bench-only): the AWS CDK CLI (aws-cdk 2.1144.0,
Apache-2.0), `cdk context --json -vv`, which loads cdk.json as every command does and logs what
it read (tier A, definitions); the app command's file, read from the command and checked
against the subject's tree (tier C, references; only running the app would name it).
Subjects: `calibration/corpus.d/aws-cdk.yaml`.
