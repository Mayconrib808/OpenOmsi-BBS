// DLSS 5 is not the already installed DLSS Super Resolution runtime.
// The launcher may report RTX 50 readiness but MUST NOT fake a render stage.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const file = (path) => readFileSync(new URL(path, import.meta.url), 'utf8');
const ui = file('../ui/js/pages/settings.js');
const schema = file('../ui/js/pages/settings-schema.js');
const api = file('../ui/js/api.js');
const commands = file('../src-tauri/src/commands.rs');
const backend = file('../src-tauri/src/nvidia_dlss5.rs');
const gpu = file('../../crates/omsi-cfg/src/upscaler_files.rs');

 test('DLSS 5 is an independent, read-only renderer feature, not an upscaler alias', () => {
  assert.match(schema, /id: 'nvidia_dlss5'.*windows/);
  assert.doesNotMatch(schema, /\['dlss5',/);
  assert.match(ui, /function NvidiaDlss5\(\)/);
  assert.match(ui, /DLSS 5 is not yet integrated/);
  assert.doesNotMatch(ui, /nvidia_dlss5_set_enabled|nvidia_dlss5_install/);
});

test('hardware readiness allows GeForce RTX 50 only, and fails closed', () => {
  assert.match(gpu, /fn dlss5_candidate\(vendor: u32, name: &str\)/);
  assert.match(gpu, /vendor == 0x10de && name\.to_ascii_uppercase\(\)\.contains\("GEFORCE RTX 50"\)/);
  assert.match(backend, /"sdk_ready": false/);
  assert.match(backend, /"renderer_integrated": false/);
  assert.match(backend, /"active": false/);
  assert.match(commands, /fn nvidia_dlss5_status\(\)/);
  assert.match(api, /'nvidia_dlss5_status'/);
});
