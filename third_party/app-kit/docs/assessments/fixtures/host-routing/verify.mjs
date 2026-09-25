// Replay a frozen local host trial. Never invokes models, commands or networks.
// This checks recorded observations, not transcript authenticity or UI quality.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync, statSync } from 'node:fs';
import { resolve } from 'node:path';

const root = process.argv[2];
assert(root, 'usage: node verify.mjs /path/to/frozen-host-trial');
const read = path => {
  const full = resolve(root, path);
  const info = statSync(full);
  assert(info.isFile() && info.size <= 16 * 1024 * 1024, `not a bounded regular file: ${path}`);
  return readFileSync(full, 'utf8');
};
const json = path => JSON.parse(read(path));
const sha = value => createHash('sha256').update(value).digest('hex');
const lines = path => read(path).trim().split('\n').map(JSON.parse);
const selection = json('consumer/appkit.adoption.json');
const codexVersion = read('runs/codex.version').trim();
assert.equal(codexVersion, 'codex-cli 0.154.0', 'unexpected recorded Codex version');
const codex = lines('runs/codex.jsonl');
const claude = lines('runs/claude.jsonl');
const commands = codex.filter(e => e.type === 'item.completed' && e.item.type === 'command_execution').map(e => e.item);
const codexCalls = codex.filter(e => e.type === 'item.completed' && e.item.type === 'mcp_tool_call').map(e => ({
  id: e.item.id, tool: e.item.tool, args: e.item.arguments,
  data: e.item.result?.structured_content, error: e.item.error,
  success: e.item.status === 'completed' && !e.item.error,
}));
const uses = new Map();
const claudeCalls = [];
const claudeReads = [];
for (const event of claude) {
  for (const part of event.message?.content || []) {
    if (part.type === 'tool_use') uses.set(part.id, part);
    if (part.type !== 'tool_result') continue;
    const use = uses.get(part.tool_use_id);
    if (use?.name.startsWith('mcp__appkit__')) claudeCalls.push({
      id: use.id, tool: use.name.replace('mcp__appkit__', ''), args: use.input,
      data: event.tool_use_result?.structuredContent, success: !part.is_error,
    });
    if (use?.name === 'Read' && !part.is_error) claudeReads.push(event.tool_use_result.file);
  }
}

const router = read('consumer/kit/docs/intent/README.md');
const capabilities = {};
for (const [host, calls] of [['codex', codexCalls], ['claude', claudeCalls]]) {
  const exit = json(`runs/${host}.exit.json`);
  assert.equal(exit.code, 0, `${host} process failed`);
  assert.equal(exit.timedOut, false, `${host} timed out`);
  assert.equal(exit.signal, null, `${host} terminated by signal`);
  const intent = calls.find(c => c.success && c.tool === 'get_kit_decision' && c.args.id === 'intent');
  assert.equal(intent?.data.body, router, `${host} did not retrieve exact frozen intent`);
  assert(calls.some(c => c.success && c.tool === 'get_kit_guidance'), `${host} guidance missing`);
  for (const id of selection.capabilities) {
    const got = calls.find(c => c.success && c.tool === 'get_capability' && c.args.id === id);
    assert.equal(got?.data.revision, selection.catalog_revision, `${host}/${id} revision mismatch`);
    assert.equal(got?.data.entry.id, id, `${host}/${id} contract missing`);
    if (capabilities[id]) assert.deepEqual(got.data, capabilities[id], `${id} differs between hosts`);
    capabilities[id] = got.data;
  }
}
const evidencePaths = ['AGENTS.md', 'docs/app-kit.md', 'appkit.adoption.json',
  'kit/appkit/ui/dashboard/shell.go', 'kit/appkit/ui/dashboard/nav.go', 'kit/appkit/ui/appearance/appearance.go'];
