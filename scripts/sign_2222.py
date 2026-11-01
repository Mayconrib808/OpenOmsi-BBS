"""Sign 2.2.2.2 ONLY after a verified Windows candidate; never upload the publisher key."""
import argparse,base64,json,hashlib,re
from pathlib import Path
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey,Ed25519PublicKey
ROOT=Path(__file__).resolve().parents[1]
VERSION="2.2.2.2"
SEQUENCE=2026103105
PUBLIC=base64.b64decode("5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs=")
ORIGIN="https://openomsi-bbs-updates.maycongamesmaximo.workers.dev"
p=argparse.ArgumentParser()
p.add_argument("--publisher-key",type=Path,required=True)
p.add_argument("--candidate-report",type=Path,required=True)
p.add_argument("--candidate-run",type=int,required=True)
p.add_argument("--candidate-commit",required=True)
a=p.parse_args()
report=json.loads(a.candidate_report.read_text(encoding="utf-8-sig"))
meta=json.loads((ROOT/"build-support/release2222/native-build.json").read_text())
assert report["version"]==VERSION and report["sequence"]==SEQUENCE
assert report["native_candidate_run"]==meta["run_id"]
assert report["native_build_commit"]==meta["commit"]
assert report["native_version"]=="0.3.46-coop.15-2.2.2.2"
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
assert re.fullmatch("[a-f0-9]{64}",report["updater_sha256"])
assert 1_000_000<report["installer_size"]<=25<<20
assert a.candidate_run>0 and re.fullmatch("[0-9a-f]{40}",a.candidate_commit)
for name in ("HostAgent.exe","CompanyHost.exe"):
    assert report["retained_host_hashes"][name]==report["retained_runtime_sha256"][name]
    assert report["components_sha256"][name]==report["retained_host_hashes"][name]
previous=json.loads((ROOT/"build-support/release2221-hotfix2/stable.json").read_text())
signed_bytes=base64.b64decode(previous["payload"])
Ed25519PublicKey.from_public_bytes(PUBLIC).verify(base64.b64decode(previous["signature"]),signed_bytes)
prior=json.loads(signed_bytes)
assert prior["version"]=="2.2.2.1-hotfix.2" and prior["sequence"]==2026103104 and prior["sequence"]<SEQUENCE
key_data=json.loads(a.publisher_key.read_text(encoding="utf-8-sig"))
private=Ed25519PrivateKey.from_private_bytes(base64.b64decode(key_data["private_seed"],validate=True))
assert private.public_key().public_bytes_raw()==PUBLIC
assert key_data["last_sequence"]<SEQUENCE
notes={
    "pt":("2.2.2.2: roupas de inverno 3D reais para passageiros padrões, usando modelos de artistas. "
          "Até 15 °C, casacos/suéteres; até 5 °C (ou neve até 12 °C), também toucas e luvas. "
          "Opção desativável, limites de FPS; encaixe visual em partida ainda não confirmado. "
          "Logs aprimorados de ultrapassagens IA, preferência, estacionamento, conversas, áudio, "
          "densidade, roupas e eventos BCS: motivos, resultados e resumos host/cliente a cada 60 s. "
          "Setup abre o changelog acumulado, incluindo as hotfixes anteriores."),
    "en":("2.2.2.2: authentic artist-made winter clothing on stock passengers: jackets/sweaters "
          "at 15 C or below, hats and gloves at 5 C or below (or snow at 12 C or below). "
          "Optional and FPS-limited; in-game fitting has not been visually verified. "
          "Detailed host/client event evidence for AI passing, yielding, parking, conversations, "
          "audio, traffic density, climate clothing and BCS, with minute summaries. "
          "Setup shows cumulative release and hotfix notes."),
    "de":("2.2.2.2: echte Künstler-Winterkleidung für Standardfahrgäste. Jacken/Pullover "
          "bis 15 °C; Mützen/Handschuhe bis 5 °C oder bei Schnee bis 12 °C. "
          "Optional und begrenzt; sichtbare Passform im Spiel noch nicht bestätigt. "
          "Detaillierte Host-/Client-Protokolle zu Überholen, Vorfahrt, Falschparken, "
          "Gesprächen, Audio, Verkehr, Winterkleidung und BBS-Ereignissen. "
          "Änderungsverlauf einschließlich aller Hotfixes im Setup.")
}
out=ROOT/"build-support/release2222"
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
print(f"Signed validated Windows {VERSION} candidate; updater hash = {report['updater_sha256']}")
