// react_oracle.mjs: the language-breadth harness's oracle for React components,
// props, hooks and the edges between them (`indexbench lang-react`, README.md).
//
// Components and props are react-docgen-typescript's (MIT, the props table
// Storybook renders): its Parser.getComponentInfo decides what a component is
// and getPropsInfo what its props are, run with Storybook's default options over
// a TypeScript program built from the subject's own tsconfig.json. Two things
// widen docgen's own file walk, each disclosed: every module-scope declaration
// is offered to getComponentInfo, not only a file's exports (so an unexported
// component is truth too), and a re-export is followed to the declaration (a
// component is keyed where it is declared, once). Three rules then narrow
// docgen's type test, each counted under `excluded` by reason:
//
//   - a component's name starts with an upper-case letter (React's rule: JSX
//     reads a lower-case tag as an HTML element, so only a capitalized name can
//     be rendered as a component);
//   - its call signature returns a React node (an Element, a ReactNode, null)
//     or its instances have a render method: docgen admits any function of one
//     parameter, which renders nothing when it returns a Promise or a handler;
//   - it is not a context object (createContext): React 19's types make a
//     context renderable as its own provider, but it is no component definition.
//
// A prop is a docgen prop with a declaration in the subject (a library's
// inherited props are dropped, Storybook's propFilter applied to every
// declaration as well as the parent), graded at its property signature: named
// Owner.prop after the interface or type alias that declares it, or after the
// component when the declaring type literal has no name.
//
// The edges are the TypeScript checker's: a JSX element whose tag resolves to a
// component above, and a call that resolves to an in-repo function named
// use[A-Z0-9] (React's hook naming rule), give a pair from the enclosing
// definition to that component or hook (`calls`) and, across files, a file pair
// (`file_edges`); an import declaration that binds one of them gives the file
// its module resolves to (`imports`).
//
//	node react_oracle.mjs --tools <npm prefix holding react-docgen-typescript> \
//	  --app <project dir (its tsconfig.json)> --repo <checkout root> --out <oracle.json>
//
// The whole file is tier C: docgen's type test is a heuristic (tier C caps a
// level at good, design §1.3), and the edges are graded over the population it
// and the hook rule select.
import { createRequire } from "node:module";
import fs from "node:fs";
import path from "node:path";

const args = {};
for (let i = 2; i < process.argv.length; i += 2) args[process.argv[i].replace(/^--/, "")] = process.argv[i + 1];
for (const k of ["tools", "app", "repo", "out"]) {
  if (!args[k]) {
    console.error("usage: node react_oracle.mjs --tools DIR --app DIR --repo DIR --out FILE");
    process.exit(2);
  }
}
const require = createRequire(path.join(path.resolve(args.tools), "package.json"));
const ts = require("typescript");
const docgen = require("react-docgen-typescript");
const { Parser } = require("react-docgen-typescript/lib/parser.js");
const docgenVersion = require("react-docgen-typescript/package.json").version;

const app = path.resolve(args.app);
const repo = path.resolve(args.repo);
const rel = (f) => path.relative(repo, path.resolve(f)).split(path.sep).join("/");
const inRepo = (f) => !/[\\/]node_modules[\\/]/.test(f) && path.resolve(f).startsWith(repo + path.sep);

