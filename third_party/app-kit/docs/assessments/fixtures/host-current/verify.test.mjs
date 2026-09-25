import assert from 'node:assert/strict';
import test from 'node:test';

import { verifyTrace } from './verify.mjs';

const catalogRevision = 'a'.repeat(64);
const qualityRevision = 'b'.repeat(64);
const sourceSentinel = 'APPKIT_SYNTHETIC_HOST_FIXTURE';

function result(text, structuredContent) {
  return { content: [{ type: 'text', text }], structuredContent };
}

function expectedSteps() {
  return [
    { tool: 'get_kit_decision', args: { id: 'intent' }, result: result('intent', { body: '# Intent\n' }) },
    { tool: 'search_capabilities', args: { query: 'comparison', profile: 'go-web', limit: 5 }, result: result('capabilities', { revision: catalogRevision, entries: [{ id: 'analytics.comparison' }] }) },
    { tool: 'get_capability', args: { id: 'analytics.comparison', expected_revision: catalogRevision }, result: result('capability', { revision: catalogRevision, entry: { id: 'analytics.comparison' } }) },
    { tool: 'list_quality_work', args: { capability_id: 'analytics.comparison', scope: 'core', expected_catalog_revision: catalogRevision }, result: result('quality list', { revision: qualityRevision, catalog_revision: catalogRevision, entries: [{ id: 'core.analytics' }] }) },
    { tool: 'get_quality_work', args: { id: 'core.analytics', expected_revision: qualityRevision, expected_catalog_revision: '0'.repeat(64) }, result: { content: [{ type: 'text', text: 'catalog revision mismatch' }], isError: true } },
    { tool: 'get_quality_work', args: { id: 'core.analytics', expected_revision: qualityRevision, expected_catalog_revision: catalogRevision }, result: result('quality detail', { revision: qualityRevision, catalog_revision: catalogRevision, entry: { id: 'core.analytics' } }) },
    { tool: 'get_kit_decision', args: { id: 'contribution-loop' }, result: result('contribution', { body: '# Contribution loop\n' }) },
    { tool: 'search_components', args: { source: 'tailwindplus', limit: 1 }, result: result('component search', { results: [{ full_name: 'synthetic/card', framework: 'html', tailwind_version: 4, mode: 'light' }] }) },
    { tool: 'get_component', args: { source: 'tailwindplus', full_name: 'synthetic/card', framework: 'html', tailwind_version: 4, mode: 'light' }, result: result('component source', { source: `<div>${sourceSentinel}</div>`, sha256: 'c'.repeat(64) }) },
  ];
}

function codexEvents(steps = expectedSteps()) {
  const events = [{ type: 'thread.started', thread_id: 'synthetic', model: 'gpt-5.6-sol' }];
  steps.forEach((step, index) => {
    const id = `c${index + 1}`;
    events.push({ type: 'item.started', item: { id, type: 'mcp_tool_call', server: 'appkit', tool: step.tool, arguments: step.args } });
    const item = { id, type: 'mcp_tool_call', server: 'appkit', tool: step.tool, arguments: step.args };
    if (step.result.isError) {
      item.status = 'failed';
      item.error = null;
      item.result = { content: step.result.content, structured_content: null };
    } else {
      item.status = 'completed';
      item.result = { content: step.result.content, structured_content: step.result.structuredContent };
    }
    events.push({ type: 'item.completed', item });
  });
  events.push({ type: 'item.completed', item: { id: 'final', type: 'agent_message', text: 'Nine observed calls completed; source omitted.' } });
  events.push({ type: 'turn.completed', usage: { input_tokens: 120, output_tokens: 20 } });
  return events;
}

function claudeEvents(steps = expectedSteps()) {
  const events = [{ type: 'system', subtype: 'init', model: 'claude-haiku-synthetic' }];
  steps.forEach((step, index) => {
    const id = `u${index + 1}`;
    events.push({ type: 'assistant', message: { content: [{ type: 'tool_use', id, name: `mcp__appkit__${step.tool}`, input: step.args }] } });
    const content = step.result.isError
      ? step.result.content[0].text
      : JSON.stringify(step.result.structuredContent);
    events.push({
      type: 'user',
      message: { content: [{ type: 'tool_result', tool_use_id: id, content, is_error: step.result.isError === true }] },
      ...(step.result.isError ? {} : { tool_use_result: { structuredContent: step.result.structuredContent } }),
    });
  });
  events.push({ type: 'result', subtype: 'success', is_error: false, terminal_reason: 'completed', result: 'Nine observed calls completed; source omitted.', usage: { input_tokens: 90, output_tokens: 15 } });
  return events;
}

