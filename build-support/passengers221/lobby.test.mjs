import fs from 'node:fs';import assert from 'node:assert/strict';
const src=fs.readFileSync((process.argv[2]||'game')+'/launcher/ui/js/pages/passenger-settings.js','utf8');
const {mapKey,enabledMaps,withMap}=await import('data:text/javascript;base64,'+Buffer.from(src).toString('base64'));
assert.equal(mapKey('F:\\OMSI\\maps\\SP_Carrao City\\GLOBAL.CFG'),'spcarraocity');
assert.equal(mapKey('Vale_do_Aço_2025'),'valedoaco2025');
assert.ok(enabledMaps('Grundorf;SP_Carrao City').has(mapKey('maps/SP Carrao City/global.cfg')));
assert.equal(withMap('Grundorf;SP_Carrao City','SP Carrao City',false),'Grundorf');
assert.equal(withMap('custom','Vilarejo 5.5',true),'custom;vilarejo55');
console.log('2.2.1 installed-map aliases and independent choices passed.');
