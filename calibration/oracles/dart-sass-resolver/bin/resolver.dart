#!/usr/bin/env dart
// Native SCSS import resolver oracle. Resolution only; no stylesheet evaluation.
import 'dart:convert';
import 'dart:io';

import 'package:sass_api/sass_api.dart' as sass;

const _sassApiVersion = '18.1.0';
const _sassVersion = '1.106.1';

class _Dependency {
  const _Dependency(this.kind, this.url, {this.forImport = false});

  final String kind;
  final Uri url;
  final bool forImport;
}

Map<String, Object?> resolveSubject({
  required Directory root,
  String? directory,
  List<String> loadPaths = const [],
}) {
  final rootPath = root.resolveSymbolicLinksSync();
  final rootDir = Directory(rootPath);
  final scopePath = directory == null
      ? rootPath
      : Directory(directory.startsWith(Platform.pathSeparator)
              ? directory
              : '$rootPath${Platform.pathSeparator}$directory')
          .absolute
          .resolveSymbolicLinksSync();
  if (!_isWithin(rootPath, scopePath)) {
    throw ArgumentError('--dir must resolve to a directory within ROOT');
  }

  final files = _scssFiles(Directory(scopePath));
  final before = _sourceFingerprints(rootDir, files);
  final scopedPaths = <String, String>{};
  for (final file in files) {
    final rel = _relativeTo(rootPath, file);
    scopedPaths[File(file).resolveSymbolicLinksSync()] = rel;
  }

  final cache = sass.ImportCache(loadPaths: loadPaths);
  final imports = <Map<String, Object?>>[];
  final edges = <String>{};
  final failures = <Map<String, String>>[];
  final skipped = <String, int>{
    'builtin': 0,
    'external': 0,
    'css': 0,
    'out_of_scope': 0,
  };
  var resolved = 0;
  var unresolved = 0;
  var declared = 0;
  var parseFailures = 0;

  for (final file in files) {
    final fromPath = _relativeTo(rootPath, file);
    final baseUrl = Uri.file(file);
    late final sass.Stylesheet stylesheet;
    late final sass.DependencyReport report;
    try {
      stylesheet = sass.Stylesheet.parseScss(File(file).readAsStringSync(), url: baseUrl);
      report = sass.findDependencies(stylesheet);
    } catch (error) {
      parseFailures++;
      failures.add({'path': fromPath, 'stage': 'parse', 'error': error.runtimeType.toString()});
      continue;
    }

    final dependencies = <_Dependency>[
      for (final url in report.uses) _Dependency('use', url),
      for (final url in report.forwards) _Dependency('forward', url),
      for (final url in report.imports) _Dependency('import', url, forImport: true),
      for (final url in report.metaLoadCss) _Dependency('meta.load-css', url),
    ];

    // findDependencies excludes Sass built-ins from uses/forwards. Preserve
    // their declaration as an explicit skip, without asserting a file edge.
    for (final rule in stylesheet.uses) {
      if (rule.url.scheme == 'sass' &&
          !dependencies.any((dependency) => dependency.kind == 'use' && dependency.url == rule.url)) {
        dependencies.add(_Dependency('use', rule.url));
      }
    }
    for (final rule in stylesheet.forwards) {
      if (rule.url.scheme == 'sass' &&
          !dependencies.any((dependency) => dependency.kind == 'forward' && dependency.url == rule.url)) {
        dependencies.add(_Dependency('forward', rule.url));
      }
    }

    dependencies.sort((a, b) {
      final byKind = a.kind.compareTo(b.kind);
      return byKind != 0 ? byKind : a.url.toString().compareTo(b.url.toString());
    });
    for (final dependency in dependencies) {
      declared++;
      final entry = <String, Object?>{
        'path': fromPath,
        'to_path': '',
        'module': dependency.url.toString(),
        'resolution': '',
        'kind': dependency.kind,
      };
      final skip = _skipKind(dependency.url);
      if (skip != null) {
        skipped[skip] = skipped[skip]! + 1;
        entry['resolution'] = 'skipped_$skip';
        imports.add(entry);
        continue;
      }

      try {
        final result = cache.canonicalize(
          dependency.url,
          baseImporter: sass.FilesystemImporter.noLoadPath,
          baseUrl: baseUrl,
          forImport: dependency.forImport,
        );
        final canonical = result == null ? null : result.$2;
        if (canonical == null) {
          unresolved++;
          entry['resolution'] = 'unresolved';
        } else if (canonical.scheme != 'file') {
          skipped['external'] = skipped['external']! + 1;
          entry['resolution'] = 'skipped_external';
        } else {
          final target = File.fromUri(canonical).resolveSymbolicLinksSync();
          final toPath = scopedPaths[target];
          if (toPath == null) {
            skipped['out_of_scope'] = skipped['out_of_scope']! + 1;
            entry['resolution'] = 'out_of_scope';
          } else {
            entry['to_path'] = toPath;
            entry['resolution'] = 'resolved';
            resolved++;
            edges.add('$fromPath\u0000$toPath');
          }
        }
      } catch (error) {
        unresolved++;
        entry['resolution'] = 'error';
        failures.add({
          'path': fromPath,
          'stage': 'canonicalize',
          'module': dependency.url.toString(),
          'error': error.runtimeType.toString(),
        });
      }
      imports.add(entry);
    }
  }

  final currentFiles = _scssFiles(Directory(scopePath));
  if (!_sameList(files, currentFiles)) {
    throw StateError('SCSS source inventory changed during resolver measurement');
  }
  final after = _sourceFingerprints(rootDir, currentFiles);
  if (!_sameMap(before, after)) {
    throw StateError('SCSS sources changed during resolver measurement');
  }

  imports.sort((a, b) {
    final pathOrder = (a['path']! as String).compareTo(b['path']! as String);
    if (pathOrder != 0) return pathOrder;
    final moduleOrder = (a['module']! as String).compareTo(b['module']! as String);
    return moduleOrder != 0 ? moduleOrder : (a['kind']! as String).compareTo(b['kind']! as String);
  });
  final fileResolvedEdges = edges
      .map((edge) => edge.split('\u0000'))
      .map((pair) => <String, String>{'from_path': pair[0], 'to_path': pair[1]})
      .toList()
    ..sort((a, b) {
      final fromOrder = a['from_path']!.compareTo(b['from_path']!);
      return fromOrder != 0 ? fromOrder : a['to_path']!.compareTo(b['to_path']!);
    });

  return {
    'tool': 'sass-api-native-resolver',
    'version': 'sass_api $_sassApiVersion (sass $_sassVersion)',
    'tier': 'A',
    'native_dependencies': imports,
    'file_resolved_edges': fileResolvedEdges,
    'disclose': [
      'Dart Sass ImportCache canonicalization resolves declared SCSS dependencies without evaluating stylesheets. '
          '$declared dependency URLs: $resolved resolved to scoped files, $unresolved unresolved or failed, '
          '${skipped['builtin']} Sass built-ins, ${skipped['external']} external URLs, ${skipped['css']} CSS URLs, '
          '${skipped['out_of_scope']} targets outside the measured scope. '
          'Parse failures: $parseFailures. meta.load-css without primary spans is reported as unpositioned coverage. '
          'Empty to_path entries with a skip status are not negative file-edge assertions.'
    ],
    'metadata': {
      'scope': _relativeTo(rootPath, scopePath),
      'source_files': files.length,
      'source_fingerprints_algorithm': 'SHA-256 via shasum',
      'source_fingerprints_before': before,
      'source_fingerprints_after': after,
      'load_paths': loadPaths,
      'declared_dependencies': declared,
      'resolved_dependencies': resolved,
      'unresolved_dependencies': unresolved,
      'skipped': skipped,
      'parse_failures': parseFailures,
      'failures': failures,
    },
  };
}

