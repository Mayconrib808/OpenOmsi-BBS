"""Sign 2.2.2.2-test.1 ONLY after verified Windows CI with the official local Ed25519 key. Never upload the private key."""
import argparse,base64,json,hashlib,re,subprocess
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey,Ed25519PublicKey
ROOT=Path(__file__).resolve().parents[1]
VERSION="2.2.2.2-test.1"
SEQUENCE=2026103107
PUBLIC=base64.b64decode("5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=")
ORIGIN="https://openomsi-bbs-updates.maycongamesmaximo.workers.dev"
p=argparse.ArgumentParser()
p.add_argument("--publisher-key",type=Path,required=True)
p.add_argument("--candidate-report",type=Path,required=True)
p.add_argument("--candidate-installer",type=Path,required=True)
p.add_argument("--candidate-spec",type=Path,required=True)
p.add_argument("--candidate-run",type=int,required=True)
p.add_argument("--candidate-commit",required=True)
a=p.parse_args()
report=json.loads(a.candidate_report.read_text(encoding="utf-8-sig"))
meta=json.loads((ROOT/"build-support/release2222-test1/native-build.json").read_text())
assert not a.publisher_key.resolve().is_relative_to(ROOT.resolve()),"Publisher key must stay outside the Git repository"
run=json.loads(subprocess.check_output(["gh","api",f"repos/Mayconrib808/OpenOmsi-BBS/actions/runs/{a.candidate_run}"],text=True))
assert run["status"]=="completed" and run["conclusion"]=="success" and run["head_sha"]==a.candidate_commit and run["path"]==".github/workflows/release-2222-test1-candidate.yml","Candidate CI must be successful and exact"
assert report["version"]==VERSION and report["sequence"]==SEQUENCE
assert report["native_candidate_run"]==meta["run_id"]
assert report["native_build_commit"]==meta["commit"]
assert report["native_version"]=="0.3.46-coop.15-2.2.2.2-test.1"
assert report["native_build_commit"]==meta["commit"] and report["native_candidate_commit"]==meta["commit"]
assert report["coop_protocol"]==15
assert report["winter_real_artist_assets_included"] is True
assert report["feature_event_audit_included"] is True
assert report["per_release_changelog_including_hotfixes"] is True
assert report["setup_changelog_ui_tested"] is True
assert report["setup_live_update_ui_tested"] is True
assert report["complete_hosts_unchanged"] and not report["complete_hosts_rebuilt"]
assert report["p14_server_unchanged"] and report["bridge_regressions_tested"]
assert report["native_client_sha256"]["openomsi.exe"]==report["dynamic_p15_sha256"]
assert report["desktop_launcher_run"]==41260965494 and not report["desktop_launcher_unchanged"]
assert not report["windows_gameplay_tested"] and not report["paired_gameplay_tested"]
assert report["winter_test_preview_opt_in"] is True and report["winter_visual_approved"] is False
assert report["setup_live_update_ui_tested"] is True
binary=a.candidate_installer.read_bytes()
spec=json.loads(a.candidate_spec.read_text(encoding="utf-8-sig"))
assert hashlib.sha256(binary).hexdigest()==report["updater_sha256"] and len(binary)==report["installer_size"]
assert spec["sha256"]==report["coop_archive_sha256"]
assert spec["files"]["OpenOmsi Coop 0.3.41/programa/openomsi.exe"]==spec["files"]["app/server-dinamico-p15/openomsi.exe"]==report["dynamic_p15_sha256"]
assert re.fullmatch("[a-f0-9]{64}",report["updater_sha256"])
assert 1_000_000<report["installer_size"]<=25<<20
assert a.candidate_run>0 and re.fullmatch("[0-9a-f]{40}",a.candidate_commit)
for name in ("HostAgent.exe","CompanyHost.exe"):
    assert report["retained_host_hashes"][name]==report["retained_runtime_sha256"][name]
    assert report["components_sha256"][name]==report["retained_host_hashes"][name]
previous=json.loads((ROOT/"build-support/release2222-hotfix1/stable.json").read_text())
signed_bytes=base64.b64decode(previous["payload"])
Ed25519PublicKey.from_public_bytes(PUBLIC).verify(base64.b64decode(previous["signature"]),signed_bytes)
prior=json.loads(signed_bytes)
assert prior["version"]=="2.2.2.2-hotfix.1" and prior["sequence"]==2026103106 and prior["sequence"]<SEQUENCE
key_data=json.loads(a.publisher_key.read_text(encoding="utf-8-sig"))
private=Ed25519PrivateKey.from_private_bytes(base64.b64decode(key_data["private_seed"],validate=True))
assert private.public_key().public_bytes_raw()==PUBLIC
assert key_data["last_sequence"]<SEQUENCE
notes={
    "pt":"TESTE EXPERIMENTAL 2.2.2.2-test.1: prévia opt-in de sete roupas de frio 3D. Ativar manualmente weather_clothing; proteção contra deformações permanece. O encaixe visual em partida NÃO foi aprovado. Preserva P15, BCS, 14 configurações e changelog acumulado de versões e hotfixes.",
    "en":"EXPERIMENTAL TEST 2.2.2.2-test.1: opt-in preview of seven 3D winter garments. Enable weather_clothing manually; deformation guards remain. In-game visual fitting is NOT approved. P15, BCS, 14 settings and cumulative version/hotfix changelog are retained.",
    "de":"EXPERIMENTELLER TEST 2.2.2.2-test.1: freiwillige Vorschau von sieben 3D-Winterkleidungsstücken. weather_clothing manuell aktivieren; Schutz vor Verformungen bleibt. Die sichtbare Passform im Spiel ist NICHT freigegeben. P15, BBS, 14 Einstellungen und kumulativer Änderungsverlauf bleiben erhalten."
}
out=ROOT/"build-support/release2222-test1"
out.mkdir(parents=True,exist_ok=True)
payload={
    "schema":1,"version":VERSION,"sequence":SEQUENCE,"notes":notes,
    "installer":{
        "url":ORIGIN+"/packages/"+report["updater_sha256"]+".exe",
        "sha256":report["updater_sha256"],"size":report["installer_size"]
    }
}
raw=json.dumps(payload,ensure_ascii=False,separators=(",",":")).encode()
envelope={
    "payload":base64.b64encode(raw).decode(),
    "signature":base64.b64encode(private.sign(raw)).decode(),
}
(out/"stable.json").write_text(json.dumps(envelope,separators=(",",":"))+"\n")
publication=dict(version=VERSION,sequence=SEQUENCE,candidate_run_id=a.candidate_run,candidate_commit=a.candidate_commit,
    installer_sha256=report["updater_sha256"],installer_size=report["installer_size"],coop_archive_sha256=report["coop_archive_sha256"])
(out/"publication.json").write_text(json.dumps(publication,indent=2)+"\n")
print(f"Signed verified EXPERIMENTAL Windows {VERSION} candidate; updater SHA256 = {report['updater_sha256']}")
