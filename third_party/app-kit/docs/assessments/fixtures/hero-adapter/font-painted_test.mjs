import assert from 'node:assert/strict';
import test from 'node:test';
import {inspectPaintedFonts} from './font-painted.mjs';
const good=()=>[{selector:'body > p:nth-child(1)',fonts:[{isCustomFont:true,familyName:'Inter Variable',postScriptName:'InterVariable-Thin',glyphCount:12}]}];
test('actual custom Inter glyph records pass, including two loaded Inter faces',()=>{
  const nodes=good();assert.deepEqual(inspectPaintedFonts(nodes),[]);
  nodes[0].fonts.push({...nodes[0].fonts[0],postScriptName:'InterVariable',glyphCount:2});assert.deepEqual(inspectPaintedFonts(nodes),[]);
});
test('missing, system, wrong-family and malformed glyph evidence refuses',()=>{
  for(const nodes of [null,[],Array(513).fill(good()[0]),[{}],[{selector:'x',fonts:[]}],good().concat(good())])assert.ok(inspectPaintedFonts(nodes).length);
  for(const patch of [{isCustomFont:false},{familyName:'Lucida Grande'},{postScriptName:'Fake'},{glyphCount:0},{glyphCount:1.2},{glyphCount:'12'}, {isCustomFont:'true'}]){const nodes=good();Object.assign(nodes[0].fonts[0],patch);assert.ok(inspectPaintedFonts(nodes).length,JSON.stringify(patch))}
});
