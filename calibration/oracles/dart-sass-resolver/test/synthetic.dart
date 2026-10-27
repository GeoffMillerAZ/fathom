import 'dart:io';

import '../bin/resolver.dart';

void main() {
  final repo = Directory.current.resolveSymbolicLinksSync();
  final scratch = Directory('${repo}${Platform.pathSeparator}workspace${Platform.pathSeparator}tmp');
  scratch.createSync(recursive: true);
  final root = Directory('${scratch.path}${Platform.pathSeparator}dart-sass-resolver-${pid}');
  root.createSync();
  try {
    final src = Directory('${root.path}${Platform.pathSeparator}src');
    final support = Directory('${root.path}${Platform.pathSeparator}support');
    Directory('${src.path}${Platform.pathSeparator}partials').createSync(recursive: true);
    Directory('${src.path}${Platform.pathSeparator}tokens').createSync();
    support.createSync();
    File('${src.path}${Platform.pathSeparator}main.scss').writeAsStringSync('''
@forward "tokens";
@use "sass:meta";
@use "same";
@use "partials/card";
@use "broken";
@use "load-path-only";
@use "missing";
@use "sass:color";
@use "reset.css";
@use "https://cdn.example.invalid/theme.scss";
@import "legacy";
.dynamic-container {
  @include meta.load-css("same");
}
@mixin include-compile-only {
  @content;
}
@include include-compile-only {
  @include meta.load-css("compile-only");
}
''');
    File('${src.path}${Platform.pathSeparator}partials${Platform.pathSeparator}_card.scss')
        .writeAsStringSync('.card { color: red; }\n');
    File('${src.path}${Platform.pathSeparator}tokens${Platform.pathSeparator}_index.scss')
        .writeAsStringSync('\$token: blue;\n');
    File('${src.path}${Platform.pathSeparator}_legacy.scss').writeAsStringSync('.legacy { color: red; }\n');
    File('${src.path}${Platform.pathSeparator}_broken.scss')
        .writeAsStringSync(r'$value: $undefinedAtEvaluation;');
    File('${src.path}${Platform.pathSeparator}_same.scss')
        .writeAsStringSync('.same { color: purple; }\n');
    File('${src.path}${Platform.pathSeparator}_compile-only.scss')
        .writeAsStringSync('.compile-only { color: teal; }\n');
    File('${support.path}${Platform.pathSeparator}_load-path-only.scss')
        .writeAsStringSync('.support { color: green; }\n');

    final withoutLoadPath = resolveSubject(root: root, directory: 'src');
    _check(withoutLoadPath['native_dependencies'] is List, 'native diagnostics missing');
    _check((withoutLoadPath['metadata']! as Map)['source_files'] == 7, 'source count');
    final primary = _primaryFixture();
    final composedWithout = composePrimary(primary, withoutLoadPath);
    final neutralWithout = composedWithout['primary']! as Map<String, Object?>;
    final importsWithout = (neutralWithout['imports']! as List).cast<Map<String, Object?>>();
    _check(neutralWithout['tool'] == 'dart-sass+native-resolver', 'composite producer identity names both producers');
    _check(neutralWithout['version'] == 'synthetic; sass_api 18.1.0 (sass 1.106.1) import resolver', 'composite producer versions are exact');
    _check(neutralWithout['tier'] == 'A', 'primary tier preserved');
    _check(neutralWithout['definitions'] == primary['definitions'], 'primary fields preserved');
    _check(importsWithout.length == (primary['imports']! as List).length, 'all primary imports preserved');
    _check(importsWithout.every((entry) =>
        entry.keys.toSet().containsAll({'path', 'line', 'module', 'to_path', 'primary_marker'}) &&
        entry['primary_marker'] == 'primary-${entry['line']}'), 'primary import fields preserved');
    _check(importsWithout.singleWhere((entry) => entry['module'] == 'partials/card')['to_path'] == 'src/partials/_card.scss', 'partial resolution');
    _check(importsWithout.singleWhere((entry) => entry['module'] == 'tokens')['to_path'] == 'src/tokens/_index.scss', 'index resolution');
    _check(importsWithout.singleWhere((entry) => entry['module'] == 'same')['to_path'] == 'src/_same.scss', 'positioned @use keeps its native edge');
    _check(importsWithout.singleWhere((entry) => entry['module'] == 'legacy')['to_path'] == 'src/_legacy.scss', 'legacy import resolution');
    _check(importsWithout.singleWhere((entry) => entry['module'] == 'broken')['to_path'] == 'src/_broken.scss', 'resolution does not evaluate source');
    _check(importsWithout.singleWhere((entry) => entry['module'] == 'missing')['to_path'] == '', 'missing target retained unresolved');
    _check(importsWithout.any((entry) => entry['module'] == 'sass:color' && entry['to_path'] == ''), 'builtin skip');
    _check(importsWithout.any((entry) => entry['module'] == 'reset.css' && entry['to_path'] == ''), 'CSS skip');
    _check(importsWithout.any((entry) => entry['module'] == 'https://cdn.example.invalid/theme.scss' && entry['to_path'] == ''), 'external skip');
    _check((composedWithout['metadata']! as Map).containsKey('file_resolved_edges'), 'native pairs retained in diagnostics');
    final nativeMetadata = composedWithout['metadata']! as Map;
    final unpositioned = (nativeMetadata['unpositioned_native_dependencies']! as List).cast<Map<String, Object?>>();
    _check(nativeMetadata['unpositioned_native_count'] == 1, 'unpositioned meta.load-css count');
    _check((nativeMetadata['unpositioned_native_by_kind']! as Map)['meta.load-css'] == 1, 'unpositioned family count despite shared URL');
    _check(unpositioned.single['module'] == 'same' && unpositioned.single['to_path'] == 'src/_same.scss', 'same-URL unpositioned native resolution retained');
    _check(((nativeMetadata['file_resolved_edges']! as List).cast<Map<String, String>>())
        .any((pair) => pair['to_path'] == 'src/_same.scss'), 'direct native pair retained');
    final primaryWithNativeGap = _primaryFixture();
    (primaryWithNativeGap['imports']! as List).add(<String, Object?>{
      'path': 'src/main.scss',
      'line': 19,
      'module': 'compile-only',
      'to_path': 'src/_compile-only.scss',
      'primary_marker': 'primary-19',
    });
    final composedGap = composePrimary(primaryWithNativeGap, withoutLoadPath);
    final gapMetadata = composedGap['metadata']! as Map;
    final retained = ((composedGap['primary']! as Map)['imports']! as List)
        .cast<Map<String, Object?>>()
        .singleWhere((entry) => entry['module'] == 'compile-only');
    _check(retained['to_path'] == 'src/_compile-only.scss', 'old primary attestation survives missing native statement');
    _check(gapMetadata['inherited_primary_attestations'] == 1, 'inherited primary attestation count');
    _check(gapMetadata['missing_native_statement_count'] == 1, 'missing native statement count');
    _check((gapMetadata['primary_unresolved_coverage_count'] as int) > 0, 'primary unresolved coverage reported');
    final conflictPrimary = _primaryFixture();
    ((conflictPrimary['imports']! as List).cast<Map<String, Object?>>())
        .singleWhere((entry) => entry['module'] == 'same')['to_path'] = 'src/_broken.scss';
    _throws(() => composePrimary(conflictPrimary, withoutLoadPath), 'conflicting native and primary attestations must fail');
    final incompletePrimary = _primaryFixture();
    (incompletePrimary['imports']! as List).removeWhere((entry) => (entry as Map)['module'] == 'partials/card');
    _throws(() => composePrimary(incompletePrimary, withoutLoadPath), 'resolved native URL missing from primary must fail');
    final missingPrimary = _primaryFixture();
    (missingPrimary['imports']! as List).removeWhere((entry) => (entry as Map)['module'] == 'missing');
    _throws(() => composePrimary(missingPrimary, withoutLoadPath), 'unresolved supported dependency missing from primary must fail');

    final withLoadPath = resolveSubject(root: root, directory: 'src', loadPaths: [support.path]);
    final loadPathMetadata = withLoadPath['metadata']! as Map;
    _check((loadPathMetadata['skipped']! as Map)['out_of_scope'] == 1, 'out-of-scope load path counted');
    final composedWith = composePrimary(primary, withLoadPath);
    final resolvedEntry = ((composedWith['primary']! as Map)['imports']! as List)
        .cast<Map<String, Object?>>()
        .singleWhere((entry) => entry['module'] == 'load-path-only');
    _check(resolvedEntry['to_path'] == '', 'out-of-scope target does not become file edge');
  } finally {
    root.deleteSync(recursive: true);
  }
  stdout.writeln('native Sass resolver synthetic controls passed');
}

