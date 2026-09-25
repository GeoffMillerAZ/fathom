// P09-local capture admission. Do not broaden the frozen Gallery/drawer oracle.
export function navbarZoomCaptureClip(metrics, physicalWidth) {
  if (![780, 1440].includes(physicalWidth)) throw Error('Unsupported navbar window');
  const dip=metrics?.contentSize,css=metrics?.cssContentSize;
  for(const r of [dip,css]) {
    if(!r||!['x','y','width','height'].every(k=>Number.isFinite(r[k]))||r.x!==0||r.y!==0||r.height<=0)throw Error('Invalid navbar capture metrics');
  }
  if(dip.width!==physicalWidth||css.width!==physicalWidth/2||!Number.isInteger(dip.height)||
    dip.height>20000||Math.abs(dip.height-css.height*2)>1)throw Error('Unexpected actual navbar zoom geometry');
  return {...dip,scale:1};
}
