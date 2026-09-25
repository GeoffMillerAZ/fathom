import test from 'node:test'
import assert from 'node:assert/strict'
import {checkObservation, csvHeaders, expectedRows, expectedSelection} from './oracle.mjs'

const revision = 'a'.repeat(64)
const completeEngineerPage1 = {status:200,ids:['p01','p04','p07'],pageCount:2,matchingCount:4,coverage:'complete'}
const completeEngineerPage2 = {status:200,ids:['p10'],pageCount:2,matchingCount:4,coverage:'complete'}
const engineerDetail = {status:200,ids:[],pageCount:2,matchingCount:4,coverage:'complete'}
const unavailableObservation = {status:503,ids:[],pageCount:null,matchingCount:null,coverage:'unavailable'}

test('literal dataset has the frozen identities and exceptional names', () => {
  assert.deepEqual(expectedRows.map(row => [row.ID,row.Name,row.Title,row.Email,row.Role]), [
    ['p01','Ada Vale','Software Engineer','p01@example.invalid','Member'],
    ['p02','Bruno Chen','Product Designer','p02@example.invalid','Owner'],
    ['p03','Cora Nunez','QA Analyst','p03@example.invalid','Viewer'],
    ['p04','Dae Park','Software Engineer','p04@example.invalid','Member'],
    ['p05','Erin Holt','Product Designer','p05@example.invalid','Member'],
    ['p06','Farah Issa','Customer Support','p06@example.invalid','Member'],
    ['p07','Gita Sen','Platform Engineer','p07@example.invalid','Member'],
    ['p08','Hugo Reed','Finance Analyst','p08@example.invalid','Member'],
    ['p09','Imani Cole','Product Designer','p09@example.invalid','Member'],
    ['p10','Jules Mora','Software Engineer','p10@example.invalid','Member'],
    ['p11','Kai "Quoted", Vale','QA Analyst','p11@example.invalid','Member'],
    ['p12','=1+1','Security Analyst','p12@example.invalid','Member'],
  ])
})

test('literal complete, filtered, partial and empty selections', () => {
  assert.deepEqual(expectedSelection(), {
    q:'',page:1,state:'complete',id:undefined,datasetRevision:undefined,status:200,
    ids:['p01','p02','p03'],pageCount:4,matchingCount:12,coverage:'complete',
    matchingIDs:['p01','p02','p03','p04','p05','p06','p07','p08','p09','p10','p11','p12'],row:null,
  })
  const engineer = expectedSelection({q:'  EnGiNeEr ',page:'2'})
  assert.deepEqual({q:engineer.q,ids:engineer.ids,pageCount:engineer.pageCount,matchingCount:engineer.matchingCount},
    {q:'EnGiNeEr',ids:['p10'],pageCount:2,matchingCount:4})
  const partial = expectedSelection({state:'partial',page:2})
  assert.deepEqual({ids:partial.ids,pageCount:partial.pageCount,matchingCount:partial.matchingCount,coverage:partial.coverage},
    {ids:['p04','p05','p06'],pageCount:2,matchingCount:6,coverage:'partial'})
  const empty = expectedSelection({q:'not present'})
  assert.deepEqual({status:empty.status,ids:empty.ids,pageCount:empty.pageCount,matchingCount:empty.matchingCount},
    {status:200,ids:[],pageCount:1,matchingCount:0})
})

test('unavailable and malformed pages never become empty success', () => {
  const unavailable = expectedSelection({state:'unavailable'})
  assert.deepEqual({status:unavailable.status,ids:unavailable.ids,pageCount:unavailable.pageCount,matchingCount:unavailable.matchingCount,coverage:unavailable.coverage},
    {status:503,ids:[],pageCount:null,matchingCount:null,coverage:'unavailable'})
  for (const page of ['bad',0,-1,1.5]) assert.equal(expectedSelection({page}).status, 400)
  assert.equal(expectedSelection({q:'engineer',page:3}).status, 400)
  assert.equal(expectedSelection({q:'bad\nquery'}).status, 400)
  assert.equal(expectedSelection({q:'\nengineer'}).status, 400)
  assert.equal(expectedSelection({state:'invented'}).status, 400)
  assert.deepEqual({status:expectedSelection({id:'p99'}).status,ids:expectedSelection({id:'p99'}).ids}, {status:404,ids:[]})
  assert.equal(expectedSelection({q:'engineer',id:'p02'}).status, 404)
})