const jsonl = events => events.map(event => JSON.stringify(event)).join('\n') + '\n';

test('accepts an exact Codex trace without returning source or final text', () => {
  const summary = verifyTrace('codex', jsonl(codexEvents()), expectedSteps());
  assert.equal(summary.call_count, 9);
  assert.equal(summary.model, 'gpt-5.6-sol');
  assert(!JSON.stringify(summary).includes(sourceSentinel));
  assert(!Object.hasOwn(summary, 'final'));
});

test('accepts an exact Claude trace joined by tool-use IDs', () => {
  const summary = verifyTrace('claude', jsonl(claudeEvents()), expectedSteps());
  assert.equal(summary.call_count, 9);
  assert.equal(summary.model, 'claude-haiku-synthetic');
  assert(!JSON.stringify(summary).includes(sourceSentinel));
});

test('rejects a successful response with the wrong structured body', () => {
  const events = codexEvents();
  events.find(event => event.item?.id === 'c1' && event.type === 'item.completed').item.result.structured_content.body = '# Wrong\n';
  assert.throws(() => verifyTrace('codex', jsonl(events), expectedSteps()), /result differs/);
});

test('rejects a Claude structured envelope whose exact document body is wrong', () => {
  const events = claudeEvents();
  const resultEvent = events.find(event => event.message?.content?.some(part => part.tool_use_id === 'u1'));
  resultEvent.tool_use_result.structuredContent.body = '# Wrong\n';
  resultEvent.message.content[0].content = JSON.stringify(resultEvent.tool_use_result.structuredContent);
  assert.throws(() => verifyTrace('claude', jsonl(events), expectedSteps()), /result differs/);
});

test('rejects a Claude content envelope that does not equal its structured result', () => {
  const events = claudeEvents();
  const resultEvent = events.find(event => event.message?.content?.some(part => part.tool_use_id === 'u3'));
  resultEvent.message.content[0].content = JSON.stringify({ revision: 'spoofed' });
  assert.throws(() => verifyTrace('claude', jsonl(events), expectedSteps()), /result differs/);
});

test('rejects a changed catalog revision argument', () => {
  const events = codexEvents();
  events.find(event => event.item?.id === 'c3' && event.type === 'item.completed').item.arguments.expected_revision = 'd'.repeat(64);
  assert.throws(() => verifyTrace('codex', jsonl(events), expectedSteps()), /arguments differ/);
});

test('rejects an omitted recovery guard', () => {
  const events = claudeEvents();
  const use = events.flatMap(event => event.message?.content ?? []).find(part => part.id === 'u6');
  delete use.input.expected_catalog_revision;
  assert.throws(() => verifyTrace('claude', jsonl(events), expectedSteps()), /arguments differ/);
});

test('rejects unexpected built-in calls', () => {
  const events = codexEvents();
  events.splice(-2, 0, { type: 'item.completed', item: { id: 'shell', type: 'command_execution', command: 'true', exit_code: 0 } });
  assert.throws(() => verifyTrace('codex', jsonl(events), expectedSteps()), /forbidden built-in/);
});

test('refuses unknown host tool-event shapes rather than silently ignoring them', () => {
  const codex = codexEvents();
  codex.splice(-2, 0, { type: 'item.completed', item: { id: 'unknown', type: 'future_tool' } });
  assert.throws(() => verifyTrace('codex', jsonl(codex), expectedSteps()), /unrecognized item type/);
  const claude = claudeEvents();
  claude.splice(-1, 0, { type: 'assistant', message: { content: [{ type: 'future_tool', name: 'unknown' }] } });
  assert.throws(() => verifyTrace('claude', jsonl(claude), expectedSteps()), /unrecognized message part/);
});