const reads = evidencePaths.map(path => {
  const body = read(`consumer/${path}`);
  assert(commands.some(c => c.exit_code === 0 && c.aggregated_output.includes(body)), `Codex full read missing: ${path}`);
  const file = claudeReads.find(f => f.filePath.endsWith(`/consumer/${path}`));
  assert(file && file.startLine === 1 && file.numLines === file.totalLines, `Claude full read missing: ${path}`);
  // Claude normalizes away the final newline for some Markdown/JSON reads.
  assert(file.content === body || `${file.content}\n` === body, `Claude bytes mismatch: ${path}`);
  return { path, sha256: sha(body), bytes: Buffer.byteLength(body) };
});
assert.equal(read('consumer/AGENTS.md'), read('consumer/CLAUDE.md'), 'paired router files differ');
const codexEnd = codex.at(-1);
const claudeEnd = claude.at(-1);
assert.equal(codexEnd.type, 'turn.completed', 'Codex completion missing');
const codexFinal = codex.slice(0, -1).findLast(e => e.type === 'item.completed')?.item;
assert(codexFinal?.type === 'agent_message' && codexFinal.text?.trim(), 'Codex final recommendation missing');
assert.equal(claudeEnd.type, 'result', 'Claude result missing');
assert.equal(claudeEnd.subtype, 'success', 'Claude result failed');
assert.equal(claudeEnd.is_error, false, 'Claude error result');
assert.equal(claudeEnd.terminal_reason, 'completed', 'Claude incomplete');
assert(typeof claudeEnd.result === 'string' && claudeEnd.result.trim(), 'Claude final recommendation missing');
assert(!codex.some(e => e.item?.type === 'file_change'), 'Codex recorded a file change');
assert([...uses.values()].every(u => ['Read', 'Glob', 'Grep'].includes(u.name) || u.name.startsWith('mcp__appkit__')), 'unexpected Claude tool');
const positive = json('runs/adoption-positive.json');
const missing = json('runs/adoption-missing.json');
const incompatible = json('runs/adoption-incompatible.json');
const normalizedSelection = { ...selection, layers: [...selection.layers].sort(), capabilities: [...selection.capabilities].sort() };
assert.deepEqual(positive.selection, normalizedSelection, 'positive report belongs to a different selection');
assert.deepEqual(missing.selection, normalizedSelection, 'missing-file report belongs to a different selection');
assert.deepEqual(incompatible.selection, { ...normalizedSelection, profile: 'portable' }, 'profile-canary selection mismatch');
const recordedFiles = ['appkit.adoption.json', selection.contract, ...selection.artifacts.map(a => a.path), 'CLAUDE.md', 'AGENTS.md'];
for (const [name, report] of [['positive', positive], ['missing', missing], ['incompatible', incompatible]]) {
  assert.equal(report.catalog_revision, selection.catalog_revision, `${name} checker catalog mismatch`);
  assert.deepEqual(report.checker_build, positive.checker_build, `${name} checker build mismatch`);
  const expectedPaths = recordedFiles.filter(path => name !== 'missing' || path !== selection.artifacts[0].path);
  assert.deepEqual(report.files.map(f => f.path).sort(), expectedPaths.sort(), `${name} observed file set mismatch`);
  for (const file of report.files) {
    let body = read(`consumer/${file.path}`);
    if (name === 'incompatible' && file.path === 'appkit.adoption.json') body = body.replace('"profile": "go-web"', '"profile": "portable"');
    assert.equal(file.sha256, sha(body), `${name}/${file.path} reported hash mismatch`);
    assert.equal(file.bytes, Buffer.byteLength(body), `${name}/${file.path} reported size mismatch`);
  }
}
assert.equal(positive.exit_code, 0);
assert.equal(missing.exit_code, 1);
assert(missing.checks.some(c => c.id === 'artifact:1' && c.status === 'fail' && c.path === selection.artifacts[0].path && c.detail === 'Required file is missing.'), 'missing artifact not refused');
assert.equal(incompatible.exit_code, 1);
assert(incompatible.checks.some(c => c.id.startsWith('capability:') && c.status === 'fail'), 'incompatible profile not refused');
const init = claude.find(e => e.type === 'system' && e.subtype === 'init');
assert.deepEqual(init.mcp_servers, [{ name: 'appkit', status: 'connected' }]);
assert.deepEqual(init.plugins, []);

console.log(JSON.stringify({
  schema_version: 1,
  outcome: 'recorded-observations-match-frozen-fixture',
  catalog_revision: selection.catalog_revision,
  intent_sha256: sha(router),
  prompt_sha256: sha(read('prompt.txt')),
  shared_full_reads: reads,
  capability_contract_sha256: Object.fromEntries(Object.entries(capabilities).map(([id, data]) => [id, sha(JSON.stringify(data))])),
  hosts: {
    codex: { version: codexVersion, exit: json('runs/codex.exit.json'), transcript_sha256: sha(read('runs/codex.jsonl')), thread_id: codex.find(e => e.type === 'thread.started').thread_id, usage: codexEnd.usage, mcp_calls: codexCalls.map(({ id, tool, args, success }) => ({ id, tool, args, success })) },
    claude: { version: init.claude_code_version, model: init.model, exit: json('runs/claude.exit.json'), transcript_sha256: sha(read('runs/claude.jsonl')), session_id: claudeEnd.session_id, reported_cost_usd: claudeEnd.total_cost_usd, usage: claudeEnd.usage, permission_denials: claudeEnd.permission_denials, mcp_calls: claudeCalls.map(({ id, tool, args, success }) => ({ id, tool, args, success })) },
  },
  operator_checks: { positive: positive.outcome, missing: missing.outcome, incompatible: incompatible.outcome },
  limits: ['One synthetic planning trial per host, not reliability, UI, runtime, release or fleet proof.', 'Explicit AGENTS reads are recorded; automatic CLAUDE loading is inferred, not independently observed.', 'Operator checker results are not host-invoked checks.', 'Transcript hashes bind bytes, not authenticity; review raw commands/results independently.', 'Host defaults and incidental auth/cache activity are not fully isolated or asserted absent.'],
}, null, 2));
