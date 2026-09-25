import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import {deflateSync,inflateSync} from 'node:zlib';
import test from 'node:test';

// Exercise the exact local oracle functions without importing the browser entry,
// accessing any fixture server, or constructing a second implementation.
const source=fs.readFileSync(new URL('./scale.mjs',import.meta.url),'utf8');
const visibleExpression=source.match(/const visible=(b=>[^\n]+);/);
assert(visibleExpression,'capture visibility oracle must remain identifiable');
const visible=vm.runInNewContext('('+visibleExpression[1]+')',{viewport:{width:390,height:1000}});
const start=source.indexOf('function pngPixels('),end=source.indexOf('\nconst missingReason',start);
assert(start>=0&&end>start,'PNG oracle must remain identifiable');
const pngPixels=vm.runInNewContext('('+source.slice(start,end).trim()+')',{Buffer,inflateSync});

test('capture refuses X-correct targets above or below viewport',()=>{
  assert(visible({x:20,y:40,width:128,height:57}));
  for(const y of [-60,1001,980])assert(!visible({x:20,y,width:128,height:57}));
});
test('capture refuses absent empty or horizontally clipped targets',()=>{
  for(const box of [null,{x:20,y:40,width:0,height:57},{x:-140,y:40,width:128,height:57},{x:350,y:40,width:128,height:57}])assert(!visible(box));
});

function chunk(type,bytes) {
  const name=Buffer.from(type),payload=Buffer.concat([name,bytes]);let crc=0xffffffff;
  for(const byte of payload){crc^=byte;for(let i=0;i<8;i++)crc=(crc>>>1)^((crc&1)?0xedb88320:0);}
  const head=Buffer.alloc(4),tail=Buffer.alloc(4);head.writeUInt32BE(bytes.length);tail.writeUInt32BE((crc^0xffffffff)>>>0);
  return Buffer.concat([head,payload,tail]);
}
function fixture(blank=false) {
  const width=390,height=1000,header=Buffer.alloc(13),raw=Buffer.alloc((width*3+1)*height);
  header.writeUInt32BE(width);header.writeUInt32BE(height,4);header[8]=8;header[9]=2;
  for(let y=0;y<height;y++)for(let x=0;x<width;x++){
    const offset=y*(width*3+1)+1+x*3;raw[offset]=blank?240:x%256;raw[offset+1]=blank?240:y%256;raw[offset+2]=240;
  }
  return Buffer.concat([Buffer.from([137,80,78,71,13,10,26,10]),chunk('IHDR',header),chunk('IDAT',deflateSync(raw)),chunk('IEND',Buffer.alloc(0))]);
}
test('PNG oracle decodes complete nonblank expected viewport',()=>{
  const observed=pngPixels(fixture(),390);
  assert.equal(observed.width,390);assert.equal(observed.height,1000);assert.equal(observed.decoded_rows,1000);assert(observed.distinct_colors_at_least>=32);
});
test('PNG oracle refuses blank wrong-width truncated and invalid-signature captures',()=>{
  assert.throws(()=>pngPixels(fixture(true),390),/pixel variation/);
  assert.throws(()=>pngPixels(fixture(),1440),/geometry/);
  assert.throws(()=>pngPixels(fixture().subarray(0,-12),390),/Incomplete PNG/);
  assert.throws(()=>pngPixels(Buffer.alloc(32),390),/signature/);
});
