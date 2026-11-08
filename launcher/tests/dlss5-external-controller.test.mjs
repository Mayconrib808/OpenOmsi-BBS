// External NR controller is opt-in only: never silently download/inject into games.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const file = (path) => readFileSync(new URL(path, import.meta.url), 'utf8');
const backend = file('../src-tauri/src/nvidia_dlss5.rs');
const commands = file('../src-tauri/src/commands.rs');
const api = file('../ui/js/api.js');
const ui = file('../ui/js/pages/settings.js');

test('external helper is explicit, locally selected, and never injected by Bridge', () => {
  assert.match(backend, /fn valid_controller_path/);
  assert.match(backend, /path\.is_absolute\(\)/);
  assert.match(backend, /eq_ignore_ascii_case\("UniversalDLSS5\.exe"\)/);
  assert.match(backend, /pub fn select_controller\(path: String\)/);
  assert.match(backend, /pub fn open_controller\(\)/);
  assert.match(backend, /std::process::Command::new\(&exe\)/);
  assert.doesNotMatch(backend, /InjectDll|VirtualAllocEx|CreateRemoteThread|reqwest/);
  assert.match(backend, /"neural_rendering_active": false/);
});
test('launcher opens controller only on user click, never reports NR active', () => {
  assert.match(commands, /fn nvidia_dlss5_controller_select\(path: String\)/);
  assert.match(commands, /fn nvidia_dlss5_controller_open\(\)/);
  for (const name of ['nvidia_dlss5_controller_select', 'nvidia_dlss5_controller_open']) assert.match(api, new RegExp(name));
  assert.match(ui, /call\('pick_file'/);
  assert.match(ui, /onClick=\$\{openController\}/);
  assert.match(ui, /Neural Active/);
});
