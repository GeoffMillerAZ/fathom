import { scenarios, viewNames } from './scenarios.js';

const root = document.getElementById('app');
const modes = ['inline', 'panel', 'split', 'full'];
const params = new URLSearchParams(location.search);
const initial = scenarios.find(s => s.id === params.get('scenario')) || scenarios[0];
const sourceKey = source => source && `${source.id}@${source.version}`;
function sourceFor(logicalId) {
  const matches = state.scenario.sources.filter(source => source.id === logicalId);
  return matches.at(-1) || state.scenario.sources[0];
}
const state = {
  scenario: initial, mode: modes.includes(params.get('mode')) ? params.get('mode') : 'full',
  view: viewNames.includes(params.get('view')) ? params.get('view') : initial.view,
  role: 'Reviewer', theme: 'Light', tab: 'Evidence', selected: initial.queue[0]?.id || null,
  source: sourceKey(initial.sources[0]) || null, filter: 'All', level: 'sentence', variant: 'Executive',
  audience: 'Executive', voice: 'Clear and direct', annotation: '',
  compare: false, corrected: false, rebuilt: false, reviewFresh: false, approvals: new Set(),
  checks: { purpose: false, destination: false, expiry: false }, destination: 'synthetic inbox A',
  runStep: 0, runStopped: false, allowance: 3, delivery: 'Unknown', absentDecision: false,
  clarification: 'Unknown', answer: '', lessonStep: 0, announcement: '', dialogReturn: null,
};

function node(tag, attrs = {}, ...children) {
  const element = document.createElement(tag);
  for (const [name, value] of Object.entries(attrs)) {
    if (value == null || value === false) continue;
    if (name === 'class') element.className = value;
    else if (name === 'text') element.textContent = value;
    else if (name === 'value') element.value = value;
    else if (name === 'checked') element.checked = Boolean(value);
    else if (name === 'disabled') element.disabled = Boolean(value);
    else if (name === 'onClick') element.addEventListener('click', value);
    else if (name === 'onChange') element.addEventListener('change', value);
    else if (name === 'onInput') element.addEventListener('input', value);
    else element.setAttribute(name, String(value));
  }
  for (const child of children.flat()) {
    if (child == null) continue;
    element.append(child instanceof Node ? child : document.createTextNode(String(child)));
  }
  return element;
}
const h = (tag, cls, text) => node(tag, { class: cls, text });
const label = text => h('span', 'section-label', text);
const badge = (text, status = text.toLowerCase()) => node('span', { class: 'badge', 'data-status': status, text });
const button = (text, action, kind = 'subtle', disabled = false, extra = {}) =>
  node('button', { type: 'button', class: `button ${kind}`, disabled, onClick: action, ...extra }, text);
