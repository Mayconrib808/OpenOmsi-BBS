import test from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
const source=(await readFile(new URL('../ui/js/pages/settings-schema.js',import.meta.url),'utf8'))
 .replace("import { t } from '../i18n.js';",'const t=x=>x;')
 .replace("import { optionalRows } from './optional-features.js';",'const optionalRows=()=>[];');
const schema=await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
function rows(x){if(Array.isArray(x))return x.flatMap(rows);if(!x||typeof x!=='object')return [];return [...(x.type?[x]:[]),...Object.values(x).flatMap(rows)];}
test('three upstream controls are unique and winter/per-map hailing controls are removed',()=>{
 const actual=rows(schema.SETTINGS_TABS);
 for(const key of ['passenger_hailing','passenger_connections','passenger_full_bus_reaction']) {
  const matches=actual.filter(r=>r.key===key);assert.equal(matches.length,1,key);assert.equal(matches[0].type,'toggle');
 }
 for(const key of ['weather_clothing','passenger_connections_enabled'])assert.equal(actual.filter(r=>r.key===key).length,0);
 assert.equal(actual.filter(r=>r.id==='passenger-maps').length,0);
});
