import assert from 'node:assert/strict';
import test from 'node:test';
import {navbarZoomCaptureClip} from './zoom-contract.mjs';

test('actual navbar zoom admits only its two complete DIP windows',()=>{
  for(const width of [780,1440]) {
    const m={contentSize:{x:0,y:0,width,height:4051},cssContentSize:{x:0,y:0,width:width/2,height:2025}};
    assert.deepEqual(navbarZoomCaptureClip(m,width),{...m.contentSize,scale:1});
    for(const [field,value] of [['width',width+1],['x',1],['height',Infinity],['height',0],['height',20001],['height',4053]]) {
      const bad=structuredClone(m);bad.contentSize[field]=value;assert.throws(()=>navbarZoomCaptureClip(bad,width));
    }
    const fake=structuredClone(m);fake.cssContentSize.width=width;assert.throws(()=>navbarZoomCaptureClip(fake,width));
    assert.throws(()=>navbarZoomCaptureClip(m,390));assert.throws(()=>navbarZoomCaptureClip({},width));
  }
});