const notice = text => h('p', 'notice', text);
function announce(text) { state.announcement = text; render(); }
function url() {
  const query = new URLSearchParams(location.search);
  query.set('scenario', state.scenario.id); query.set('mode', state.mode); query.set('view', state.view);
  history.replaceState(null, '', `${location.pathname}?${query}${location.hash}`);
}
function changeScenario(id) {
  state.scenario = scenarios.find(s => s.id === id) || scenarios[0];
  state.view = state.scenario.view; state.selected = state.scenario.queue[0]?.id || null;
  state.source = sourceKey(state.scenario.sources[0]) || null; state.tab = 'Evidence'; state.filter = 'All';
  state.compare = false; state.corrected = false; state.rebuilt = false; state.reviewFresh = false;
  state.approvals = new Set(); state.checks = { purpose: false, destination: false, expiry: false };
  state.destination = 'synthetic inbox A'; state.runStep = 0; state.runStopped = false; state.allowance = 3;
  state.delivery = 'Unknown'; state.absentDecision = false; state.clarification = 'Unknown';
  state.answer = ''; state.lessonStep = 0; state.annotation = ''; state.announcement = 'Scenario changed. Previous selection and decisions cleared.';
  url(); render();
}
function selectField(title, value, values, change) {
  const select = node('select', { 'aria-label': title, 'data-key': `select:${title}`, onChange: e => change(e.target.value) },
    values.map(v => node('option', { value: typeof v === 'string' ? v : v.id, text: typeof v === 'string' ? v : v.title })));
  select.value = value;
  return node('label', { class: 'control-field' }, h('span', '', title), select);
}
function openDialog(title, content, trigger, afterClose) {
  const dialog = node('dialog', { 'aria-label': title, class: 'prototype-dialog' });
  dialog.append(h('h2', '', title), content, button('Close', () => dialog.close()));
  dialog.addEventListener('close', () => { dialog.remove(); trigger?.focus(); afterClose?.(); });
  root.append(dialog); dialog.showModal();
}
function toolbar() {
  const bar = node('div', { class: 'prototype-bar' });
  bar.append(node('div', { class: 'prototype-label' },
    h('strong', '', 'BACKENDLESS PROTOTYPE'), h('span', '', 'Synthetic data / local simulation')));
  const controls = node('div', { class: 'prototype-controls' });
  controls.append(
    selectField('Scenario', state.scenario.id, scenarios, changeScenario),
    selectField('Presentation', state.mode, modes, v => { state.mode = v; url(); render(); }),
    selectField('Simulated role', state.role, ['Reviewer', 'Reader'], v => { state.role = v; announce(`${v} presentation preview.`); }),
    selectField('Theme', state.theme, ['Light', 'Dark'], v => { state.theme = v; render(); }),
    button('About', e => openDialog('About this prototype', node('div', {},
      h('p', '', 'Explore how a shared workbench presents review, prose, evidence, runs, catalog, and learning decisions.'),
      h('p', '', 'There is no backend, access control, delivery, provider call, or production workflow. All controls change local simulation state only.'),
      h('p', '', 'Client fixtures are public within this demo. Redaction is illustrative and contains no real secret. Reader is a presentation preview, not actual RBAC.')
    ), e.currentTarget)));
  bar.append(controls); return bar;
}
function nav() {
  return node('nav', { class: 'app-nav', 'aria-label': 'Workbench views' },
    node('div', { class: 'nav-brand' }, h('strong', '', 'Governed workbench'), h('small', '', 'Design study')),
    viewNames.map(v =>
    button(v[0].toUpperCase() + v.slice(1), () => { state.view = v; url(); render(); }, 'nav-item', state.view === v,
      { 'aria-current': state.view === v ? 'page' : null })),
    h('small', 'nav-foot', 'Backendless · synthetic'));
}
function contextStrip() {
  const s = state.scenario;
  return node('div', { class: 'context-strip', 'aria-label': 'Current task context' },
    ...[
      ['Revision', s.revision], ['Sources', String(s.sources.length)],
      ['Governing criteria', String(s.criteria.length)],
      ['Task view', state.view[0].toUpperCase() + state.view.slice(1)],
    ].map(([name, value]) => node('div', { class: 'context-stat' }, h('span', 'section-label', name), h('strong', '', value))));
}
function queue() {
  const records = state.scenario.queue.map(r => ({ ...r, state: state.approvals.has(r.id) ? 'Approved' : r.state }));
  const filtered = state.filter === 'All' ? records : records.filter(r => r.state === state.filter);
  const statuses = [...new Set(records.map(r => r.state))];
  const section = node('aside', { class: 'queue', 'aria-label': 'Ordered queue' });
  section.append(h('h2', '', `Queue · ${filtered.length}/${records.length}`),
    selectField('Filter', state.filter, ['All', ...statuses], v => { state.filter = v; render(); }));
  if (!filtered.length) section.append(h('p', 'empty-state', 'No records match this filter.'));
  filtered.forEach(r => section.append(button(`${records.indexOf(r) + 1}. ${r.title} · ${r.state}`,
    () => selectRecord(r.id),
    `queue-item ${state.selected === r.id ? 'selected' : ''}`, false,
    { 'aria-current': state.selected === r.id ? 'true' : null, 'data-status': r.state.toLowerCase(), 'data-key': `queue:${r.id}` })));
  return section;
}
function relatedSource(id) { return state.scenario.nodes?.find(n => n.id === id)?.sources?.[0]; }
function selectRecord(id) {
  state.selected = id;
  state.source = sourceKey(sourceFor(relatedSource(id))) || null;
  state.announcement = `Selected ${state.scenario.queue.find(r => r.id === id)?.title || id}.`;
  render();
}
function inspector() {
  const aside = node('aside', { class: 'inspector', 'aria-label': 'Evidence, criteria, and history inspector' });
  const tabs = node('div', { class: 'tabs', role: 'group', 'aria-label': 'Inspector sections' },
    ['Evidence', 'Criteria', 'History'].map(t => button(t, () => { state.tab = t; render(); },
      'tab', false, { 'aria-pressed': state.tab === t ? 'true' : 'false', 'data-key': `inspector:${t}` })));
  aside.append(tabs);
  const body = node('div', { class: 'inspector-body' });
  const scenario = state.scenario;
  if (state.tab === 'Evidence') {
    body.append(notice(`Purpose: ${scenario.id === 'delivery-unknown' ? 'approved summary delivery' : 'synthetic task review'}. Destination: ${state.destination}.`));
    for (const s of scenario.sources) body.append(button(`${s.id} ${s.version} · ${s.state}`, () => { state.source = sourceKey(s); render(); },
      `evidence-card ${state.source === sourceKey(s) ? 'selected' : ''}`, false,
      { 'aria-pressed': state.source === sourceKey(s) ? 'true' : 'false', 'data-status': s.state, 'data-key': `source:${sourceKey(s)}` }));
    const selected = scenario.sources.find(s => sourceKey(s) === state.source);
    if (selected) body.append(node('div', { class: 'panel' }, h('strong', '', `${selected.id} ${selected.version}`),
      h('p', '', selected.detail), badge(selected.state)));
    if (state.rebuilt && scenario.id === 'prose-conflict') body.append(notice('Local adjudication uses SRC-17 v3 for the rebuilt draft. Original source versions remain unchanged.'));
  } else if (state.tab === 'Criteria') {
    scenario.criteria.forEach(c => {
      const condition = ['CR-4', 'APP-2'].includes(c.id) && state.reviewFresh ? 'met' : c.state;
      body.append(node('div', { class: 'panel', 'data-status': condition },
        h('strong', '', c.id), badge(condition), h('p', '', c.detail)));
    });
    if (scenario.criteria.some(c => c.state === 'warning')) body.append(notice('Style warning is advisory. Policy block remains separate.'));
  } else {
    const entries = [...scenario.history, ...localHistory()];
    body.append(node('ol', { class: 'timeline' }, entries.map(x => h('li', '', x))));
  }
  aside.append(body); return aside;
}
function localHistory() {
  const id = state.scenario.id, rows = [];
  if (state.corrected) rows.push('Local source correction staged');
  if (state.rebuilt) rows.push('Affected summary and variants rebuilt; unrelated nodes retained');
  if (state.reviewFresh) rows.push('Fresh review recorded for rebuilt revision');
  if (state.approvals.size) rows.push(`Local decisions: ${[...state.approvals].join(', ')}`);
  if (id === 'investigation-ladder' && state.runStep) rows.push(`Investigation step ${state.runStep}; allowance ${state.allowance}`);
  if (id === 'delivery-unknown') rows.push(`Receipt query result: ${state.delivery}`);
  if (id === 'clarification') rows.push(`Answer status: ${state.clarification}${state.answer ? ' · text retained' : ''}`);
  if (id === 'learning-flywheel' && state.lessonStep) rows.push(`Lesson stage ${state.lessonStep}/4; policy unchanged`);
  return rows;
}
function selectedRecord() {
  const record = state.scenario.queue.find(r => r.id === state.selected) || state.scenario.queue[0];
  return record && { ...record, state: state.approvals.has(record.id) ? 'Approved' : record.state };
}
function approvalButton(id, text = 'Simulate approval') {
  const blocked = state.role === 'Reader' || selectedRecord()?.state === 'Blocked' || state.approvals.has(id);
  return button(state.approvals.has(id) ? 'Approved in local simulation' : text,
    () => { state.approvals.add(id); announce(`${id} approved in local simulation.`); }, 'primary', blocked);
}
function proseView() {
  const s = state.scenario;
  const panel = node('section', { class: 'content' });
  const toolbar = node('div', { class: 'toolbar' });
  toolbar.append(selectField('Selection level', state.level, ['sentence', 'paragraph', 'section', 'document'], v => { state.level = v; render(); }),
    button(state.compare ? 'Hide version comparison' : 'Compare source versions', () => { state.compare = !state.compare; render(); }, 'subtle', !['prose-conflict', 'stale-approval'].includes(s.id)));
  toolbar.append(selectField('Voice profile', state.voice, ['Clear and direct', 'Formal'], v => { state.voice = v; announce(`Voice profile selected: ${v}. This mock does not regenerate wording.`); }));
  if (s.variants) toolbar.append(selectField('Audience', state.variant, Object.keys(s.variants), v => { state.variant = v; render(); }));
  else toolbar.append(selectField('Audience profile', state.audience, ['Executive', 'Technical'], v => { state.audience = v; announce(`Audience profile selected: ${v}. This mock does not regenerate wording.`); }));
  toolbar.append(button('Annotate source presentation', e => openDialog('Source presentation note',
    node('div', {}, notice('Local note about how the source is shown. It cannot correct a fact or change policy.'),
      node('label', { class: 'control-field' }, 'Presentation note', node('textarea', {
        rows: '4', 'aria-label': 'Presentation note', onInput: event => { state.annotation = event.target.value; },
      }, state.annotation))), e.currentTarget,
    () => announce('Presentation note stored in this page session only.')), 'subtle'));
  panel.append(toolbar);
  const nodes = s.nodes || [];
  const affectedIds = s.id === 'prose-conflict' ? ['claim', 'caveat'] : ['affected'];
  const outlineNames = { opening: 'Opening', claim: 'Key fact', affected: 'Key fact', keyfact: 'Key fact',
    context: 'Context', scope: 'Scope', method: 'Method', caveat: 'Caveat', nextstep: 'Next step', unrelated: 'Unrelated scope' };
  const selectedNode = nodes.find(n => n.id === state.selected) || nodes[0];
  const selectNode = n => { state.selected = n.id; state.source = sourceKey(sourceFor(n.sources[0])); render(); };
  const outline = node('nav', { class: 'document-outline', 'aria-label': 'Document outline' },
    label('Document outline'), nodes.map(n => button(`${n.section || 'Draft'} · ${outlineNames[n.id] || n.id}`, () => selectNode(n),
      'subtle', false, { 'aria-current': state.selected === n.id ? 'location' : null, 'data-key': `outline:${n.id}` })));
  panel.append(outline);
  const page = node('article', { class: 'prose-page', contenteditable: 'false', 'data-voice': state.voice.toLowerCase().replaceAll(' ', '-') },
    h('p', 'section-label', `${s.revision} · ${state.level} selection · ${state.voice} · ${s.variants ? state.variant : state.audience}`),
    h('h2', 'prose-heading', s.title));
  let sectionName = null, section = null;
  const visibleNodes = s.variants ? nodes.filter(n => s.variants[state.variant].includes(n.id)) : nodes;
  visibleNodes.forEach(n => {
    if (n.section !== sectionName) {
      sectionName = n.section;
      section = node('section', { class: 'prose-section' }, h('h3', '', sectionName || 'Draft'));
      page.append(section);
    }
    const selected = state.level === 'document' ||
      (state.level === 'section' && n.section === selectedNode?.section) || state.selected === n.id;
    section.append(button(state.rebuilt && n.id === 'claim' ? 'The approval window is 14 days.' :
      state.rebuilt && s.id === 'prose-conflict' && n.id === 'caveat' ? 'The corrected source now supports the 14-day timing sentence.' :
      state.rebuilt && n.id === 'affected' ? 'The work begins in phase two.' : n.text,
      () => selectNode(n), `prose-node ${selected ? 'selected' : ''}`, false,
      { 'aria-pressed': selected ? 'true' : 'false', 'data-status': state.rebuilt && affectedIds.includes(n.id) ? (state.reviewFresh ? 'ready' : 'review') : n.status, 'data-key': `prose:${n.id}` }));
  });
  panel.append(page, notice('Draft text is read-only. Change a linked source, then rebuild and review the affected content.'));
  if (state.annotation) panel.append(node('p', { class: 'annotation-note' }, 'Presentation note: ', state.annotation));
  const linked = nodes.find(n => n.id === state.selected);
  if (linked) panel.append(notice(`Selected ${state.level} scope anchored at ${linked.id}. Linked source: ${linked.sources.join(', ')}.${s.variants && !s.variants[state.variant].includes(linked.id) ? ' This required claim is absent from the selected audience draft.' : ' The selected audience draft remains visible.'}`));
  if (state.compare) panel.append(node('div', { class: 'panel' }, label('Source version comparison'),
    h('p', 'diff-remove', s.id === 'stale-approval' ? '− phase one · SRC-41 v1' : '− 10 days · SRC-17 v2'),
    h('p', 'diff-add', s.id === 'stale-approval' ? '+ phase two · SRC-41 v2' : '+ 14 days · SRC-17 v3'),
    notice(`Changed: ${affectedIds.length} · Reused: ${Math.max(0, nodes.length - affectedIds.length)}. Text fixture counts, not semantic distance.`)));
  if (s.variants) {
    const required = nodes.map(n => n.id), present = s.variants[state.variant];
    panel.append(node('div', { class: 'variant-grid panel' }, label(`${state.variant} publication check`),
      ...required.map(id => node('p', { class: 'fact-row' }, id, ' · ',
        badge(present.includes(id) ? 'Present' : 'Missing', present.includes(id) ? 'ready' : 'blocked'))),
      notice(present.length === required.length ? 'All required claims present in this variant.' : 'Publication blocked: at least one required claim is missing.')));
  }
  if (['prose-conflict', 'stale-approval'].includes(s.id)) {
    const actions = node('div', { class: 'workspace-actions' });
    actions.append(button('Stage source correction', () => { state.corrected = true; state.rebuilt = false; state.reviewFresh = false; announce('Correction staged. Rebuild affected content.'); }, 'subtle', state.corrected || state.role === 'Reader'),
      button('Rebuild affected content', () => { state.rebuilt = true; state.selected = s.id === 'stale-approval' ? 'affected' : 'claim'; state.source = sourceKey(sourceFor(relatedSource(state.selected))); announce('Affected summary and audience variants rebuilt; unrelated content reused.'); }, 'primary', !state.corrected || state.rebuilt || state.role === 'Reader'),
      button('Record fresh review', () => { state.reviewFresh = true; announce('Fresh exact-revision review recorded.'); }, 'primary', !state.rebuilt || state.reviewFresh || state.role === 'Reader'));
    panel.append(actions);
    if (state.rebuilt) panel.append(node('div', { class: 'impact-grid', 'aria-label': 'Change impact' },
      label('Change impact'),
      node('div', { class: 'context-stat' }, h('strong', '', String(affectedIds.length)), h('span', '', 'Affected nodes rebuilt')),
      node('div', { class: 'context-stat' }, h('strong', '', String(nodes.length - affectedIds.length)), h('span', '', 'Nodes reused')),
      notice('Text fixture counts, not semantic distance. Summary and audience drafts use the rebuilt claim.')));
    panel.append(notice(state.role === 'Reader' ? 'Reader preview cannot simulate approval.' :
      state.reviewFresh ? 'New exact revision reviewed. Old approval remains invalid.' : 'Publication remains blocked until correction, rebuild, and fresh review.'));
  }
  return panel;
}
function reviewView() {
  const s = state.scenario, record = selectedRecord();
  const panel = node('section', { class: 'content' }, label('Decision surface'));
  if (s.id === 'agent-suggestion') {
    panel.append(h('h2', '', 'Typed change P-8'), node('div', { class: 'panel' },
      h('p', 'diff-remove', 'Before: summary-only output'), h('p', 'diff-add', 'After: evidence-linked output')));
    panel.append(selectField('Destination', state.destination, ['synthetic inbox A', 'synthetic inbox B'], v => {
      state.destination = v; state.approvals.clear(); state.checks.destination = false;
      announce('Destination changed. Prior decision invalidated; confirm the new destination.');
    }));
    for (const [key, text] of Object.entries({ purpose: 'Trusted purpose: evidence-linked review', destination: `Destination: ${state.destination}`, expiry: 'Expiry: end of this simulation' }))
      panel.append(node('label', { class: 'fact-row' }, node('input', { type: 'checkbox', 'data-key': `condition:${key}`, checked: state.checks[key], onChange: e => {
        state.checks[key] = e.target.checked; state.approvals.clear();
        announce(`${key} condition changed. Prior P-8 approval invalidated.`);
      } }), ` ${text}`));
    const eligible = Object.values(state.checks).every(Boolean) && state.role === 'Reviewer';
    panel.append(button(state.approvals.size ? `P-8 approved for ${state.destination}` : 'Approve exact proposed change',
      () => { state.approvals.add(`P-8 → ${state.destination}`); announce(`Exact P-8 destination ${state.destination} approved locally.`); },
      'primary', !eligible || state.approvals.size > 0));
    panel.append(notice(state.role === 'Reader' ? 'Reader preview cannot simulate approval.' : 'Changing destination invalidates the prior simulated decision.'));
  } else if (s.id === 'clarification') {
    panel.append(h('h2', '', 'What date is supported by the current evidence?'),
      notice('The premise is disputed: one source says Friday, another says Monday.'));
    panel.append(node('label', { class: 'control-field' }, 'Answer (optional for unknown or defer)',
      node('textarea', { rows: '3', 'aria-label': 'Clarification answer', onInput: e => {
        state.answer = e.target.value;
        const action = panel.querySelector('[data-action="record-answer"]');
        if (action) action.disabled = !state.answer.trim() || state.role === 'Reader';
      } }, state.answer)));
    const actions = node('div', { class: 'workspace-actions' });
    for (const [name, required] of [['Record answer', true], ['Mark unknown', false], ['Defer for evidence', false]])
      actions.append(button(name, () => { state.clarification = name; announce(`${name} retained with source and question.`); },
        'subtle', state.role === 'Reader' || (required && !state.answer.trim()), required ? { 'data-action': 'record-answer' } : {}));
    panel.append(actions, notice(`Question, source REQ-2 v2, and status remain visible. Current status: ${state.clarification}.`));
  } else {
    panel.append(h('h2', '', record?.title || 'No selected record'),
      notice(record ? `Selected ${record.id} · ${record.state}. Source: ${relatedSource(record.id) || s.sources[0]?.id}.` : 'No record selected.'));
    if (s.id === 'review-queue') {
      const blocked = record?.state !== 'Ready' || state.role === 'Reader' || state.approvals.has(record?.id);
      panel.append(button(state.approvals.has(record?.id) ? 'Local decision recorded' : `Approve ${record?.id}`,
        () => { state.approvals.add(record.id); announce(`${record.id} approved locally; queue order unchanged.`); }, 'primary', blocked));
      if (record?.state === 'Blocked') panel.append(notice('Hard block: privacy authority is missing. This item cannot be approved.'));
      if (state.role === 'Reader') panel.append(notice('Reader preview cannot simulate approval.'));
    } else panel.append(notice('Choose a scenario in the prototype toolbar for an executable review path.'));
  }
  return panel;
}
function evidenceView() {
  const s = state.scenario, panel = node('section', { class: 'content' });
  panel.append(h('h2', '', s.id === 'privacy-review' ? 'Redacted reference REF-9' : 'Evidence states'));
  if (s.id === 'privacy-review') {
    panel.append(node('div', { class: 'evidence-card', 'data-status': 'redacted' },
      h('strong', '', 'REF-9 v2'), h('p', '', '••••-••••-4821'), badge('Redacted')),
      notice('No real secret is present. Reveal and export have separate authority checks.'));
    panel.append(button('Request reveal authority', () => announce('Reveal authority is unavailable in this prototype. Reference remains redacted.'), 'subtle'),
      button('Request export authority', () => announce('Export authority is unavailable in this prototype. No file was created.'), 'subtle'));
  } else {
    for (const src of s.sources) panel.append(node('div', { class: 'evidence-card', 'data-status': src.state },
      h('strong', '', `${src.id} ${src.version}`), badge(src.state), h('p', '', src.detail),
      button('Inspect in evidence tab', () => { state.source = sourceKey(src); state.tab = 'Evidence'; render(); })));
  }
  return panel;
}
function runsView() {
  const s = state.scenario, panel = node('section', { class: 'content' });
  if (s.id === 'investigation-ladder') {
    panel.append(h('h2', '', 'Investigation ladder'), notice(`Shared synthetic allowance: ${state.allowance}/3 credits. No provider or price data.`));
    const steps = ['Local document check', 'Distinct cross-reference', 'Human review or stop'];
    panel.append(node('ol', { class: 'timeline' }, steps.map((x, i) => h('li', '', `${x} · ${i < state.runStep ? 'completed' : state.runStopped ? (i === 2 ? 'referred' : 'skipped') : i === state.runStep ? 'next' : 'waiting'}`))));
    if (state.runStep >= 1) panel.append(node('div', { class: 'panel' }, label('Local check · synthetic observation'),
      h('p', '', 'The supplied document contains a claim without an independently current source. The local check cannot resolve it.')));
    if (state.runStep >= 2) panel.append(node('div', { class: 'panel' }, label('Distinct cross-reference · synthetic observation'),
      h('p', '', 'A second authored source describes a different effective date. Evidence is distinct but disagreement remains. Refer to a human or stop.')));
    panel.append(button(state.runStep === 0 ? 'Run local document check · 1 credit' : 'Run distinct cross-reference · 1 credit',
      () => { state.runStep++; state.allowance--; announce(`Distinct synthetic evidence recorded. ${state.allowance} credits remain.`); },
      'primary', state.runStopped || state.runStep >= 2 || state.allowance < 1 || state.role === 'Reader'),
      button('Stop and refer to human', () => { state.runStopped = true; announce('Investigation stopped; human review requested. Unrun checks were skipped.'); },
        'subtle', state.runStopped || state.role === 'Reader'));
  } else if (s.id === 'delivery-unknown') {
    panel.append(h('h2', '', 'Delivery D-6'), notice('One simulated send consumed decision A-1. A repeat is blocked while status is unknown.'));
    panel.append(selectField('Mock receipt query result', state.delivery, ['Unknown', 'Delivered', 'Confirmed absent'], v => {
      state.delivery = v; state.absentDecision = false; announce(`Mock query returned ${v}.`);
    }));
    panel.append(node('div', { class: 'panel' }, badge(state.delivery), h('p', '',
      state.delivery === 'Unknown' ? 'Wait or investigate the same attempt; do not resend.' :
      state.delivery === 'Delivered' ? 'Record delivery; do not resend.' : 'Fresh decision required before a new simulated attempt.')));
    panel.append(button('Make fresh mock send decision', () => { state.absentDecision = true; announce('Fresh decision A-2 recorded after confirmed absence.'); },
      'primary', state.delivery !== 'Confirmed absent' || state.absentDecision || state.role === 'Reader'),
      button('Simulate new attempt with A-2', () => { state.delivery = 'Unknown'; state.absentDecision = false; announce('New simulated attempt consumed A-2; result unknown.'); },
        'subtle', !state.absentDecision || state.role === 'Reader'));
    if (state.role === 'Reader') panel.append(notice('Reader preview cannot make a fresh send decision.'));
  } else panel.append(h('h2', '', 'Runs'), notice('This synthetic subject has no run to execute. Choose Investigation ladder or Delivery unknown.'));
  return panel;
}
function catalogView() {
  const panel = node('section', { class: 'content' }, h('h2', '', 'Model decision trace'),
    notice('Synthetic candidates; no model IDs, provider prices, or measured savings.'));
  const rows = [
    ['Available to org', 'Local compact, Local thorough, External example'],
    ['Eligible for task', 'Local compact, Local thorough'],
    ['Considered', 'Local compact, Local thorough'],
    ['Evaluated', 'Local compact'],
    ['Selected', 'Local compact'],
  ];
  const table = node('table', { class: 'data-table' });
  table.append(node('thead', {}, node('tr', {}, h('th', '', 'Stage'), h('th', '', 'Synthetic set'))));
  table.append(node('tbody', {}, rows.map(([a,b]) => node('tr', {}, h('th', '', a), h('td', '', b)))));
  panel.append(table, notice('A task-ineligible option can be available to the org without reaching selection.'));
  return panel;
}
function learningView() {
  const panel = node('section', { class: 'content' }, h('h2', '', 'Issue signal SIG-3'),
    notice('The signal is inert. Steps below add synthetic evidence but never change policy.'));
  const stages = ['Candidate reviewed', 'Mismatch reproduced', 'Shadow run observed', 'Receiver accepted'];
  panel.append(node('ol', { class: 'timeline' }, stages.map((x,i) => h('li', '', `${x} · ${i < state.lessonStep ? 'complete' : i === state.lessonStep ? 'next' : 'waiting'}`))));
  panel.append(button(state.lessonStep < 4 ? stages[state.lessonStep] : 'Receiver accepted', () => {
    state.lessonStep++; announce(`${stages[state.lessonStep - 1]}. Policy remains unchanged.`);
  }, 'primary', state.lessonStep >= 4 || state.role === 'Reader'));
  if (state.lessonStep === 3 && state.role === 'Reader') panel.append(notice('Reader preview cannot simulate receiver acceptance.'));
  if (state.lessonStep === 4) panel.append(notice('Receiver accepted this local lesson candidate. No automatic policy effect.'));
  return panel;
}
function content() {
  return ({ review: reviewView, prose: proseView, evidence: evidenceView, runs: runsView,
    catalog: catalogView, learning: learningView })[state.view]();
}
function hostDocument() {
  return node('article', { class: 'host-page', 'aria-label': 'Synthetic host context' },
    label('Working document · synthetic fixture'), h('h2', '', 'Release readiness brief'),
    h('p', 'host-lead', 'A useful decision leaves a clear trail from the source to the proposed change, and from the change to its consequence.'),
    node('div', { class: 'host-facts' },
      node('div', {}, label('State'), h('strong', '', 'Draft for review')),
      node('div', {}, label('Destination'), h('strong', '', state.destination)),
      node('div', {}, label('Authority'), h('strong', '', 'None in this demo'))),
    h('h3', '', 'What is being considered'),
    h('p', '', 'The working brief currently uses a short summary. The proposed workflow would attach source references and make unresolved claims visible before the brief is used.'),
    h('h3', '', 'What the reviewer needs'),
    node('ul', {}, h('li', '', 'The exact proposed difference and its intended destination.'),
      h('li', '', 'Current source versions, missing evidence and applicable criteria.'),
      h('li', '', 'An explicit hold when authority or the outcome is unknown.')),
    h('h3', '', 'Decision context'),
    h('p', '', `Current exploration: ${state.scenario.title}. The assistance panel presents this task beside the host. Expanding the workbench retains its selected subject.`),
    notice('Host context is illustrative. Approval controls only change local simulation state. Nothing is sent or saved.'));
}
function inlineHost() {
  return node('main', { class: 'compact-host', id: 'workspace-main', tabindex: '-1' },
    h('p', 'section-label', 'Host document · embedded assistance'),
    h('h1', '', 'Draft decision brief'),
    contextStrip(),
    h('p', '', 'A synthetic host document keeps the task in context. The workbench can expand when review needs more room.'),
    state.announcement ? h('p', 'status', state.announcement) : null,
    node('div', { class: 'compact-card' }, badge(state.scenario.state),
      h('strong', '', state.scenario.title), h('p', '', state.scenario.instruction),
      button('Inspect evidence', e => openDialog('Synthetic evidence', node('div', {},
        state.scenario.sources.map(s => node('p', { class: 'evidence-card' }, `${s.id} ${s.version} · ${s.state}: ${s.detail}`))), e.currentTarget), 'mini-trigger'),
      button('Expand workbench', () => { state.mode = 'full'; url(); render(); root.querySelector('#workspace-main')?.focus(); }, 'primary')), hostDocument());
}
function render() {
  if (!root) return;
  const active = root.contains(document.activeElement) ? document.activeElement : null;
  const focusKey = active?.getAttribute('data-key') || active?.getAttribute('aria-label') || active?.textContent?.trim();
  const selection = active && Number.isInteger(active.selectionStart) && Number.isInteger(active.selectionEnd)
    ? [active.selectionStart, active.selectionEnd] : null;
  root.dataset.mode = state.mode; root.dataset.theme = state.theme.toLowerCase();
  root.replaceChildren(toolbar());
  if (state.mode === 'inline') root.append(inlineHost());
  else {
    const shell = node('div', { class: `app-shell mode-${state.mode} theme-${state.theme.toLowerCase()}` });
    shell.append(nav());
    const workspace = node('main', { class: 'workspace', id: 'workspace-main', tabindex: '-1' });
    const displayState = state.reviewFresh ? 'Fresh review recorded' : state.rebuilt ? 'Rebuilt · review required' : state.scenario.state;
    const header = node('header', { class: 'workspace-header' },
      node('div', {}, label(state.scenario.id), h('h1', '', state.scenario.title),
        h('p', '', `${state.scenario.revision} · ${displayState}`)), badge(displayState));
    workspace.append(header, contextStrip(), notice(state.scenario.instruction));
    if (state.announcement) workspace.append(node('p', { class: 'status', role: 'status', text: state.announcement }));
    workspace.append(node('div', { class: 'toolbar record-picker' }, selectField('Selected record', state.selected,
      state.scenario.queue.map(r => ({ id: r.id, title: `${r.title} · ${state.approvals.has(r.id) ? 'Approved' : r.state}` })), selectRecord)));
    const grid = node('div', { class: 'work-grid' });
    if (state.mode === 'full') grid.append(queue());
    if (state.mode === 'panel') grid.append(node('section', { class: 'compact-host panel-host' },
      hostDocument()));
    if (state.mode === 'panel') grid.append(node('aside', { class: 'drawer', 'aria-label': 'Assistance panel' }, content(), inspector()));
    else grid.append(content(), inspector());
    workspace.append(grid); shell.append(workspace); root.append(shell);
  }
  const live = node('div', { class: 'sr-only', role: 'status', 'aria-live': 'polite', text: state.announcement });
  root.append(live, node('div', { class: 'simulation-mark', 'aria-hidden': 'true', text: 'BACKENDLESS · SYNTHETIC' }));
  if (focusKey) {
    const candidates = root.querySelectorAll('button, select, input, textarea');
    const replacement = [...candidates].find(el => !el.disabled &&
      (el.getAttribute('data-key') === focusKey || el.getAttribute('aria-label') === focusKey || el.textContent?.trim() === focusKey));
    (replacement || root.querySelector('#workspace-main'))?.focus({ preventScroll: true });
    if (selection && replacement && 'setSelectionRange' in replacement) replacement.setSelectionRange(...selection);
  }
}

url(); render();