String _normalizedUrl(String value) {
  final uri = Uri.parse(value);
  return uri.normalizePath().toString();
}

Map<String, Object?> composePrimary(Map<String, dynamic> primary, Map<String, Object?> native) {
  final importsValue = primary['imports'];
  if (importsValue is! List) throw FormatException('primary JSON must contain an imports array');
  final byKey = <String, List<Map<String, Object?>>>{};
  for (final item in (native['native_dependencies']! as List).cast<Map<String, Object?>>()) {
    final key = '${item['path']}\u0000${_normalizedUrl(item['module']! as String)}';
    byKey.putIfAbsent(key, () => []).add(item);
  }
  // DependencyReport exposes URL sets per family, not source occurrences.
  // Preserve every meta.load-css family URL as unpositioned evidence even if
  // the same path/URL also appears as a positioned @use/@import primary row.
  final unpositionedNative = (native['native_dependencies']! as List)
      .cast<Map<String, Object?>>()
      .where((item) => item['kind'] == 'meta.load-css')
      .map((item) => Map<String, Object?>.from(item))
      .toList();
  final unpositionedByKind = <String, int>{};
  for (final item in unpositionedNative) {
    final kind = item['kind']! as String;
    unpositionedByKind[kind] = (unpositionedByKind[kind] ?? 0) + 1;
  }

  var unmapped = 0;
  var ambiguous = 0;
  final unmappedByKind = <String, int>{};
  final outputImports = <Map<String, Object?>>[];
  for (final raw in importsValue) {
    if (raw is! Map) throw FormatException('primary imports entries must be objects');
    final sourceEntry = raw;
    final path = sourceEntry['path'];
    final module = sourceEntry['module'];
    final line = sourceEntry['line'];
    if (path is! String || module is! String || line is! int) {
      throw FormatException('primary imports require path, line, and module');
    }
    final entry = Map<String, Object?>.from(sourceEntry.cast<String, Object?>());
    entry['to_path'] = '';
    final key = '$path\u0000${_normalizedUrl(module)}';
    final matches = (byKey[key] ?? const <Map<String, Object?>>[])
        .where((match) => match['kind'] != 'meta.load-css')
        .toList();
    final signatures = matches.map((match) => '${match['resolution']}\u0000${match['to_path']}').toSet();
    if (signatures.length == 1 && matches.isNotEmpty) {
      final match = matches.first;
      entry['to_path'] = match['to_path'];
    } else if (signatures.length > 1) {
      ambiguous++;
    }
    outputImports.add(entry);
  }

  final primaryKeys = <String>{};
  for (final raw in importsValue) {
    final entry = raw as Map;
    if (entry['path'] is String && entry['module'] is String) {
      primaryKeys.add('${entry['path']}\u0000${_normalizedUrl(entry['module'] as String)}');
    }
  }
  final unresolvedNative = <Map<String, Object?>>[];
  for (final entry in byKey.entries) {
    final signatures = entry.value.map((match) => '${match['resolution']}\u0000${match['to_path']}').toSet();
    final nativeEntry = signatures.length == 1 ? entry.value.first : null;
    if (!primaryKeys.contains(entry.key)) {
      unmapped += entry.value.length;
      for (final match in entry.value) {
        final kind = match['kind']! as String;
        unmappedByKind[kind] = (unmappedByKind[kind] ?? 0) + 1;
        if (kind == 'meta.load-css') {
          // Already included above, even when another family shares its key.
        } else {
          throw StateError('native dependency has no primary statement: ${match['path']} ${match['module']} ($kind)');
        }
      }
    }
    if (nativeEntry != null && nativeEntry['resolution'] == 'unresolved') unresolvedNative.add(nativeEntry);
  }

  final composite = Map<String, Object?>.from(primary.cast<String, Object?>());
  composite['imports'] = outputImports;
  final metadata = Map<String, Object?>.from(native);
  metadata['unmapped_native_dependencies'] = unmapped;
  metadata['unmapped_native_by_kind'] = unmappedByKind;
  metadata['unpositioned_native_dependencies'] = unpositionedNative;
  metadata['unpositioned_native_by_kind'] = unpositionedByKind;
  metadata['unpositioned_native_count'] = unpositionedNative.length;
  metadata['unpositioned_native_resolved_count'] = unpositionedNative.where((item) => item['resolution'] == 'resolved').length;
  metadata['unpositioned_native_unresolved_count'] = unpositionedNative.where((item) => item['resolution'] == 'unresolved' || item['resolution'] == 'error').length;
  metadata['ambiguous_native_dependencies'] = ambiguous;
  metadata['unresolved_native_dependencies'] = unresolvedNative;
  return {'primary': composite, 'metadata': metadata};
}

