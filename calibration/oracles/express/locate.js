'use strict'
// locate.js — where a live function object was written: V8's own
// [[FunctionLocation]] read through the inspector protocol in-process
// (node:inspector, no flags). A bound function is followed to its target.
//
//   const locate = require('./locate.js')()
//   locate(fn) // => { file: '/abs/path.js', line0, col0 } or null

const inspector = require('node:inspector')
const { fileURLToPath } = require('node:url')

module.exports = function makeLocator () {
  const session = new inspector.Session()
  session.connect()
  const scripts = new Map()
  session.on('Debugger.scriptParsed', (m) => { scripts.set(m.params.scriptId, m.params.url) })

  // In-process sessions answer synchronously.
  function post (method, params) {
    let err, res
    let done = false
    session.post(method, params || {}, (e, r) => { err = e; res = r; done = true })
    if (!done) throw new Error('inspector answered asynchronously: ' + method)
    if (err) throw err
    return res
  }
  post('Debugger.enable')

  function locateObject (objectId, depth) {
    const props = post('Runtime.getProperties', { objectId, ownProperties: true })
    const internal = props.internalProperties || []
    const target = internal.find((p) => p.name === '[[TargetFunction]]')
    if (target && target.value && target.value.objectId && depth < 4) return locateObject(target.value.objectId, depth + 1)
    const loc = internal.find((p) => p.name === '[[FunctionLocation]]')
    if (!loc) return null
    const { scriptId, lineNumber, columnNumber } = loc.value.value
    let url = scripts.get(scriptId) || ''
    if (url.startsWith('file://')) url = fileURLToPath(url)
    return { file: url, line0: lineNumber, col0: columnNumber }
  }

  return function locate (fn) {
    if (typeof fn !== 'function') return null
    globalThis.__fathomProbe = fn
    try {
      const { result } = post('Runtime.evaluate', { expression: 'globalThis.__fathomProbe' })
      return locateObject(result.objectId, 0)
    } finally {
      delete globalThis.__fathomProbe
    }
  }
}