void _check(bool condition, String message) {
  if (!condition) throw StateError('synthetic control failed: $message');
}

void _throws(void Function() action, String message) {
  try {
    action();
  } on StateError {
    return;
  }
  throw StateError('synthetic control failed: $message');
}

Map<String, dynamic> _primaryFixture() => {
      'tool': 'independent-primary-fixture',
      'version': 'synthetic',
      'tier': 'A',
      'definitions': [<String, Object?>{'path': 'src/main.scss', 'line': 1, 'name': 'kept'}],
      'calls': [<String, Object?>{'path': 'src/main.scss', 'line': 1, 'name': 'kept'}],
      'imports': [
        for (final entry in <(int, String)>[
          (1, 'tokens'),
          (2, 'sass:meta'),
          (3, 'same'),
          (4, 'partials/card'),
          (5, 'broken'),
          (6, 'load-path-only'),
          (7, 'missing'),
          (8, 'sass:color'),
          (9, 'reset.css'),
          (10, 'https://cdn.example.invalid/theme.scss'),
          (11, 'legacy'),
        ])
          <String, Object?>{
            'path': 'src/main.scss',
            'line': entry.$1,
            'module': entry.$2,
            'to_path': '',
            'primary_marker': 'primary-${entry.$1}',
          },
      ],
    };
