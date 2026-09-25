// Observation decoding only. Expected values live in the independent oracle.
export function parseCSV(text) {
  if (typeof text !== 'string') throw Error('CSV text required');
  const rows=[]; let row=[],field='',quoted=false,closed=false,started=false;
  const cell=()=>{row.push(field);field='';closed=false;started=false};
  const line=()=>{cell();rows.push(row);row=[]};
  for(let i=0;i<text.length;i++){
    const c=text[i];
    if(quoted){
      if(c==='"'&&text[i+1]==='"'){field+='"';i++}
      else if(c==='"'){quoted=false;closed=true}
      else field+=c;
      continue;
    }
    if(c===','){cell();continue}
    if(c==='\n'){line();continue}
    if(c==='\r'&&text[i+1]==='\n'){line();i++;continue}
    if(closed)throw Error('characters after closing CSV quote');
    if(c==='"'){
      if(started)throw Error('quote inside unquoted CSV cell');
      quoted=true;started=true;continue;
    }
    if(c==='\r')throw Error('bare CSV carriage return');
    field+=c;started=true;
  }
  if(quoted)throw Error('unterminated CSV quote');
  if(started||closed||row.length)line();
  return rows;
}