List<String> _scssFiles(Directory scope) {
  if (!scope.existsSync()) throw ArgumentError('SCSS scope does not exist');
  return scope
      .listSync(recursive: true, followLinks: false)
      .where((entity) =>
          FileSystemEntity.typeSync(entity.path, followLinks: false) == FileSystemEntityType.file &&
          entity.path.toLowerCase().endsWith('.scss'))
      .map((entity) => File(entity.path).absolute.path)
      .toList()
    ..sort();
}

Map<String, String> _sourceFingerprints(Directory root, List<String> files) {
  final hashes = <String, String>{};
  for (final file in files) {
    final result = Process.runSync('shasum', ['-a', '256', file], runInShell: false);
    if (result.exitCode != 0) {
      throw StateError('cannot hash SCSS source ${_relativeTo(root.path, file)}: ${result.stderr}');
    }
    hashes[_relativeTo(root.path, file)] = (result.stdout as String).split(RegExp(r'\s+')).first;
  }
  return hashes;
}

String? _skipKind(Uri url) {
  if (url.scheme == 'sass') return 'builtin';
  if ((url.scheme.isNotEmpty && url.scheme != 'file') || url.hasAuthority) return 'external';
  if (url.path.toLowerCase().endsWith('.css')) return 'css';
  return null;
}

