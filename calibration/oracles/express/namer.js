'use strict'
// namer.js — names the function at a source position the way fathom's
// TS/JS dialect names its definition (internal/extractor/dialects/
// {javascript,typescript}/tags.scm), using the TypeScript compiler's own
// parser (Apache-2.0), independent of fathom's scanner:
//
//   - a function declaration: its name, at any depth;
//   - a class method: Class.method;
//   - a function or arrow bound by a variable declaration that is a direct
//     child of the module (or of an export statement): the variable;
//   - `x.y = function` / `x.prototype.y = function` / `a.b = a.c = function`
//     as a module-level statement: y (the assigned property);
//   - anything else (an inline argument, an object-literal member, a
//     nested binding, `module.exports.y = …`): "" — the dialect defines no
//     symbol for it.
//
// Usage from another script: require('./namer.js')(tsModulePath).name(file, line0, col0)

const fs = require('node:fs')
const path = require('node:path')

module.exports = function makeNamer (tsPath) {
  const ts = require(tsPath)
  const cache = new Map()

  function source (file) {
    if (!cache.has(file)) {
      const text = fs.readFileSync(file, 'utf8')
      const ext = path.extname(file)
      const kind = ext === '.tsx' ? ts.ScriptKind.TSX : ext === '.jsx' ? ts.ScriptKind.JSX
        : (ext === '.ts' || ext === '.mts' || ext === '.cts') ? ts.ScriptKind.TS : ts.ScriptKind.JS
      cache.set(file, ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true, kind))
    }
    return cache.get(file)
  }

  function isFunctionLike (n) {
    return ts.isFunctionDeclaration(n) || ts.isFunctionExpression(n) || ts.isArrowFunction(n) ||
      ts.isMethodDeclaration(n) || ts.isConstructorDeclaration(n) || ts.isGetAccessor(n) || ts.isSetAccessor(n)
  }

  // innermost function-like node whose span holds pos
  function functionAt (sf, pos) {
    let best = null
    function visit (n) {
      if (pos < n.getStart(sf) || pos >= n.end) return
      if (isFunctionLike(n)) best = n
      n.forEachChild(visit)
    }
    visit(sf)
    return best
  }

  function moduleLevel (stmt) {
    const p = stmt.parent
    return p && (ts.isSourceFile(p) || ts.isModuleBlock(p))
  }

  function nameOf (fn) {
    if (!fn) return ''
    if (ts.isFunctionDeclaration(fn)) return fn.name ? fn.name.text : ''
    if (ts.isMethodDeclaration(fn) || ts.isGetAccessor(fn) || ts.isSetAccessor(fn)) {
      const cls = fn.parent
      if ((ts.isClassDeclaration(cls) || ts.isClassExpression(cls)) && fn.name && ts.isIdentifier(fn.name)) {
        return cls.name ? cls.name.text + '.' + fn.name.text : fn.name.text
      }
      return ''
    }
    if (ts.isConstructorDeclaration(fn)) return ''
    let p = fn.parent
    while (p && (ts.isParenthesizedExpression(p) || ts.isAsExpression(p) || ts.isSatisfiesExpression?.(p))) p = p.parent
    if (p && ts.isVariableDeclaration(p) && ts.isIdentifier(p.name)) {
      const stmt = p.parent && p.parent.parent // VariableDeclarationList -> VariableStatement
      if (stmt && ts.isVariableStatement(stmt) && moduleLevel(stmt)) return p.name.text
      return ''
    }
    if (p && ts.isBinaryExpression(p) && p.operatorToken.kind === ts.SyntaxKind.EqualsToken && p.right !== undefined) {
      // a.b = a.c = function …: the innermost assignment names it
      let top = p
      while (top.parent && ts.isBinaryExpression(top.parent) && top.parent.operatorToken.kind === ts.SyntaxKind.EqualsToken) top = top.parent
      if (!top.parent || !ts.isExpressionStatement(top.parent) || !moduleLevel(top.parent)) return ''
      const left = p.left
      if (ts.isPropertyAccessExpression(left)) {
        const obj = left.expression
        if (ts.isIdentifier(obj)) return left.name.text
        if (ts.isPropertyAccessExpression(obj) && obj.name.text === 'prototype') return left.name.text
      }
      return ''
    }
    return ''
  }

  return {
    name (file, line0, col0) {
      const sf = source(file)
      const pos = ts.getPositionOfLineAndCharacter(sf, line0, col0)
      return nameOf(functionAt(sf, pos))
    }
  }
}
