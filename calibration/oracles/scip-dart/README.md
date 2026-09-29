# scip-dart — tier-A SCIP index for Dart (LB-Q-dart)

The maintained indexer, `scip_dart` 1.7.0 (Apache-2.0, Workiva/scip-dart), a Dart program over the SDK's `analyzer`. Bench-only:
installed with the Dart SDK inside a scratch directory, nothing global and nothing in the product.

    curl -sL https://storage.googleapis.com/dart-archive/channels/stable/release/3.13.4/sdk/dartsdk-macos-arm64-release.zip -o dartsdk.zip
    unzip -q dartsdk.zip -d sdk        # dart 3.13.4 (stable), 625 MB unpacked
    export PATH=$PWD/sdk/dart-sdk/bin:$PATH PUB_CACHE=$PWD/pubcache
    dart pub global activate scip_dart # 1.7.0 (needs an SDK >= 3.12), ~84 MB of packages

Run once per package, in the package's own directory, after `dart pub get` (a Flutter package cannot resolve without the Flutter SDK
and is not measured). The root argument must be `.`: `scip_dart ./` treats the package's own pubspec as a nested package and writes an
index with no documents.

    cd <checkout>/packages/bloc && dart pub get && dart pub global run scip_dart . -o <scratch>/bloc.scip

One to seven seconds a package (`Analyzing Source` is nearly all of it). Packages measured (docs/bench/readings/lang/dart.json,
225 files): felangel/bloc `packages/{bloc,bloc_test,bloc_concurrency}` at bloc-v9.2.1 and dart-lang/http
`pkgs/{http,http_parser,http2,web_socket_channel,http_multi_server}` at http-v1.5.0.

What the index carries: `SymbolInformation.kind` on every symbol (Class, Mixin, Enum, TypeAlias, Extension, Constructor, Method,
Function, Variable, Property for an accessor, Field for a field or an enum constant, Namespace for an `import ... as p` prefix and a file,
Parameter, TypeParameter) and an `enclosing_range` on every definition, the whole declaration including its body, so call truth needs
no fallback. Its shapes, and how `internal/langbench` reads them (all scoped to `scip-dart` symbols or to the `dart` dialect):

- A private member (a leading `_`) and a local variable are `local N` symbols. The kind still says what a private one is, so a private
  field, accessor, variable, method or class is a definition (`SCIPDefs` reads its name from the source at the occurrence), and a call
  inside a private method belongs to that method (`SCIPOptions.PrivateMembers`); a local variable (kind unspecified) and a parameter are
  not definitions.
- An unnamed constructor is `Foo#<constructor>().`, named as its class (`Foo`, which is also how `Foo(...)` is written as a call); an
  accessor is `Foo#<get>x.` / `<set>x`, named x; a file is a `/` descriptor and an `import ... as p` prefix a Namespace, neither a
  definition.
- The producer's kind is authoritative: a `get`, `const` or `operator` written before a name is not a different kind (`keywordBefore` is
  for scip-typescript's declaration merging).
- A callable reference is a call site only where its name is written followed by `(` or by type arguments and `(`. The producer also
  references a tear-off (`onError: controller.addError`), an operator and the `super(...)` of an initializer; they are counted as
  `scip_calls_unwritten` and disclosed. A call through a function-typed parameter or field (`emit(state)`) is a reference to the
  parameter, not to a callable descriptor, so the index omits it and the dialect's pair counts against `call_precision`.
- `typed_recall` counts every in-index reference: 46% of them (9,076 of 19,736 across the eight packages) target a parameter, a type
  parameter or a local variable, which no extractor emits as a symbol (`docs/plans/language-breadth.md`, `LB-Q-dart-rest`).
