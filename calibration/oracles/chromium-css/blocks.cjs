'use strict';

// Lexical proposals only. Chromium's CSS.setStyleTexts decides whether a
// proposed body is a native CSSStyle range; this scanner proves no end line.
function lineStarts(source) {
  const starts = [0];
  for (let i = 0; i < source.length; i++) if (source[i] === '\n') starts.push(i + 1);
  return starts;
}

function lineOf(starts, offset) {
  let low = 0, high = starts.length - 1;
  while (low < high) {
    const mid = (low + high + 1) >>> 1;
    if (starts[mid] <= offset) low = mid;
    else high = mid - 1;
  }
  return low + 1;
}

function point(starts, offset) {
  const line = lineOf(starts, offset) - 1;
  return { line, column: offset - starts[line] };
}

function skipString(source, start) {
  const quote = source[start];
  for (let i = start + 1; i < source.length; i++) {
    if (source[i] === '\\') { i++; continue; }
    if (source[i] === quote || source[i] === '\n') return i + 1;
  }
  return source.length;
}

function skipComment(source, start) {
  const end = source.indexOf('*/', start + 2);
  return end < 0 ? source.length : end + 2;
}

function space(char) { return char === ' ' || char === '\t' || char === '\n' || char === '\r' || char === '\f'; }

function trimmed(source, begin, end) {
  for (;;) {
    while (begin < end && space(source[begin])) begin++;
    if (source[begin] === '/' && source[begin + 1] === '*') {
      begin = Math.min(end, skipComment(source, begin));
      continue;
    }
    break;
  }
  for (;;) {
    while (end > begin && space(source[end - 1])) end--;
    if (end - 2 >= begin && source[end - 2] === '*' && source[end - 1] === '/') {
      const open = source.lastIndexOf('/*', end - 2);
      if (open >= begin) { end = open; continue; }
    }
    break;
  }
  return [begin, end];
}

function headerParts(source, begin, end, starts) {
  const parts = [];
  let start = begin, depth = 0;
  const add = stop => {
    const [a, b] = trimmed(source, start, stop);
    if (a < b) parts.push({ text: source.slice(a, b), line: lineOf(starts, a) });
  };
  for (let i = begin; i < end;) {
    const c = source[i];
    if (c === '"' || c === "'") { i = skipString(source, i); continue; }
    if (c === '/' && source[i + 1] === '*') { i = skipComment(source, i); continue; }
    if (c === '\\') { i += 2; continue; }
    if (c === '(' || c === '[') depth++;
    else if (c === ')' || c === ']') depth = Math.max(0, depth - 1);
    else if (c === ',' && depth === 0) { add(i); start = i + 1; }
    i++;
  }
  add(end);
  return parts;
}

function scanBlocks(source) {
  const starts = lineStarts(source);
  const root = { next: 0 };
  const stack = [root];
  const found = [];
  let depth = 0;
  for (let i = 0; i < source.length;) {
    const c = source[i];
    if (c === '"' || c === "'") { i = skipString(source, i); continue; }
    if (c === '/' && source[i + 1] === '*') { i = skipComment(source, i); continue; }
    if (c === '\\') { i += 2; continue; }
    if (c === '(' || c === '[') { depth++; i++; continue; }
    if (c === ')' || c === ']') { depth = Math.max(0, depth - 1); i++; continue; }
    if (depth === 0) {
      const context = stack[stack.length - 1];
      if (c === '{') {
        if (context.block) context.block.hasNested = true;
        const block = { open: i, close: -1, parts: headerParts(source, context.next, i, starts), hasNested: false };
        found.push(block);
        stack.push({ block, next: i + 1 });
      } else if (c === '}') {
        if (stack.length > 1) { stack.pop().block.close = i; stack[stack.length - 1].next = i + 1; }
      } else if (c === ';') context.next = i + 1;
    }
    i++;
  }
  return found.filter(block => block.close >= 0);
}

module.exports = { scanBlocks, lineStarts, lineOf, point };
