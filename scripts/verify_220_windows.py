#!/usr/bin/env python3
"""Exercise the new native GUI and actual Coop CLI using Bridge capability probes."""
import json
import os
from pathlib import Path
import subprocess
import tempfile

from build_220 import ROOT, PACKAGE
from test_openomsi_compat import build_probe

output = ROOT / 'build/release-220'
package = output / PACKAGE
evidence = output / 'evidence'
evidence.mkdir(exist_ok=True)
env = dict(os.environ, BRIDGE_SETUP_PREVIEWS=str(evidence), OMSI_NO_UPDATE='1', OMSI_NO_GUARD='1')
subprocess.run([str(package / 'Setup.exe'), '--gui-smoke'], env=env, check=True, timeout=40)
subprocess.run([str(output / 'Atualizar-Bridge.exe'), '--coop-progress-smoke'], env=env, check=True, timeout=20)
game = package / 'OpenOmsi Coop 0.3.38/programa/openomsi.exe'
checks = {}
for flag in ('--version', '--help', '--self-test'):
    run = subprocess.run([str(game), flag], cwd=game.parent, env=env, capture_output=True, text=True, encoding='utf-8', check=True, timeout=60)
    checks[flag] = {'exit_code': run.returncode, 'stdout': run.stdout, 'stderr': run.stderr}
    if flag == '--version' and '0.3.38-coop.6' not in run.stdout + run.stderr:
        raise ValueError('Unexpected Coop identity')
with tempfile.TemporaryDirectory(prefix='bridge220-probe-') as temporary:
    probe = build_probe('go', Path(temporary))
    checks['bridge_capability_probe'] = {}
    for mode in ('player', 'server'):
        result = subprocess.run([str(probe), str(game), mode], env=env, capture_output=True, text=True, encoding='utf-8', check=True, timeout=30)
        checks['bridge_capability_probe'][mode] = json.loads(result.stdout)
checks.update(setup_gui='passed', bundled_coop_button='passed', updater_progress='passed', gameplay_two_players_tested=False)
(evidence / 'windows-native.json').write_text(json.dumps(checks, indent=2) + '\n', encoding='utf-8')
print('Native Setup, included Coop selection, updater progress and real game capability probes passed.')
