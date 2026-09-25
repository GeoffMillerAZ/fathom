// Offline verifier for frozen Codex/Claude JSONL host traces.
// It parses retained events only: no commands, inference, network, or configuration.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import { isDeepStrictEqual } from 'node:util';

const sentinel = 'APPKIT_SYNTHETIC_HOST_FIXTURE';
const zeroRevision = '0'.repeat(64);
const requiredTools = [
  'get_kit_decision',
  'search_capabilities',
  'get_capability',
  'list_quality_work',
  'get_quality_work',
  'get_quality_work',
  'get_kit_decision',
  'search_components',
  'get_component',
];

function fail(message) {
  throw new Error(`host-current verifier: ${message}`);
}

function parseJSONL(stdoutString) {
  if (typeof stdoutString !== 'string' || stdoutString.trim() === '') fail('trace is empty');
  return stdoutString.trim().split(/\r?\n/).map((line, index) => {
    try {
      return JSON.parse(line);
    } catch {
      fail(`trace line ${index + 1} is not JSON`);
    }
  });
}

function isRevision(value) {
  return typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
}

function exactKeys(value, keys, label) {
  assert.deepStrictEqual(Object.keys(value ?? {}).sort(), [...keys].sort(), `${label} arguments changed`);
}

function validateOracle(expectedSteps) {
  if (!Array.isArray(expectedSteps) || expectedSteps.length !== requiredTools.length) {
    fail(`oracle must contain exactly ${requiredTools.length} steps`);
  }
  for (let i = 0; i < expectedSteps.length; i++) {
    const step = expectedSteps[i];
    if (step?.tool !== requiredTools[i]) fail(`oracle step ${i + 1} must be ${requiredTools[i]}`);
    if (!step.args || typeof step.args !== 'object' || Array.isArray(step.args)) fail(`oracle step ${i + 1} lacks arguments`);
    if (!step.result || !Array.isArray(step.result.content)) fail(`oracle step ${i + 1} lacks an MCP result`);
  }

  exactKeys(expectedSteps[0].args, ['id'], 'intent');
  assert.equal(expectedSteps[0].args.id, 'intent');
  exactKeys(expectedSteps[1].args, ['query', 'profile', 'limit'], 'capability search');
  assert.deepStrictEqual(expectedSteps[1].args, { query: 'comparison', profile: 'go-web', limit: 5 });
  exactKeys(expectedSteps[2].args, ['id', 'expected_revision'], 'capability get');
  assert.equal(expectedSteps[2].args.id, 'analytics.comparison');
  const catalogRevision = expectedSteps[2].args.expected_revision;
  if (!isRevision(catalogRevision) || catalogRevision === zeroRevision) fail('oracle catalog guard is invalid');

  exactKeys(expectedSteps[3].args, ['capability_id', 'scope', 'expected_catalog_revision'], 'quality list');
  assert.deepStrictEqual(expectedSteps[3].args, {
    capability_id: 'analytics.comparison', scope: 'core', expected_catalog_revision: catalogRevision,
  });
  exactKeys(expectedSteps[4].args, ['id', 'expected_revision', 'expected_catalog_revision'], 'quality refusal');
  assert.equal(expectedSteps[4].args.id, 'core.analytics');
  assert.equal(expectedSteps[4].args.expected_catalog_revision, zeroRevision);
  if (!isRevision(expectedSteps[4].args.expected_revision) || expectedSteps[4].result.isError !== true) {
    fail('oracle quality refusal lacks its inventory guard or error result');
  }
  exactKeys(expectedSteps[5].args, ['id', 'expected_revision', 'expected_catalog_revision'], 'quality recovery');
  assert.deepStrictEqual(expectedSteps[5].args, {
    id: 'core.analytics',
    expected_revision: expectedSteps[4].args.expected_revision,
    expected_catalog_revision: catalogRevision,
  });
  if (expectedSteps[5].result.isError === true) fail('oracle quality recovery is an error');
  exactKeys(expectedSteps[6].args, ['id'], 'contribution loop');
  assert.equal(expectedSteps[6].args.id, 'contribution-loop');
  exactKeys(expectedSteps[7].args, ['source', 'limit'], 'component search');
  assert.deepStrictEqual(expectedSteps[7].args, { source: 'tailwindplus', limit: 1 });
  exactKeys(expectedSteps[8].args, ['source', 'full_name', 'framework', 'tailwind_version', 'mode'], 'component get');
  assert.equal(expectedSteps[8].args.source, 'tailwindplus');
  for (const key of ['full_name', 'framework', 'mode']) {
    if (typeof expectedSteps[8].args[key] !== 'string' || expectedSteps[8].args[key] === '') fail(`oracle component ${key} is empty`);
  }
  if (!Number.isInteger(expectedSteps[8].args.tailwind_version) || expectedSteps[8].args.tailwind_version <= 0) {
    fail('oracle component tailwind_version is invalid');
  }
}

