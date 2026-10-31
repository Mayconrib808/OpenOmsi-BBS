import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const source = await readFile(new URL('../ui/js/pages/optional-features.js', import.meta.url), 'utf8');
const options = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
const schemaSource = await readFile(new URL('../ui/js/pages/settings-schema.js', import.meta.url), 'utf8');
const executableSchema = schemaSource
  .replace("import { t } from '../i18n.js';", 'const t = (text) => text;')
  .replace("import { optionalRows } from './optional-features.js';", `const rows = ${JSON.stringify(options.OPTIONAL_FEATURE_ROWS)}; const optionalRows = (traffic) => rows.filter((r) => (r.key === 'extra_traffic_density' || r.key === 'ai_bus_greetings') === traffic);`);
const schema = await import(`data:text/javascript;base64,${Buffer.from(executableSchema).toString('base64')}`);

function rows(value) {
  if (Array.isArray(value)) return value.flatMap(rows);
  if (!value || typeof value !== 'object') return [];
  return [...(value.type ? [value] : []), ...Object.values(value).flatMap(rows)];
}

test('all seven optional settings are real, unique toggles in the launcher settings schema', () => {
  const actual = rows(schema.SETTINGS_TABS);
  assert.equal(options.OPTIONAL_FEATURE_ROWS.length, 7);
  assert.equal(new Set(options.OPTIONAL_FEATURE_ROWS.map((r) => r.key)).size, 7);
  for (const option of options.OPTIONAL_FEATURE_ROWS) {
    const matches = actual.filter((r) => r.key === option.key);
    assert.equal(matches.length, 1, `missing or duplicated launcher switch ${option.key}`);
    assert.equal(matches[0].type, 'toggle');
    assert.equal(matches[0].label, option.label);
    assert.equal(matches[0].help, option.help);
    assert.ok(option.help.includes('Restart') || option.help.includes('next time'));
  }
});

test('traffic options are kept separate and the transfer percentage retains its normal slider', () => {
  assert.deepEqual(options.optionalRows(true).map((r) => r.key), ['extra_traffic_density', 'ai_bus_greetings']);
  assert.equal(options.optionalRows(false).length, 5);
  const actual = rows(schema.SETTINGS_TABS);
  const chance = actual.find((r) => r.key === 'passenger_transfer_percent');
  assert.equal(chance.type, 'slider');
  assert.equal(chance.label, 'Connection chance');
  assert.equal(chance.def, 20);
});
