const freezeRow = row => Object.freeze(row)

export const expectedRows = Object.freeze([
  freezeRow({ID:'p01', Name:'Ada Vale', Title:'Software Engineer', Email:'p01@example.invalid', Role:'Member'}),
  freezeRow({ID:'p02', Name:'Bruno Chen', Title:'Product Designer', Email:'p02@example.invalid', Role:'Owner'}),
  freezeRow({ID:'p03', Name:'Cora Nunez', Title:'QA Analyst', Email:'p03@example.invalid', Role:'Viewer'}),
  freezeRow({ID:'p04', Name:'Dae Park', Title:'Software Engineer', Email:'p04@example.invalid', Role:'Member'}),
  freezeRow({ID:'p05', Name:'Erin Holt', Title:'Product Designer', Email:'p05@example.invalid', Role:'Member'}),
  freezeRow({ID:'p06', Name:'Farah Issa', Title:'Customer Support', Email:'p06@example.invalid', Role:'Member'}),
  freezeRow({ID:'p07', Name:'Gita Sen', Title:'Platform Engineer', Email:'p07@example.invalid', Role:'Member'}),
  freezeRow({ID:'p08', Name:'Hugo Reed', Title:'Finance Analyst', Email:'p08@example.invalid', Role:'Member'}),
  freezeRow({ID:'p09', Name:'Imani Cole', Title:'Product Designer', Email:'p09@example.invalid', Role:'Member'}),
  freezeRow({ID:'p10', Name:'Jules Mora', Title:'Software Engineer', Email:'p10@example.invalid', Role:'Member'}),
  freezeRow({ID:'p11', Name:'Kai "Quoted", Vale', Title:'QA Analyst', Email:'p11@example.invalid', Role:'Member'}),
  freezeRow({ID:'p12', Name:'=1+1', Title:'Security Analyst', Email:'p12@example.invalid', Role:'Member'}),
])

export const csvHeaders = Object.freeze([
  'dataset_revision', 'coverage', 'query', 'id', 'name', 'title', 'email', 'role',
])

const shaPattern = /^[0-9a-f]{64}$/
const pageSize = 3
const exactFields = (value, fields) => value && typeof value === 'object' && !Array.isArray(value) &&
  Object.keys(value).sort().join('\0') === [...fields].sort().join('\0')

function validUnicode(value) {
  for (let i = 0; i < value.length; i++) {
    const unit = value.charCodeAt(i)
    if (unit >= 0xd800 && unit <= 0xdbff) {
      const next = value.charCodeAt(++i)
      if (!(next >= 0xdc00 && next <= 0xdfff)) return false
    } else if (unit >= 0xdc00 && unit <= 0xdfff) return false
  }
  return true
}

function normalizeQuery(value) {
  if (typeof value !== 'string' || !validUnicode(value) || /\p{Cc}/u.test(value)) return null
  const query = value.trim()
  if (Buffer.byteLength(query, 'utf8') > 120) return null
  return query
}

function normalizePage(value) {
  if (typeof value === 'number') return Number.isSafeInteger(value) && value > 0 ? value : null
  if (typeof value !== 'string' || !/^[0-9]+$/.test(value)) return null
  const page = Number(value)
  return Number.isSafeInteger(page) && page > 0 ? page : null
}

function validID(value) {
  return typeof value === 'string' && value.length > 0 && validUnicode(value) &&
    Buffer.byteLength(value, 'utf8') <= 120 && !/\p{Cc}/u.test(value)
}

const safeCell = value => /^[=+\-@]/.test(value) ? `'${value}` : value
const same = (left, right) => JSON.stringify(left) === JSON.stringify(right)