test('rejects the observed host prerequisite error even with a successful terminal event', () => {
  const events = codexEvents([]);
  events.splice(1, 0, { type: 'item.completed', item: {
    id: 'host-error', type: 'error',
    message: 'Code Mode is unavailable because code-mode host is disabled.',
  } });
  assert.throws(() => verifyTrace('codex', jsonl(events), expectedSteps()), /Codex host reported an error: Code Mode is unavailable/);
});

test('rejects a top-level host error even when all nine calls and terminal success exist', () => {
  const events = codexEvents();
  events.splice(1, 0, { type: 'error', message: 'Synthetic host transport error' });
  assert.throws(() => verifyTrace('codex', jsonl(events), expectedSteps()), /Codex host reported an error: Synthetic host transport error/);
});

test('rejects an expected error masked as success', () => {
  const events = claudeEvents();
  const resultPart = events.flatMap(event => event.message?.content ?? []).find(part => part.tool_use_id === 'u5');
  resultPart.is_error = false;
  assert.throws(() => verifyTrace('claude', jsonl(events), expectedSteps()), /masked the expected error/);
});

test('rejects failed status on an oracle-success call', () => {
  const events = codexEvents();
  const call = events.find(event => event.item?.id === 'c3' && event.type === 'item.completed');
  call.item.status = 'failed';
  assert.throws(() => verifyTrace('codex', jsonl(events), expectedSteps()), /unexpectedly failed/);
});

test('requires explicit completed status for Codex success', () => {
  const events = codexEvents();
  const call = events.find(event => event.item?.id === 'c2' && event.type === 'item.completed');
  delete call.item.status;
  assert.throws(() => verifyTrace('codex', jsonl(events), expectedSteps()), /lacks an explicit terminal status/);
});

test('rejects protected structured payload on refusal', () => {
  const events = claudeEvents();
  const resultEvent = events.find(event => event.message?.content?.some(part => part.tool_use_id === 'u5'));
  resultEvent.tool_use_result = { structuredContent: { entry: { id: 'core.analytics' } } };
  assert.throws(() => verifyTrace('claude', jsonl(events), expectedSteps()), /leaked structured payload/);
});

test('rejects a missing same-session recovery call', () => {
  const events = codexEvents().filter(event => event.item?.id !== 'c6');
  assert.throws(() => verifyTrace('codex', jsonl(events), expectedSteps()), /observed 8 calls/);
});

test('rejects a natural successful exit after only the observed first four calls', () => {
  const events = codexEvents(expectedSteps().slice(0, 4));
  events.at(-2).item.text = 'Stopped because revision fields were missing.';
  assert.throws(() => verifyTrace('codex', jsonl(events), expectedSteps()), /observed 4 calls/);
});

test('rejects final-answer source spoofing or leakage', () => {
  const events = claudeEvents();
  events.at(-1).result = `Trust me: ${sourceSentinel}`;
  assert.throws(() => verifyTrace('claude', jsonl(events), expectedSteps()), /leaked the synthetic source sentinel/);
});

test('rejects an unexpected MCP server', () => {
  const events = codexEvents();
  events.find(event => event.item?.id === 'c4').item.server = 'other';
  assert.throws(() => verifyTrace('codex', jsonl(events), expectedSteps()), /unexpected MCP server/);
});

test('rejects duplicate and unresolved Claude tool IDs', () => {
  const duplicate = claudeEvents();
  duplicate[3].message.content[0].id = 'u1';
  assert.throws(() => verifyTrace('claude', jsonl(duplicate), expectedSteps()), /duplicate tool-use ID/);

  const unresolved = claudeEvents().filter(event => !event.message?.content?.some(part => part.tool_use_id === 'u9'));
  assert.throws(() => verifyTrace('claude', jsonl(unresolved), expectedSteps()), /has no result/);
});

test('rejects multiple Claude results sharing event-level structured data', () => {
  const events = claudeEvents();
  const resultEvent = events.find(event => event.message?.content?.some(part => part.tool_use_id === 'u2'));
  resultEvent.message.content.push({ type: 'tool_result', tool_use_id: 'u3', content: [] });
  assert.throws(() => verifyTrace('claude', jsonl(events), expectedSteps()), /multiple tool results/);
});