function normalizeContent(content) {
  if (content === undefined || content === null) return [];
  if (Array.isArray(content)) return content;
  if (typeof content === 'string') return [{ type: 'text', text: content }];
  fail('tool result content is neither an array nor text');
}

function errorText(call) {
  const content = call.result.content
    .filter(block => block?.type === 'text' && typeof block.text === 'string')
    .map(block => block.text);
  if (typeof call.hostError === 'string') content.push(call.hostError);
  else if (call.hostError) content.push(JSON.stringify(call.hostError));
  return content.join('\n');
}

function hasProtectedPayload(value) {
  if (value === undefined || value === null) return false;
  if (Array.isArray(value)) return value.length > 0;
  if (typeof value === 'object') return Object.keys(value).length > 0;
  return true;
}

function verifyCalls(calls, expectedSteps) {
  if (calls.length !== expectedSteps.length) fail(`observed ${calls.length} calls, want ${expectedSteps.length}`);
  return calls.map((call, index) => {
    const expected = expectedSteps[index];
    if (call.tool !== expected.tool) fail(`call ${index + 1} is ${call.tool}, want ${expected.tool}`);
    try {
      assert.deepStrictEqual(call.args, expected.args);
    } catch {
      fail(`call ${index + 1} ${call.tool} arguments differ from the oracle`);
    }

    const expectedError = expected.result.isError === true;
    if (expectedError) {
      if (!call.result.isError) fail(`call ${index + 1} masked the expected error`);
      if (hasProtectedPayload(call.result.structuredContent)) fail(`call ${index + 1} error leaked structured payload`);
      const observed = errorText(call);
      const expectedTexts = expected.result.content
        .filter(block => block?.type === 'text' && typeof block.text === 'string')
        .map(block => block.text);
      if (expectedTexts.length === 0 || expectedTexts.some(text => !observed.includes(text))) {
        fail(`call ${index + 1} error content differs from the oracle`);
      }
    } else {
      if (call.result.isError) fail(`call ${index + 1} unexpectedly failed`);
      try {
        assert.deepStrictEqual(call.result.structuredContent, expected.result.structuredContent);
        // Claude stream-json replaces the SDK content blocks with a JSON string
        // of structuredContent. Accept that envelope only after proving it is an
        // exact duplicate; the structured result still must exactly match the
        // SDK oracle, including document bodies and synthetic component source.
        if (!call.result.contentIsStructuredEnvelope) {
          assert.deepStrictEqual(call.result.content, expected.result.content);
        } else if (expected.result.structuredContent === undefined) {
          throw new Error('structured envelope has no oracle');
        }
      } catch {
        fail(`call ${index + 1} ${call.tool} result differs from the oracle`);
      }
    }
    return { tool: call.tool, args: call.args, outcome: expectedError ? 'refused' : 'passed' };
  });
}

function codexTrace(events) {
  const calls = [];
  const started = new Set();
  const completed = new Set();
  const forbidden = /(command|file_change|web|browser|computer|delegat|collaboration)/i;
  for (const event of events) {
    if (event?.type === 'error') fail(`Codex host reported an error: ${event.message ?? '<no diagnostic>'}`);
    const item = event?.item;
    if (!item) continue;
    if (item.type === 'error') fail(`Codex host reported an error: ${item.message ?? '<no diagnostic>'}`);
    if (forbidden.test(String(item.type ?? ''))) fail(`Codex used forbidden built-in event ${item.type}`);
    if (!['mcp_tool_call', 'agent_message', 'reasoning'].includes(item.type)) fail(`Codex unrecognized item type ${item.type}`);
    if (item.type !== 'mcp_tool_call') continue;
    if (item.server !== 'appkit') fail(`Codex called unexpected MCP server ${item.server ?? '<missing>'}`);
    if (event.type === 'item.started') {
      if (!item.id || started.has(item.id)) fail('Codex has a duplicate MCP start ID');
      started.add(item.id);
      continue;
    }
    if (event.type !== 'item.completed') continue;
    if (!item.id || completed.has(item.id)) fail('Codex has a duplicate MCP completion ID');
    if (!['completed', 'failed'].includes(item.status)) fail(`Codex MCP ID ${item.id} lacks an explicit terminal status`);
    completed.add(item.id);
    const result = item.result ?? {};
    calls.push({
      id: item.id,
      tool: item.tool,
      args: item.arguments,
      result: {
        content: normalizeContent(result.content),
        structuredContent: result.structured_content ?? result.structuredContent,
        isError: item.status === 'failed' || Boolean(item.error || result.is_error || result.isError),
      },
      hostError: item.error,
    });
  }
  for (const id of started) if (!completed.has(id)) fail(`Codex MCP ID ${id} has no completion`);
  if (started.size > 0) for (const id of completed) if (!started.has(id)) fail(`Codex MCP ID ${id} has no start`);

  const terminal = events.at(-1);
  if (terminal?.type !== 'turn.completed') fail('Codex terminal success event is missing');
  const final = events.slice(0, -1).findLast(event =>
    event?.type === 'item.completed' && event.item?.type === 'agent_message');
  if (typeof final?.item?.text !== 'string' || final.item.text.trim() === '') fail('Codex final message is missing');
  if (final.item.text.includes(sentinel)) fail('Codex final message leaked the synthetic source sentinel');
  const modelEvent = events.find(event =>
    typeof event?.model === 'string' && ['thread.started', 'turn.started', 'turn.completed'].includes(event.type));
  return { calls, usage: terminal.usage, model: modelEvent?.model };
}