const SKIP = new Set(["node_modules", "dist", "build", "out", "coverage"]);
const EXT = /\.(tsx?|jsx?|mjs|cjs|mts|cts)$/;
const files = [];
(function walk(d) {
  for (const e of fs.readdirSync(d, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
    if (e.name.startsWith(".") || SKIP.has(e.name)) continue;
    const p = path.join(d, e.name);
    if (e.isDirectory()) walk(p);
    else if (EXT.test(e.name) && !/\.d\.[mc]?ts$/.test(e.name)) files.push(p);
  }
})(app);

let options = { jsx: ts.JsxEmit.Preserve, allowJs: true, esModuleInterop: true, skipLibCheck: true };
const tsconfig = path.join(app, "tsconfig.json");
if (fs.existsSync(tsconfig)) {
  const cfg = ts.getParsedCommandLineOfConfigFile(tsconfig, {}, { ...ts.sys, onUnRecoverableConfigFileDiagnostic: () => {} });
  if (cfg) options = { ...cfg.options, allowJs: true, noEmit: true };
}
const program = ts.createProgram(files, options);
const checker = program.getTypeChecker();
// Storybook's react-docgen-typescript options (literal enum values, optional
// props without undefined), with every prop kept: docgen's table hides an
// undocumented `children` and Storybook's filter drops a prop whose first
// declaration is a library's, both display choices; the subject test below
// grades each prop at its own declaration instead.
const parser = new Parser(program, {
  shouldExtractLiteralValuesFromEnum: true,
  shouldRemoveUndefinedFromOptional: true,
  skipChildrenPropWithoutDoc: false,
  propFilter: () => true,
});

const excluded = {};
const exclude = (why) => (excluded[why] = (excluded[why] || 0) + 1);
const included = {};
const added = (why) => (included[why] = (included[why] || 0) + 1);

// isFunction: a function declaration, or a variable bound to an arrow
// function or function expression.
function isFunction(decl) {
  return ts.isFunctionDeclaration(decl) || (ts.isVariableDeclaration(decl) && !!decl.initializer && (ts.isArrowFunction(decl.initializer) || ts.isFunctionExpression(decl.initializer)));
}
const lineOf = (n) => n.getSourceFile().getLineAndCharacterOfPosition(n.getStart()).line + 1;
const endOf = (n) => n.getSourceFile().getLineAndCharacterOfPosition(n.getEnd()).line + 1;
const resolveAlias = (s) => (s && s.flags & ts.SymbolFlags.Alias ? checker.getAliasedSymbol(s) : s);

// declOf is the declaration a component is keyed at: a function, class or
// variable declaration with an identifier name.
function declOf(sym) {
  for (const d of sym.declarations || []) {
    if ((ts.isFunctionDeclaration(d) || ts.isClassDeclaration(d) || ts.isVariableDeclaration(d)) && d.name && ts.isIdentifier(d.name)) return d;
  }
  return null;
}

// typeOf is a declaration's own type: a variable bound to a function is read
// as that function, not as its annotation (`const Home: NextPage = () => …`
// is annotated with a union of a function and a class type, which has no
// call signature of its own).
function typeOf(sym, decl) {
  if (ts.isVariableDeclaration(decl) && decl.initializer && (ts.isArrowFunction(decl.initializer) || ts.isFunctionExpression(decl.initializer))) {
    return checker.getTypeAtLocation(decl.initializer);
  }
  return checker.getTypeOfSymbolAtLocation(sym, decl);
}

const NODE_RE = /(^|\b)(JSX\.Element|Element|ReactElement|ReactNode|ReactPortal|ReactElement<.*>|null)(\b|$)/;
function renders(sym, decl) {
  const type = typeOf(sym, decl);
  for (const sig of type.getCallSignatures()) {
    const rt = checker.typeToString(sig.getReturnType(), undefined, ts.TypeFormatFlags.NoTruncation);
    if (NODE_RE.test(rt)) return true;
  }
  for (const sig of type.getConstructSignatures()) {
    if (sig.getReturnType().getProperty("render")) return true;
  }
  return false;
}

function isCreateContext(decl) {
  if (!ts.isVariableDeclaration(decl) || !decl.initializer || !ts.isCallExpression(decl.initializer)) return false;
  const t = decl.initializer.expression.getText();
  return /(^|\.)createContext$/.test(t);
}

// propsTypeOf replicates docgen's own props source (the first parameter of a
// call signature, or a class instance's props) to find each prop's symbol.
function propsTypeOf(sym, decl) {
  const type = typeOf(sym, decl);
  for (const sig of type.getCallSignatures()) {
    const ps = sig.getParameters();
    if (ps.length === 0) continue;
    if (ps[0].getName() === "props" || ps.length === 1) return checker.getTypeOfSymbolAtLocation(ps[0], decl);
  }
  for (const sig of type.getConstructSignatures()) {
    const p = sig.getReturnType().getProperty("props");
    if (p) return checker.getTypeOfSymbolAtLocation(p, decl);
  }
  return null;
}

// ownerOf names the type a property signature is declared in: an interface,
// or a type alias whose value holds the type literal (through intersections,
// unions and parentheses); "" for a type literal with no name.
function ownerOf(sig) {
  const p = sig.parent;
  if (ts.isInterfaceDeclaration(p)) return p.name.text;
  if (ts.isTypeLiteralNode(p)) {
    let q = p.parent;
    while (q && (ts.isIntersectionTypeNode(q) || ts.isUnionTypeNode(q) || ts.isParenthesizedTypeNode(q))) q = q.parent;
    if (q && ts.isTypeAliasDeclaration(q)) return q.name.text;
  }
  return "";
}

const defs = [];
const seenDef = new Set();
const components = new Map(); // decl -> name
for (const sf of program.getSourceFiles()) {
  if (!files.includes(path.resolve(sf.fileName)) && !files.includes(sf.fileName)) continue;
  const candidates = [];
  for (const st of sf.statements) {
    if ((ts.isFunctionDeclaration(st) || ts.isClassDeclaration(st)) && st.name) candidates.push(st.name);
    if (ts.isVariableStatement(st)) for (const d of st.declarationList.declarations) if (ts.isIdentifier(d.name)) candidates.push(d.name);
  }
  for (const id of candidates) {
    const sym = resolveAlias(checker.getSymbolAtLocation(id));
    if (!sym) continue;
    let info = null;
    try {
      info = parser.getComponentInfo(sym, sf);
    } catch (e) {
      exclude("docgen threw: " + String(e && e.message).slice(0, 60));
      continue;
    }
    const decl0 = declOf(sym);
    if (!info && decl0 && /^[A-Z]/.test(decl0.name.text) && isFunction(decl0) && renders(sym, decl0)) {
      // React's rule (a capitalized function returning a React node) where
      // docgen's type test says nothing: a function with no parameter (docgen
      // documents one only with a doc comment, having no props to show), or one
      // whose annotation is a union of a function and a class type (Next's
      // NextPage), which has no call signature of its own. Its props are
      // docgen's own reading of its first parameter.
      const p0 = (typeOf(sym, decl0).getCallSignatures()[0] || { getParameters: () => [] }).getParameters()[0];
      info = { props: p0 ? parser.getPropsInfo(p0, {}) : {} };
      added(p0 ? "a component whose annotation has no call signature (a union of a function and a class type, such as NextPage)" : "a component with no parameter (docgen documents one only with a doc comment)");
    }
    if (!info) continue;
    const decl = declOf(sym);
    if (!decl) {
      exclude("no function, class or variable declaration");
      continue;
    }
    const name = decl.name.text;
    if (!/^[A-Z]/.test(name)) {
      exclude("lower-case name (React reads a lower-case JSX tag as an HTML element)");
      continue;
    }
    if (isCreateContext(decl)) {
      exclude("a context object (createContext)");
      continue;
    }
    if (!renders(sym, decl)) {
      exclude("renders nothing (no call signature returning a React node, no render method)");
      continue;
    }
    const kind = ts.isClassDeclaration(decl) ? "class_component" : "component";
    const key = rel(decl.getSourceFile().fileName) + "#" + name;
    if (seenDef.has(key)) continue;
    seenDef.add(key);
    components.set(decl, name);
    defs.push({ path: rel(decl.getSourceFile().fileName), name, line: lineOf(decl.name), end: endOf(decl), kind });

    const pt = propsTypeOf(sym, decl);
    for (const pname of Object.keys(info.props).sort()) {
      const ps = pt && pt.getProperty(pname);
      const sigs = ((ps && ps.declarations) || []).filter((d) => inRepo(d.getSourceFile().fileName));
      if (!sigs.length) {
        exclude("prop declared outside the subject");
        continue;
      }
      const sig = sigs.find((d) => ts.isPropertySignature(d) || ts.isMethodSignature(d));
      if (!sig) {
        exclude("prop with no property or method signature (a mapped or inferred type)");
        continue;
      }
      const owner = ownerOf(sig) || name;
      const pkey = rel(sig.getSourceFile().fileName) + ":" + lineOf(sig) + "#" + owner + "." + pname;
      if (seenDef.has(pkey)) continue;
      seenDef.add(pkey);
      defs.push({ path: rel(sig.getSourceFile().fileName), name: owner + "." + pname, line: lineOf(sig.name), end: endOf(sig), kind: "prop" });
    }
  }
}

// Hooks: in-repo functions (a declaration, or a const bound to a function)
// named use[A-Z0-9]; React's naming rule decides, the checker resolves.
const HOOK = /^use[A-Z0-9]/;
function hookDecl(sym) {
  const d = declOf(sym || {});
  if (!d || !HOOK.test(d.name.text) || !inRepo(d.getSourceFile().fileName)) return null;
  if (ts.isFunctionDeclaration(d)) return d;
  if (ts.isVariableDeclaration(d) && d.initializer && (ts.isArrowFunction(d.initializer) || ts.isFunctionExpression(d.initializer))) return d;
  return null;
}
const hooks = [];
for (const sf of program.getSourceFiles()) {
  if (!files.includes(path.resolve(sf.fileName)) && !files.includes(sf.fileName)) continue;
  for (const st of sf.statements) {
    const ds = ts.isFunctionDeclaration(st) ? [st] : ts.isVariableStatement(st) ? st.declarationList.declarations : [];
    for (const d of ds) {
      const sym = d.name && ts.isIdentifier(d.name) ? checker.getSymbolAtLocation(d.name) : null;
      if (sym && hookDecl(sym) === d) hooks.push({ path: rel(sf.fileName), name: d.name.text, line: lineOf(d.name), end: endOf(d), kind: "hook" });
    }
  }
}
const reactTarget = (sym) => {
  const s = resolveAlias(sym);
  if (!s) return null;
  const d = declOf(s);
  if (d && components.has(d)) return d;
  return hookDecl(s);
};

// callerOf is the enclosing definition, named as the dialect names it: a
// function declaration at any depth, a class's method (Class.method), a class
// (its fields), or a module-scope variable (its initializer, whatever it is);
// and, since the dialects' module symbol (K18, LB-F381), the file's module,
// `<module>`, for code no definition encloses (a module-level render, a test
// callback's JSX).
function callerOf(node) {
  for (let p = node.parent; p; p = p.parent) {
    if (ts.isFunctionDeclaration(p) && p.name) return p.name.text;
    if ((ts.isMethodDeclaration(p) || ts.isGetAccessor(p) || ts.isSetAccessor(p)) && p.name && ts.isClassLike(p.parent)) {
      const cls = p.parent.name ? p.parent.name.text : "";
      return (cls ? cls + "." : "") + p.name.getText();
    }
    if (ts.isClassDeclaration(p) && p.name) return p.name.text;
    if (ts.isVariableDeclaration(p) && ts.isIdentifier(p.name) && ts.isVariableStatement(p.parent.parent) && ts.isSourceFile(p.parent.parent.parent)) return p.name.text;
  }
  return "<module>";
}

function inImportOrExport(n) {
  for (let p = n.parent; p; p = p.parent) {
    if (ts.isImportDeclaration(p) || ts.isExportDeclaration(p) || ts.isImportEqualsDeclaration(p)) return true;
    if (ts.isStatement(p)) return false;
  }
  return false;
}
function isDeclarationName(n) {
  const p = n.parent;
  return !!p && p.name === n && (ts.isFunctionDeclaration(p) || ts.isClassDeclaration(p) || ts.isVariableDeclaration(p) ||
    ts.isPropertySignature(p) || ts.isMethodSignature(p) || ts.isPropertyAssignment(p) || ts.isBindingElement(p));
}

const calls = new Map();
const fileEdges = new Map();
const imports = new Map();
// names are the (file, name as written) pairs at which a reference resolves
// to one of the components or hooks: the harness counts a fathom call pair
// only when its callee is a name the file binds to one (an imported Link from
// next/link, or a global Error, is not the subject's Link or Error).
const names = new Map();
function addName(sf, written) {
  const p = rel(sf.fileName);
  names.set(p + "\0" + written, [p, written]);
}
function addPair(node, target, written) {
  addName(node.getSourceFile(), written);
  const from = rel(node.getSourceFile().fileName);
  const to = rel(target.getSourceFile().fileName);
  const caller = callerOf(node);
  if (caller) {
    const k = from + "\0" + caller + "\0" + target.name.text;
    calls.set(k, { path: from, from: caller, to: target.name.text });
  }
  if (from !== to) fileEdges.set(from + "\0" + to, [from, to]);
}
for (const sf of program.getSourceFiles()) {
  if (!files.includes(path.resolve(sf.fileName)) && !files.includes(sf.fileName)) continue;
  (function visit(n) {
    if ((ts.isJsxOpeningElement(n) || ts.isJsxSelfClosingElement(n)) && !(ts.isIdentifier(n.tagName) && /^[a-z]/.test(n.tagName.text))) {
      const tag = ts.isPropertyAccessExpression(n.tagName) ? n.tagName.name : n.tagName;
      const t = reactTarget(checker.getSymbolAtLocation(tag));
      if (t) addPair(n, t, tag.text);
    } else if (ts.isCallExpression(n)) {
      const callee = ts.isPropertyAccessExpression(n.expression) ? n.expression.name : n.expression;
      const t = ts.isIdentifier(callee) && HOOK.test(callee.text) ? reactTarget(checker.getSymbolAtLocation(callee)) : null;
      if (t) addPair(n, t, callee.text);
    } else if (ts.isIdentifier(n) && !inImportOrExport(n) && !isDeclarationName(n)) {
      // Any other reference to a component or hook from another file is a
      // file pair (a component passed as a value, a story's `component:`).
      const t = reactTarget(checker.getSymbolAtLocation(n));
      if (t) addName(sf, n.text);
      if (t && t.getSourceFile() !== sf) {
        const from = rel(sf.fileName), to = rel(t.getSourceFile().fileName);
        fileEdges.set(from + "\0" + to, [from, to]);
      }
    } else if (ts.isImportDeclaration(n) && n.importClause && ts.isStringLiteral(n.moduleSpecifier)) {
      const res = ts.resolveModuleName(n.moduleSpecifier.text, sf.fileName, options, ts.sys).resolvedModule;
      if (res && inRepo(res.resolvedFileName) && !/\.d\.[mc]?ts$/.test(res.resolvedFileName)) {
        const names = [];
        if (n.importClause.name) names.push(n.importClause.name);
        const nb = n.importClause.namedBindings;
        if (nb && ts.isNamedImports(nb)) for (const el of nb.elements) names.push(el.name);
        if (names.some((id) => reactTarget(checker.getSymbolAtLocation(id)))) {
          const p = rel(sf.fileName);
          const line = lineOf(n);
          imports.set(p + "\0" + line, { path: p, line, to_path: rel(res.resolvedFileName), module: n.moduleSpecifier.text });
        }
      }
    }
    ts.forEachChild(n, visit);
  })(sf);
}

const sortBy = (xs, f) => xs.sort((a, b) => (f(a) < f(b) ? -1 : f(a) > f(b) ? 1 : 0));
const out = {
  tool: "react-docgen-typescript (Parser.getComponentInfo, Storybook's options, every prop kept) + the TypeScript checker",
  version: `react-docgen-typescript ${docgenVersion}, typescript ${ts.version}`,
  tier: "C",
  definitions: sortBy(defs, (d) => d.path + "\0" + String(d.line).padStart(6, "0") + "\0" + d.name),
  hooks: sortBy(hooks, (d) => d.path + "\0" + String(d.line).padStart(6, "0")),
  calls: sortBy([...calls.values()], (c) => c.path + "\0" + c.from + "\0" + c.to),
  file_edges: sortBy([...fileEdges.values()], (e) => e[0] + "\0" + e[1]),
  names: sortBy([...names.values()], (e) => e[0] + "\0" + e[1]),
  imports: sortBy([...imports.values()], (e) => e.path + "\0" + String(e.line).padStart(6, "0")),
  excluded: Object.fromEntries(Object.entries(excluded).sort()),
  added: Object.fromEntries(Object.entries(included).sort()),
  files: files.length,
};
fs.writeFileSync(args.out, JSON.stringify(out, null, 1) + "\n");
const n = (k) => out.definitions.filter((d) => d.kind === k).length;
console.error(`${files.length} files: ${n("component")} components, ${n("class_component")} class components, ${n("prop")} props, ${out.hooks.length} hooks, ${out.calls.length} pairs, ${out.file_edges.length} file edges, ${out.imports.length} imports; excluded ${JSON.stringify(out.excluded)}; added ${JSON.stringify(out.added)}`);
