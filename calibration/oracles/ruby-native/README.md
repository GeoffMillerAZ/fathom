# Released Ruby owner-tool probes

Bench-only supplementary evidence for the complete original Sinatra 4.2.1 and
Jekyll 4.4.1 Git pins in `bootstrap.py`. No canonical reading, corpus shard,
product dispatch or tier policy changes. Requires Darwin ARM64, Python 3.13+,
Git, curl, a C compiler, make and Perl. Use one heavy job at a time, niceness 10,
load <=6, and `df -h ~` before every heavy step; refuse below 20 GiB free.
All artifacts/caches belong under workspace/ or the worker's cache. Set TMPDIR,
GOCACHE, GOMODCACHE, GOTMPDIR and XDG paths there, GOWORK=off, GOMAXPROCS=2,
GOFLAGS=-p=2 and PYTHONDONTWRITEBYTECODE=1. No global installation is needed.
Every --out below must be fresh. Delete measured subjects/builds after saving
sanitized aggregate receipts; never commit source, raw file lists or subject
people/identity metadata (X-R5).

## Replay

```sh
nice -n 10 python3.13 -B calibration/oracles/ruby-native/bootstrap.py --out workspace/tools/ruby-native
nice -n 10 bash scripts/fathom-go.sh build -o workspace/tools/indexbench ./cmd/indexbench
nice -n 10 python3.13 -B calibration/oracles/ruby-native/measure_typeprof.py \
  --tools workspace/tools/ruby-native --indexbench workspace/tools/indexbench \
  --out workspace/artifacts/lang-ruby/typeprof
nice -n 10 python3.13 -B calibration/oracles/ruby-native/build_openssl.py --tools workspace/tools/ruby-native
nice -n 10 python3.13 -B calibration/oracles/ruby-native/install_bundles.py --tools workspace/tools/ruby-native
nice -n 10 python3.13 -B calibration/oracles/ruby-native/run_traces.py \
  --tools workspace/tools/ruby-native --out workspace/artifacts/lang-ruby/runtime
```

Run TypeProf before bundles/tests generate untracked subject input. The exact
section 13.3.1 tracked-set recipe is `tracked_set.py`. Before/after manifests
cover all tracked kinds, hash symlink targets without following links, and
record gitlinks without claiming submodule contents. Native analysis includes
all original tracked eligible Ruby files; generated/ignored inputs must be
inspected, never silently substituted for original scope. Runtime dependencies
are installed in separate directories outside the original source scopes.

Ruby 3.3.8 portable archive, TypeProf 0.33.2, Prism 2.0.0 and native RBS 4.2.0
artifacts have checksum pins. The native RBS platform is deliberately `ruby`:
the unqualified version API can return a Java artifact. The bundle SDK builds
OpenSSL 3.4.1 inside workspace for the unchanged declared dependency constraints.
The subject Gemfiles are not rewritten; compatible installed gems are preferred.
Dependency resolution can drift; save sanitized package versions and lock hashes.

## Static truth and scope

OL-R10 admits released Ruby-org TypeProf as tier A. `typeprof_export.rb` loads
all eligible tracked `.rb`, `.rake`, `.ru`, Rakefile and Ruby/JRuby shebang files.
The original pins declare no tracked RBS files or rbs_collection configuration;
the released core RBS environment supplies built-in signatures. Call endpoints
come from TypeProf's MethodCallBox resolution, including its own `new` to
`initialize` handling and inherited/alias target identities. Constant answers
come from native ConstantRead bindings. Native MethodDefBox ranges identify the
caller; no syntax-walk caller/callee pair is accepted as an answer.

All native unresolved sites and rejected inputs are counted. The independent
upstream tree-sitter query supplies only a whole-written-pair coverage audit;
unmatched pairs remain unattested, including written aliases whose native target
has a different name. This is conservative written-name agreement, not a claim
that all such disagreements are unresolved semantic targets. Native-pair recall
is reported separately and cannot conceal whole-population misses. Full native
truth, not syntax pairs filtered by response presence, enters `lang-oracle`.

Jekyll's current full discovery includes 18 extensionless shebang scripts absent
from its historical canonical reading: 183 files / 709,051 bytes versus 165 /
694,927. Retain all 18; the larger comparison is supplementary and cannot replace
the canonical reading or be presented as an equal-set improvement.

## Executed truth and failure disclosure

OL-R10 admits native TracePoint only on the executed population. `:call` and
`:c_call` events supply the actual target, while native caller_locations supply
the source location and caller. Reflection plus ISeq code_location independently
validate the enclosing native caller. The C syntax query supplies only the
literal-site denominator and ambiguity guards. Aliases, reflective same-line
calls, duplicates, implicit callbacks and missing native ranges stay unattested.
Unexecuted sites remain in coverage counts and never enter runtime recall.
Runtime coverage cannot trigger a whole-written-population L2 grade under P-T1.

Run the original root Sinatra `rake test`, its own contrib/protection `rake spec`
tasks and Jekyll `rake test`. An occupied original Sinatra integration port uses
its own `test:core` task, disclosed with the original full denominator. Normal
suite failures are disclosed; timeout/signal refuses the whole unit. Only each
owned process group is terminated. The hook activates no gems before Bundler
selects the declared environment and retains Ruby's original File.expand_path
method so subject stubs cannot intercept instrumentation.

## Proofs

```sh
python3.13 -B -m unittest discover -s calibration/oracles/ruby-native -p 'test_*.py'
nice -n 10 python3.13 -B calibration/oracles/ruby-native/prove_mutations.py \
  --tools workspace/tools/ruby-native --indexbench workspace/tools/indexbench \
  --out workspace/artifacts/lang-ruby/owner-mutations
python3.13 -B calibration/oracles/ruby-native/prove_hook.py --tools workspace/tools/ruby-native
nice -n 10 python3.13 -B calibration/oracles/ruby-native/prove_native.py \
  --tools workspace/tools/ruby-native --indexbench workspace/tools/indexbench \
  --out workspace/artifacts/lang-ruby/proof
```

The fixture's inherited alias must resolve to the actual inherited method,
unknown receivers remain unresolved, and a known native `length` to `bytesize` edit
must change exactly one TypeProf and one TracePoint pair. Real extractor output
is placed inside both the subject and native output directories, then replaced
with fabricated pairs; native truth digests must remain unchanged. No extractor
output is a native input. `prove_hook.py` reproduces the subject's File stub and
asserts no pre-Bundler JSON/Set activation.

Mutation tests change native-caller attribution, callback/reflective/alias guards
and whole-collection caps; each focused assertion must FAIL before restoration
and GREEN afterward. Decoder caps are 64 raw files, 256 MiB total, 128 MiB per
file, 1,000,000 rows and 100,000 native ranges per file. Each has a reduced-limit
cap+1 test. Invalid/truncated JSON, raw refusal or wrong runtime refuses the
whole decode. Owner caps are 2,048 source files, 64 MiB source and 1,000,000
TypeProf answers; the hook also bounds unique native rows/ranges. The native
proof exercises lower-only limits at cap+1 and verifies no partial truth.

`score_runtime.py --tools <tools> --runtime <runtime> --indexbench <binary> --out <fresh-output>`
scores only native executed-pair recall. `audit_runtime.py --runtime <runtime>
--scored <scored-output> --out <fresh-json>` re-decodes all saved raw events and
checks every driver log under current fail-closed rules; truth must remain equal.
The final assessment publishes this revalidation and code hashes, never a claim
that discarded/incomplete traces passed newer rules.