function claudeTrace(events) {
  const uses = new Map();
  const order = [];
  const results = new Map();
  for (const event of events) {
    const parts = event?.message?.content ?? [];
    const resultParts = parts.filter(part => part?.type === 'tool_result');
    if (resultParts.length > 1) fail('Claude emitted multiple tool results in one event');
    for (const part of parts) {
      if (!['text', 'thinking', 'redacted_thinking', 'tool_use', 'tool_result'].includes(part?.type)) {
        fail(`Claude unrecognized message part ${part?.type}`);
      }
      if (part?.type === 'tool_use') {
        if (typeof part.id !== 'string' || uses.has(part.id)) fail('Claude has a duplicate tool-use ID');
        if (typeof part.name !== 'string' || !part.name.startsWith('mcp__appkit__')) {
          fail(`Claude used non-appkit tool ${part.name ?? '<missing>'}`);
        }
        uses.set(part.id, part);
        order.push(part.id);
      }
      if (part?.type === 'tool_result') {
        if (typeof part.tool_use_id !== 'string' || !uses.has(part.tool_use_id)) fail('Claude result has no matching tool use');
        if (results.has(part.tool_use_id)) fail('Claude has a duplicate tool result');
        const structuredContent = event.tool_use_result?.structuredContent;
        let contentIsStructuredEnvelope = false;
        if (typeof part.content === 'string' && structuredContent !== undefined) {
          try {
            contentIsStructuredEnvelope = isDeepStrictEqual(JSON.parse(part.content), structuredContent);
          } catch {
            // Plain-text MCP errors and non-JSON content remain ordinary text.
          }
        }
        results.set(part.tool_use_id, {
          content: normalizeContent(part.content),
          structuredContent,
          contentIsStructuredEnvelope,
          isError: part.is_error === true,
        });
      }
    }
  }
  for (const id of order) if (!results.has(id)) fail(`Claude tool-use ID ${id} has no result`);
  const calls = order.map(id => {
    const use = uses.get(id);
    return { id, tool: use.name.slice('mcp__appkit__'.length), args: use.input, result: results.get(id) };
  });

  const terminal = events.at(-1);
  if (terminal?.type !== 'result' || terminal.subtype !== 'success' || terminal.is_error !== false || terminal.terminal_reason !== 'completed') {
    fail('Claude terminal success event is missing');
  }
  if (typeof terminal.result !== 'string' || terminal.result.trim() === '') fail('Claude final message is missing');
  if (terminal.result.includes(sentinel)) fail('Claude final message leaked the synthetic source sentinel');
  const init = events.find(event => event?.type === 'system' && event.subtype === 'init');
  return { calls, usage: terminal.usage, model: typeof init?.model === 'string' ? init.model : undefined };
}

export function verifyTrace(host, stdoutString, expectedSteps) {
  if (!['codex', 'claude'].includes(host)) fail('host must be codex or claude');
  validateOracle(expectedSteps);
  const events = parseJSONL(stdoutString);
  const observed = host === 'codex' ? codexTrace(events) : claudeTrace(events);
  const calls = verifyCalls(observed.calls, expectedSteps);
  return {
    schema_version: 1,
    host,
    outcome: 'observed-trace-matches-oracle',
    call_count: calls.length,
    calls,
    ...(observed.model ? { model: observed.model } : {}),
    ...(observed.usage !== undefined ? { usage: observed.usage } : {}),
  };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  if (process.argv.length !== 5) fail('usage: node verify.mjs HOST TRACE EXPECTED');
  const [, , host, tracePath, expectedPath] = process.argv;
  const expected = JSON.parse(readFileSync(expectedPath, 'utf8'));
  console.log(JSON.stringify(verifyTrace(host, readFileSync(tracePath, 'utf8'), expected), null, 2));
}
