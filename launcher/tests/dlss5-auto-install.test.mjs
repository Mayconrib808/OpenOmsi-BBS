// Ensure a third-party injector is downloaded only after informed opt-in.
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const file = (relative) => readFileSync(new URL(relative, import.meta.url), 'utf8');
const ps = file('../../scripts/windows/download-dlss5-controller.ps1');
const backend = file('../src-tauri/src/nvidia_dlss5.rs');
const commands = file('../src-tauri/src/commands.rs');
const api = file('../ui/js/api.js');
const settings = file('../ui/js/pages/settings.js');

test('controller downloader pins an exact upstream release and validates SHA256', () => {
  assert.match(ps, /MotionflowOffical\/UniversalDLSS5\/releases\/download\/v0\.5\.1\/UniversalDLSS5-0\.5\.1-Setup-x64\.exe/);
  assert.match(ps, /0ded01f5509a5cbb427806e1574d0f9ed21e200320191022677ec82e66916231/);
  assert.match(ps, /Get-FileHash -LiteralPath \$Path -Algorithm SHA256/);
  assert.match(ps, /Test-VerifiedInstaller \$Temp/);
  assert.match(ps, /Test-VerifiedInstaller \$Destination/);
  assert.doesNotMatch(ps, /nvngx_dlssnr\.dll|CreateRemoteThread|VirtualAllocEx/);
});
test('installer is explicit and NVIDIA runtime remains separate', () => {
  assert.match(backend, /pub fn install_controller\(accepted_terms: bool\)/);
  assert.match(backend, /if !accepted_terms/);
  assert.match(backend, /fn selected_controller\(\)/);
  assert.match(backend, /include_str!\(".*download-dlss5-controller\.ps1"\)/);
  assert.match(backend, /"neural_rendering_active"\] = json!\(false\)/);
  assert.match(commands, /fn nvidia_dlss5_controller_install\(accepted_terms: bool\)/);
  assert.match(api, /'nvidia_dlss5_controller_install'/);
  assert.match(settings, /acceptedTerms: true/);
  assert.match(settings, /controllerAccepted/);
  assert.match(settings, /NVIDIA Neural Rendering runtime is NOT included/);
  assert.match(settings, /onClick=\$\{installController\}/);
});
