"""Verify the exact updater payload against the candidate and retained baseline."""
import hashlib
import json
from pathlib import Path
import sys

from build_runtime_2203 import embedded_payload

sha = lambda data: hashlib.sha256(data).hexdigest()
candidate, baseline = map(Path, sys.argv[1:3])
report = json.loads((candidate / "build-report.json").read_text())
prior = json.loads((baseline / "build-report.json").read_text())
new_exe = (candidate / "Atualizar-Bridge.exe").read_bytes()
old_exe = (baseline / "Atualizar-Bridge.exe").read_bytes()
assert sha(old_exe) == "98ec07db801f028deb0874718aac3251fb6f44893dd6c96a38d4e792289ee338"
assert sha(new_exe) == report["updater_sha256"] and len(new_exe) == report["installer_size"]
assert report["setup_live_update_ui_tested"] and report["complete_host_ui_tested"]
assert report["localized_bcs_driver"] and report["setup_update_unsaved_root"]
old, new = embedded_payload(old_exe), embedded_payload(new_exe)
assert {name: sha(data) for name, data in old.items()} == prior["components_sha256"]
assert {name: sha(data) for name, data in new.items()} == report["components_sha256"]
for name, data in new.items():
    assert data == (candidate / "runtime" / name).read_bytes(), name
for name in ("HostAgent.exe", "CompanyHost.exe", "app/compat/omsi-plugin-host32.exe"):
    assert new[name] == old[name], name
assert json.loads((candidate / "coop-spec.json").read_text()) == json.loads((baseline / "coop-spec.json").read_text())
assert report["coop_archive_sha256"] == "bef8e67010ea9c53fe2b80684ae0b8727b7f18e3b9f0e14c31ea5c75ab406574"
assert report["coop_archive_sha256"].encode() in new_exe
print("Verified exact Windows-tested updater payload, localized runtime and unchanged complete hosts/game/servers")