export function expectedSelection(input = {}) {
  if (!input || typeof input !== 'object' || Array.isArray(input)) input = {}
  const q = normalizeQuery(input.q ?? '')
  const page = normalizePage(input.page ?? 1)
  const state = input.state ?? 'complete'
  const id = input.id
  const datasetRevision = input.datasetRevision
  const invalid = q === null || page === null || !['complete', 'partial', 'unavailable'].includes(state) ||
    (id !== undefined && !validID(id))
  const base = {q:q ?? '', page:page ?? null, state, id, datasetRevision}
  if (invalid) return Object.freeze({...base, status:400, ids:Object.freeze([]), pageCount:null, matchingCount:null, coverage:null, matchingIDs:Object.freeze([]), row:null})
  if (state === 'unavailable') return Object.freeze({...base, status:503, ids:Object.freeze([]), pageCount:null, matchingCount:null, coverage:'unavailable', matchingIDs:Object.freeze([]), row:null})

  const observed = state === 'partial' ? expectedRows.slice(0, 6) : expectedRows
  const needle = q.toLocaleLowerCase('en-US')
  const matches = observed.filter(row => !needle || [row.Name,row.Title,row.Email,row.Role]
    .some(value => value.toLocaleLowerCase('en-US').includes(needle)))
  const pageCount = Math.max(1, Math.ceil(matches.length / pageSize))
  if (page > pageCount) return Object.freeze({...base, status:400, ids:Object.freeze([]), pageCount:null, matchingCount:null, coverage:null, matchingIDs:Object.freeze(matches.map(row => row.ID)), row:null})
  const matchingIDs = Object.freeze(matches.map(candidate => candidate.ID))
  const row = id === undefined ? null : matches.find(candidate => candidate.ID === id) ?? null
  // A detail error exposes neither counts nor coverage; never infer them from its retained selection.
  if (id !== undefined && row === null) return Object.freeze({...base, status:404, ids:Object.freeze([]), pageCount:null, matchingCount:null, coverage:null, matchingIDs, row:null})
  if (id !== undefined) return Object.freeze({...base, status:200, ids:Object.freeze([]),
    pageCount, matchingCount:matches.length, coverage:state, matchingIDs, row})
  return Object.freeze({...base, status:200,
    ids:Object.freeze(matches.slice((page-1)*pageSize, page*pageSize).map(candidate => candidate.ID)),
    pageCount, matchingCount:matches.length, coverage:state,
    matchingIDs, row})
}

function expectedCSV(selection) {
  const byID = new Map(expectedRows.map(row => [row.ID, row]))
  return selection.matchingIDs.map(id => {
    const row = byID.get(id)
    return [selection.datasetRevision, selection.coverage, safeCell(selection.q), row.ID,
      safeCell(row.Name), safeCell(row.Title), safeCell(row.Email), safeCell(row.Role)]
  })
}

export function checkObservation(selection, observation) {
  const failures = []
  const fail = code => { if (!failures.includes(code)) failures.push(code) }
  if (!selection || typeof selection !== 'object') return ['selection.invalid']
  if (!exactFields(observation, ['status','ids','pageCount','matchingCount','coverage','detail','csv']) &&
      !exactFields(observation, ['status','ids','pageCount','matchingCount','coverage','detail']) &&
      !exactFields(observation, ['status','ids','pageCount','matchingCount','coverage','csv']) &&
      !exactFields(observation, ['status','ids','pageCount','matchingCount','coverage'])) return ['observation.fields']
  if (observation.status !== selection.status) fail('status')
  if (!same(observation.ids, selection.ids)) fail('ids')
  if (observation.pageCount !== selection.pageCount) fail('pageCount')
  if (observation.matchingCount !== selection.matchingCount) fail('matchingCount')
  if (observation.coverage !== selection.coverage) fail('coverage')

  if (selection.status === 200 && selection.id !== undefined) {
    const detail = observation.detail
    if (!exactFields(detail, ['ID','Name','Title','Email','Role','backQuery','backPage','backState'])) fail('detail.fields')
    else {
      for (const field of ['ID','Name','Title','Email','Role']) if (detail[field] !== selection.row[field]) fail(`detail.${field}`)
      if (detail.backQuery !== selection.q) fail('detail.backQuery')
      if (detail.backPage !== selection.page) fail('detail.backPage')
      if (detail.backState !== selection.state) fail('detail.backState')
    }
  } else if (observation.detail !== undefined) fail('detail.unexpected')

  if (observation.csv !== undefined) {
    if (selection.status !== 200) fail('csv.unavailable')
    else if (!shaPattern.test(selection.datasetRevision ?? '')) fail('selection.datasetRevision')
    else if (!exactFields(observation.csv, ['headers','rows'])) fail('csv.fields')
    else {
      if (!same(observation.csv.headers, csvHeaders)) fail('csv.headers')
      if (!same(observation.csv.rows, expectedCSV(selection))) fail('csv.rows')
    }
  }
  return failures
}
