import test from 'node:test';
import assert from 'node:assert/strict';
import {parseCSV} from './csv.mjs';

test('decodes quotes, commas, line breaks and formula safety text without normalization',()=>{
  assert.deepEqual(parseCSV('id,name,note\r\np11,"Kai ""Quoted"", Vale","two\nlines"\r\np12,\'=1+1,\r\n'),[
    ['id','name','note'],['p11','Kai "Quoted", Vale','two\nlines'],['p12',"'=1+1",''],
  ]);
});
test('preserves explicit empty cells without inventing a final row',()=>{
  for(const input of [',,','\"\",,\n',',,\r\n'])assert.deepEqual(parseCSV(input),[['','','']]);
  assert.deepEqual(parseCSV(''),[]);
});
test('refuses malformed evidence rather than repairing it',()=>{
  for(const input of ['a"b,c','"a"x,b','"a','a\rb'])assert.throws(()=>parseCSV(input));
});
