"use strict";

// This adapter reads compiler facts only. The Go observation is neither an
// input to this program nor a fallback when a native result is unavailable.
const fs = require("node:fs");
const path = require("node:path");
const crypto = require("node:crypto");
const { builtinModules } = require("node:module");
const pins = require("./pins.json");
const [compiler, rootArg, pathsFile, traceFile, mode, injection] = process.argv.slice(2);
const classifyExit = mode === "--classify-native-exit";
const inject = classifyExit && injection === "--inject-native-failure";
const nativeNonzeroStatus = 79; // Private status consumed by the COV capture.
if (!traceFile || process.version !== pins.node) {
  throw new Error("expected pinned Node and arguments: compiler root file-list trace");
}
const sha = crypto.createHash("sha256").update(fs.readFileSync(compiler)).digest("hex");
if (sha !== pins.compilerSHA256) throw new Error("unexpected compiler digest");
const ts = require(path.resolve(compiler));
if (ts.version !== pins.typescript) throw new Error("unexpected compiler version");
const root = fs.realpathSync(rootArg);
const selected = JSON.parse(fs.readFileSync(pathsFile, "utf8"));
const traceFD = fs.openSync(traceFile, "w");
const stats = {
  files: selected.length,
  compilerListed: 0,
  configGroups: 0,
  configUnavailable: 0,
  outsideMembership: 0,
  notInProgram: 0,
  syntax: 0,
  optionDiagnostics: {},
  missingLocal: 0,
  missingBare: 0,
  computed: 0,
  direct: 0,
  external: 0,
  moduleTraceFallback: 0,
  trackedTargetsWithoutSourceFile: 0,
};
const builtin = new Set(builtinModules);
const sink = [];
let adapterFailure;
function adapterCallback(operation) {
  if (!classifyExit) return operation;
  return (...args) => {
    try {
      return operation(...args);
    } catch (error) {
      // Keep the first host failure fatal even if the compiler catches it.
      adapterFailure ??= { error };
      throw error;
    }
  };
}
function compilerOperation(operation) {
  if (!classifyExit) return operation();
  let result;
  try {
    result = operation();
  } catch {
    if (adapterFailure) throw adapterFailure.error;
    process.exit(nativeNonzeroStatus);
  }
  if (adapterFailure) throw adapterFailure.error;
  return result;
}
function flushTrace() {
  if (sink.length) {
    fs.writeSync(traceFD, sink.join("\n") + "\n");
    sink.length = 0;
  }
}
function trace(message) {
  sink.push(message);
  if (sink.length === 1024) flushTrace();
}
function confinedRelative(full) {
  const rel = path.relative(root, full);
  // An empty relative path identifies the root.
  if (rel === ".." || rel.startsWith(".." + path.sep) || path.isAbsolute(rel)) return undefined;
  return rel;
}
function logical(full) {
  return confinedRelative(full)?.split(path.sep).join("/") ?? "";
}
function within(full) {
  return confinedRelative(full) !== undefined;
}
function physicallyWithin(full) {
  if (!within(full)) return false;
  try { return within(fs.realpathSync(full)); } catch { return false; }
}
function fileExists(file) {
  return physicallyWithin(file) && ts.sys.fileExists(file);
}
function directoryExists(dir) {
  return physicallyWithin(dir) && ts.sys.directoryExists(dir);
}
function readFile(file) {
  return physicallyWithin(file) ? ts.sys.readFile(file) : undefined;
}
const disk = {
  useCaseSensitiveFileNames: true,
  fileExists: adapterCallback(fileExists),
  readFile: adapterCallback(readFile),
  directoryExists: adapterCallback(directoryExists),
  getDirectories: adapterCallback((dir) => directoryExists(dir) ? ts.sys.getDirectories(dir).filter(physicallyWithin) : []),
  readDirectory: adapterCallback((dir, ...args) => directoryExists(dir) ? ts.sys.readDirectory(dir, ...args).filter(physicallyWithin) : []),
  realpath: adapterCallback((file) => physicallyWithin(file) ? ts.sys.realpath(file) : file),
  getCurrentDirectory: () => root,
  trace: adapterCallback(trace),
};
function configFor(file) {
  for (let dir = path.dirname(file); ; dir = path.dirname(dir)) {
    for (const name of ["tsconfig.json", "jsconfig.json"]) {
      const candidate = path.join(dir, name);
      if (fileExists(candidate)) return candidate;
    }
    if (dir === root) return "";
  }
}
const groups = new Map();
for (const rel of selected) {
  const full = path.resolve(root, rel);
  if (logical(full) !== rel || !fileExists(full)) throw new Error("selected file absent");
  const config = configFor(full);
  if (!groups.has(config)) groups.set(config, []);
  groups.get(config).push(full);
}
const truth = [];
for (const [configFile, files] of groups) {
  const report = (d) => trace("TS" + d.code + " " + ts.flattenDiagnosticMessageText(d.messageText, "\n"));
  let parsed;
  if (configFile) {
    stats.configGroups++;
    const configHost = { ...disk, onUnRecoverableConfigFileDiagnostic: adapterCallback(report) };
    parsed = compilerOperation(() => ts.getParsedCommandLineOfConfigFile(configFile, {}, configHost));
    const configErrors = parsed?.errors.filter((d) => d.code !== 18002 && d.code !== 18003);
    if (!parsed || configErrors.length) {
      for (const error of configErrors || []) report(error);
      stats.configUnavailable += files.length;
      continue;
    }
    const nativeRoots = new Set(parsed.fileNames);
    stats.outsideMembership += files.filter((file) => !nativeRoots.has(file)).length;
  }
  const options = {
    ...(parsed?.options || { module: ts.ModuleKind.Node16, moduleResolution: ts.ModuleResolutionKind.Node16 }),
    allowJs: true,
    noEmit: true,
    noLib: true,
    types: [],
    traceResolution: true,
  };
  const host = {
    ...compilerOperation(() => ts.createCompilerHost(options)),
    ...disk,
    useCaseSensitiveFileNames: () => true,
    writeFile: adapterCallback(() => { throw new Error("oracle attempted to emit a file"); }),
  };
  // createCompilerHost's default source reader closes over ts.sys. Construct
  // source files from the confined read, preserving the native Node mode.
  const readSource = adapterCallback((file, onError) => {
    const source = readFile(file);
    if (source === undefined && onError) onError("file not readable in pinned subject");
    return source;
  });
  host.getSourceFile = (file, languageVersion, onError) => {
    const source = readSource(file, onError);
    return source === undefined ? undefined : ts.createSourceFile(file, source, languageVersion, true);
  };
  const programOptions = { rootNames: files, options, host, projectReferences: parsed?.projectReferences };
  const program = compilerOperation(() => {
    if (inject) throw new Error("synthetic compiler failure");
    return ts.createProgram(programOptions);
  });
  for (const error of compilerOperation(() => program.getOptionsDiagnostics())) {
    stats.optionDiagnostics[error.code] = (stats.optionDiagnostics[error.code] || 0) + 1;
    report(error);
  }
  const nativeFiles = new Set(compilerOperation(() => program.getSourceFiles()).map((file) => file.fileName));
  trace("Compiler file list:");
  for (const file of nativeFiles) trace(file);
  const fallbackCache = compilerOperation(() => ts.createModuleResolutionCache(root, (name) => name, options));
  for (const file of files) {
    const source = compilerOperation(() => program.getSourceFile(file));
    if (!nativeFiles.has(file) || !source) {
      stats.notInProgram++;
      continue;
    }
    stats.compilerListed++;
    if (compilerOperation(() => program.getSyntacticDiagnostics(source)).length) {
      stats.syntax++;
      continue;
    }
    const edges = [];
    const seen = new Set();
    function add(Kind, Raw, Line, Resolution, To) {
      const key = JSON.stringify([Kind, Raw, Line, Resolution, To]);
      if (!seen.has(key)) {
        seen.add(key);
        edges.push({ Kind, Raw, Line, Resolution, To });
      }
    }
    if (configFile) add("config-ref", logical(configFile), 0, "file", logical(configFile));
    const pkg = source.packageJsonScope?.packageDirectory;
    if (pkg && logical(path.join(pkg, "package.json"))) {
      const id = logical(path.join(pkg, "package.json"));
      add("config-ref", id, 0, "file", id);
    }
    for (const ref of source.referencedFiles) {
      const target = path.resolve(path.dirname(file), ref.fileName);
      if (logical(target) && fileExists(target)) {
        add("config-ref", ref.fileName, source.getLineAndCharacterOfPosition(ref.pos).line + 1, "file", logical(target));
      }
    }
    for (const ref of source.typeReferenceDirectives) {
      const typeName = ref.fileName;
      const answer = compilerOperation(() => ts.resolveTypeReferenceDirective(typeName, file, options, disk));
      const target = answer.resolvedTypeReferenceDirective?.resolvedFileName;
      if (target && logical(target)) {
        add("config-ref", ref.fileName, source.getLineAndCharacterOfPosition(ref.pos).line + 1, "file", logical(target));
      }
    }
    const examined = new Set();
    function moduleRequest(node, forcedMode) {
      if (!node || !ts.isStringLiteralLike(node)) {
        stats.computed++;
        return;
      }
      const key = node.pos + ":" + node.end + ":" + node.text;
      if (examined.has(key)) return;
      examined.add(key);
      const line = node.pos < 0 ? 0 : source.getLineAndCharacterOfPosition(node.getStart(source)).line + 1;
      const mode = forcedMode ?? compilerOperation(() => program.getModeForUsageLocation(source, node));
      const request = node.text;
      let result = compilerOperation(() => program.getResolvedModule(source, request, mode));
      if (!result) {
        // TypeScript's Program only enumerates a subset of literal calls
        // (for example, require in a TS file and require.resolve are absent).
        // Run its same resolver for those AST requests, under their mode.
        stats.moduleTraceFallback++;
        result = compilerOperation(() => ts.resolveModuleName(request, file, options, disk, fallbackCache, undefined, mode));
      }
      const target = result.resolvedModule?.resolvedFileName;
      if (target && logical(target)) {
        add("import", node.text, line, "file", logical(target));
        stats.direct++;
        if (!nativeFiles.has(target)) stats.trackedTargetsWithoutSourceFile++;
      } else if (target || builtin.has(node.text) || node.text.startsWith("node:") && builtin.has(node.text.slice(5))) {
        add("import", node.text, line, "external", "node:" + node.text.replace(/^node:/, ""));
        stats.external++;
      } else if (node.text === "." || node.text === ".." || /^\.\.?\//.test(node.text)) {
        stats.missingLocal++;
      } else {
        stats.missingBare++;
      }
    }
    function walk(node) {
      if (ts.isImportDeclaration(node) || ts.isExportDeclaration(node) || ts.isJSDocImportTag(node)) {
        if (node.moduleSpecifier) moduleRequest(node.moduleSpecifier);
      } else if (ts.isImportEqualsDeclaration(node) && ts.isExternalModuleReference(node.moduleReference)) {
        moduleRequest(node.moduleReference.expression, ts.ModuleKind.CommonJS);
      } else if (ts.isImportTypeNode(node) && ts.isLiteralTypeNode(node.argument)) {
        moduleRequest(node.argument.literal);
      } else if (ts.isCallExpression(node)) {
        const callee = node.expression;
        if (callee.kind === ts.SyntaxKind.ImportKeyword) {
          moduleRequest(node.arguments[0], ts.ModuleKind.ESNext);
        } else if (ts.isIdentifier(callee) && callee.text === "require" ||
            ts.isPropertyAccessExpression(callee) && ts.isIdentifier(callee.expression) &&
            callee.expression.text === "require" && callee.name.text === "resolve") {
          moduleRequest(node.arguments[0], ts.ModuleKind.CommonJS);
        }
      }
      if (node.jsDoc) node.jsDoc.forEach(walk);
      ts.forEachChild(node, walk);
    }
    walk(source);
    for (const item of source.imports || []) moduleRequest(item);
    truth.push({ Path: logical(file), Edges: edges });
  }
}
flushTrace();
fs.closeSync(traceFD);
process.stdout.write(JSON.stringify({ truth, stats }) + "\n");