String _relativeTo(String root, String path) {
  final rootPrefix = root.endsWith(Platform.pathSeparator) ? root : '$root${Platform.pathSeparator}';
  if (path == root) return '';
  if (!path.startsWith(rootPrefix)) throw ArgumentError('path escapes ROOT');
  return path.substring(rootPrefix.length).split(Platform.pathSeparator).join('/');
}

bool _isWithin(String root, String path) => path == root || path.startsWith(
      root.endsWith(Platform.pathSeparator) ? root : '$root${Platform.pathSeparator}',
    );

bool _sameMap(Map<String, String> a, Map<String, String> b) {
  if (a.length != b.length) return false;
  for (final entry in a.entries) {
    if (b[entry.key] != entry.value) return false;
  }
  return true;
}

bool _sameList(List<String> a, List<String> b) {
  if (a.length != b.length) return false;
  for (var i = 0; i < a.length; i++) {
    if (a[i] != b[i]) return false;
  }
  return true;
}

void main(List<String> args) {
  if (args.isEmpty) {
    stderr.writeln('usage: dart --packages=PACKAGE_CONFIG bin/resolver.dart ROOT [--dir RELATIVE] [--load-path DIR ...]');
    exitCode = 64;
    return;
  }
  final root = Directory(args.first);
  String? directory;
  String? primaryPath;
  String? metadataPath;
  final loadPaths = <String>[];
  for (var i = 1; i < args.length; i++) {
    switch (args[i]) {
      case '--dir':
        if (++i >= args.length || directory != null) throw ArgumentError('--dir requires one unique value');
        directory = args[i];
        break;
      case '--primary':
        if (++i >= args.length || primaryPath != null) throw ArgumentError('--primary requires one unique JSON file');
        primaryPath = args[i];
        break;
      case '--metadata':
        if (++i >= args.length || metadataPath != null) throw ArgumentError('--metadata requires one unique JSON file');
        metadataPath = args[i];
        break;
      case '--load-path':
        if (++i >= args.length) throw ArgumentError('--load-path requires a directory');
        loadPaths.add(args[i]);
        break;
      default:
        throw ArgumentError('unknown option ${args[i]}');
    }
  }
  if (primaryPath == null || metadataPath == null) {
    throw ArgumentError('--primary and --metadata are required');
  }
  final result = resolveSubject(root: root, directory: directory, loadPaths: loadPaths);
  final primary = jsonDecode(File(primaryPath).readAsStringSync()) as Map<String, dynamic>;
  final composed = composePrimary(primary, result);
  stdout.writeln(const JsonEncoder.withIndent('  ').convert(composed['primary']));
  File(metadataPath).writeAsStringSync('${const JsonEncoder.withIndent('  ').convert(composed['metadata'])}\n');
}