test('valid detail has no table rows, retains declared counts and selection', () => {
  const selection = expectedSelection({q:'engineer',page:2,id:'p10'})
  const detail = {ID:'p10',Name:'Jules Mora',Title:'Software Engineer',Email:'p10@example.invalid',Role:'Member',backQuery:'engineer',backPage:2,backState:'complete'}
  assert.deepEqual(checkObservation(selection, {...engineerDetail,detail}), [])
  assert.deepEqual(checkObservation(selection, {...engineerDetail,ids:['p10'],detail}), ['ids'])
  assert.deepEqual(checkObservation(selection, {...engineerDetail,pageCount:null,detail}), ['pageCount'])
  assert.deepEqual(checkObservation(selection, {...engineerDetail,matchingCount:null,detail}), ['matchingCount'])
  assert.deepEqual(checkObservation(selection, {...engineerDetail,coverage:null,detail}), ['coverage'])
})

test('complete CSV contains all matching rows, not only page two', () => {
  const selection = expectedSelection({q:'engineer',page:2,datasetRevision:revision})
  const csv = {headers:[...csvHeaders],rows:[
    [revision,'complete','engineer','p01','Ada Vale','Software Engineer','p01@example.invalid','Member'],
    [revision,'complete','engineer','p04','Dae Park','Software Engineer','p04@example.invalid','Member'],
    [revision,'complete','engineer','p07','Gita Sen','Platform Engineer','p07@example.invalid','Member'],
    [revision,'complete','engineer','p10','Jules Mora','Software Engineer','p10@example.invalid','Member'],
  ]}
  assert.deepEqual(checkObservation(selection, {...completeEngineerPage2,csv}), [])
})

test('CSV preserves parsed quotes and spreadsheet-escapes the formula name', () => {
  const selection = expectedSelection({page:4,datasetRevision:revision})
  const csv = {headers:[...csvHeaders],rows:[
    [revision,'complete','','p01','Ada Vale','Software Engineer','p01@example.invalid','Member'],
    [revision,'complete','','p02','Bruno Chen','Product Designer','p02@example.invalid','Owner'],
    [revision,'complete','','p03','Cora Nunez','QA Analyst','p03@example.invalid','Viewer'],
    [revision,'complete','','p04','Dae Park','Software Engineer','p04@example.invalid','Member'],
    [revision,'complete','','p05','Erin Holt','Product Designer','p05@example.invalid','Member'],
    [revision,'complete','','p06','Farah Issa','Customer Support','p06@example.invalid','Member'],
    [revision,'complete','','p07','Gita Sen','Platform Engineer','p07@example.invalid','Member'],
    [revision,'complete','','p08','Hugo Reed','Finance Analyst','p08@example.invalid','Member'],
    [revision,'complete','','p09','Imani Cole','Product Designer','p09@example.invalid','Member'],
    [revision,'complete','','p10','Jules Mora','Software Engineer','p10@example.invalid','Member'],
    [revision,'complete','','p11','Kai "Quoted", Vale','QA Analyst','p11@example.invalid','Member'],
    [revision,'complete','','p12',"'=1+1",'Security Analyst','p12@example.invalid','Member'],
  ]}
  assert.deepEqual(checkObservation(selection, {status:200,ids:['p10','p11','p12'],pageCount:4,matchingCount:12,coverage:'complete',csv}), [])
})

test('row, count and coverage canaries are independently rejected', () => {
  const selection = expectedSelection({q:'engineer'})
  const dropped = {...completeEngineerPage1,ids:['p01','p04']}
  const reordered = {...completeEngineerPage1,ids:['p04','p01','p07']}
  assert.deepEqual(checkObservation(selection, dropped), ['ids'])
  assert.deepEqual(checkObservation(selection, reordered), ['ids'])
  assert.deepEqual(checkObservation(selection, {...completeEngineerPage1,pageCount:9}), ['pageCount'])
  assert.deepEqual(checkObservation(selection, {...completeEngineerPage1,matchingCount:3}), ['matchingCount'])
  assert.deepEqual(checkObservation(selection, {...completeEngineerPage1,coverage:'partial'}), ['coverage'])
})

test('stale detail and Back selection canaries are independently rejected', () => {
  const selection = expectedSelection({q:'engineer',page:2,id:'p10'})
  const detail = {ID:'p10',Name:'Jules Mora',Title:'Software Engineer',Email:'p10@example.invalid',Role:'Member',backQuery:'engineer',backPage:2,backState:'complete'}
  assert.deepEqual(checkObservation(selection, {...engineerDetail,detail:{...detail,ID:'p07'}}), ['detail.ID'])
  assert.deepEqual(checkObservation(selection, {...engineerDetail,detail:{...detail,backQuery:''}}), ['detail.backQuery'])
  assert.deepEqual(checkObservation(selection, {...engineerDetail,detail:{...detail,backPage:1}}), ['detail.backPage'])
  assert.deepEqual(checkObservation(selection, {...engineerDetail,detail:{...detail,backState:'partial'}}), ['detail.backState'])
})

test('CSV row, order, formula and revision canaries are rejected', () => {
  const selection = expectedSelection({q:'=1+1',datasetRevision:revision})
  const good = {headers:[...csvHeaders],rows:[[revision,'complete',"'=1+1",'p12',"'=1+1",'Security Analyst','p12@example.invalid','Member']]}
  const formulaObservation = {status:200,ids:['p12'],pageCount:1,matchingCount:1,coverage:'complete'}
  assert.deepEqual(checkObservation(selection, {...formulaObservation,csv:good}), [])
  assert.deepEqual(checkObservation(selection, {...formulaObservation,csv:{...good,rows:[]}}), ['csv.rows'])
  assert.deepEqual(checkObservation(selection, {...formulaObservation,csv:{...good,rows:[[revision,'complete','=1+1','p12','=1+1','Security Analyst','p12@example.invalid','Member']]}}), ['csv.rows'])
  assert.deepEqual(checkObservation(selection, {...formulaObservation,csv:{...good,headers:['wrong']}}), ['csv.headers'])
  const noRevision = expectedSelection({q:'=1+1'})
  assert.deepEqual(checkObservation(noRevision, {...formulaObservation,csv:good}), ['selection.datasetRevision'])
})

test('error observations require unknown counts and cannot carry detail or CSV', () => {
  const selection = expectedSelection({state:'unavailable'})
  assert.deepEqual(checkObservation(selection, unavailableObservation), [])
  assert.deepEqual(checkObservation(selection, {...unavailableObservation,matchingCount:0}), ['matchingCount'])
  assert.deepEqual(checkObservation(selection, {...unavailableObservation,detail:{},csv:{headers:[],rows:[]}}),
    ['detail.unexpected','csv.unavailable'])
})

test('detail 404 exposes no inferred rows, counts or coverage', () => {
  const unknown = expectedSelection({id:'p99'})
  const observation = {status:404,ids:[],pageCount:null,matchingCount:null,coverage:null}
  assert.deepEqual(checkObservation(unknown, observation), [])
  assert.deepEqual(checkObservation(unknown, {...observation,ids:['p99']}), ['ids'])
  assert.deepEqual(checkObservation(unknown, {...observation,matchingCount:12}), ['matchingCount'])
  assert.deepEqual(checkObservation(unknown, {...observation,coverage:'complete'}), ['coverage'])

  const excluded = expectedSelection({q:'engineer',state:'partial',id:'p02'})
  assert.deepEqual(checkObservation(excluded, observation), [])
})

test('strict observation shapes reject extra or malformed fields', () => {
  const selection = expectedSelection()
  const pageOne = {status:200,ids:['p01','p02','p03'],pageCount:4,matchingCount:12,coverage:'complete'}
  assert.deepEqual(checkObservation(selection, {...pageOne,claimedPass:true}), ['observation.fields'])
  assert.deepEqual(checkObservation(selection, null), ['observation.fields'])
})
