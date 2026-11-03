#!/usr/bin/env python3
"""Offline publisher: sign the verified cumulative Hotfix 3 candidate. NEVER store a private key in GitHub.

The runner/deployer only receives the public, signed stable.json. This tool
produces two local files: publication.json (provenance) and stable.json (signature).
The caller commits publication.json first; only then may stable.json be committed
to trigger GitHub Actions production deployment.
"""
import argparse, base64, hashlib, json, zipfile
from pathlib import Path
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

VERSION = "2.2.2.4-hotfix.3"
SEQUENCE = 2026110206
NATIVE_COMMIT = "fa3d9197b64c7763d4c6e4819cca9fac1660ed04"
BASE = "https://openomsi-bbs-updates.maycongamesmaximo.workers.dev"
PUBLIC = "5xKCV4uIoc9e0/sGeF9s9+cd5m5rwPOdyqslWqxPRrs="
sha = lambda b: hashlib.sha256(b).hexdigest()


def run():
    ap = argparse.ArgumentParser()
    for k in ("artifact", "publisher-key", "candidate-run-id", "candidate-commit", "artifact-id", "artifact-digest", "out"):
        ap.add_argument("--" + k, required=True)
    a = ap.parse_args()
    arc = Path(a.artifact)
    archive_digest = sha(arc.read_bytes())
    assert archive_digest == a.artifact_digest.removeprefix("sha256:"), "Downloaded artifact digest does not match GitHub"
    with zipfile.ZipFile(arc) as z:
        report = json.loads(z.read("build-report.json"))
        spec = json.loads(z.read("coop-spec.json"))
        installation = json.loads(z.read("one-click-install-test.json"))
        host_ui = json.loads(z.read("native-p16-controls.json"))
        exe = z.read("Atualizar-Bridge.exe")
    assert report["version"] == VERSION and report["sequence"] == SEQUENCE
    assert report["updater_sha256"] == sha(exe) and report["installer_size"] == len(exe) and len(exe) <= 25*1024*1024
    assert report["coop_archive_sha256"] == spec["sha256"] and len(spec["parts"]) > 5
    assert report["native_candidate_run"] == int(a.candidate_run_id) and report["desktop_launcher_run"] == 41539618533
    assert report["native_candidate_commit"] == NATIVE_COMMIT and report["native_build_commit"] == NATIVE_COMMIT
    assert report["desktop_launcher_commit"] == "3d91881988bc260115184b5c4a03596f2b25d604"
    assert report["new_p16_host_ui_tested"] and report["coop_protocol"] == 16
    assert report["host_preferences_limit"] == 32<<20 and report["large_host_inventory_ui_tested"]
    assert host_ui["large_inventory_tested"] and host_ui["inventory_files"] == 22000
    assert report["p14_server_unchanged"] and report["p15_server_unchanged"] and report["complete_hosts_unchanged"]
    assert report["brazilian_passenger_recordings"] == 8 and report["custom_sounds_unchanged"]
    assert report["setup_changelog_languages"] == ["pt", "en", "de"]
    assert report["actual_updater_first_attempt_windows_tested"] and report["updater_architecture"] == "amd64"
    assert installation["status"] == "passed" and installation["invocations"] == 1
    assert installation["setup_reopened"] and installation["unmanaged_preferences_preserved"]
    assert installation["setup_sha256"] == report["components_sha256"]["Setup.exe"]
    assert report["windows_gameplay_tested"] is False and report["paired_gameplay_tested"] is False
    file = json.loads(Path(a.publisher_key).read_text())
    key = Ed25519PrivateKey.from_private_bytes(base64.b64decode(file["private_seed"]))
    raw_pub = key.public_key().public_bytes(serialization.Encoding.Raw, serialization.PublicFormat.Raw)
    assert base64.b64encode(raw_pub).decode() == file["public_key"] == PUBLIC, "WRONG PUBLISHER KEY"
    notes = {
  "pt": "2.2.2.4-hotfix.3 — Corrige host preferences exceed size limit ao salvar/iniciar empresas com inventarios grandes. Leitura, preservacao, salvamento e migracao usam o mesmo limite local de 32 MiB. Teste Windows salva e reabre 22 mil arquivos. Preserva mapas, chaves, voz, senhas e opcoes; mantem as correcoes de sinalizacao Original/Enhanced, SHA-256 das oito vozes V2 e P16 da Hotfix 2, com jogo e launcher nativos inalterados. Changelog acumulado em PT/EN/DE.",
  "en": "2.2.2.4-hotfix.3 — Fixes host preferences exceed size limit when saving/starting large company inventories. Reading, merging, saving and migration share a bounded 32 MiB local limit. Windows UI tests save/reopen 22,000 inventory files and preserve maps, keys, passwords, voice and opt-outs. Retains Hotfix 2 Original/Enhanced passenger hailing, eight approved V2 voice hashes and P16. Native game and launcher are unchanged; cumulative changelog included.",
  "de": "2.2.2.4-hotfix.3 — Behebt host preferences exceed size limit bei grossen Firmeninventaren. Lesen, Zusammenfuehren, Speichern und Migration nutzen dieselbe begrenzte lokale 32-MiB-Grenze. Windows-Tests speichern/oeffnen 22.000 Inventardateien. Karten, Schluessel, Passwoerter, Sprache und Optionen bleiben erhalten. Hotfix-2-Handzeichen, V2-Sprachpruefsummen und P16 bleiben erhalten; natives Spiel und Launcher unveraendert. Kumulatives Aenderungsprotokoll enthalten."
}
    pub = {
      "version": VERSION, "sequence": SEQUENCE,
      "candidate_run_id": int(a.candidate_run_id),
      "native_base_run_id": int(a.candidate_run_id),
      "native_build_commit": NATIVE_COMMIT,
      "launcher_base_run_id": 41539618533,
      "candidate_commit": a.candidate_commit,
      "candidate_artifact_id": int(a.artifact_id),
      "candidate_artifact_sha256": archive_digest,
      "installer_sha256": report["updater_sha256"],
      "installer_size": len(exe),
      "coop_archive_sha256": spec["sha256"],
    }
    manifest = {"schema":1, "version":VERSION,"sequence":SEQUENCE,"notes":notes,
                "installer":{"url":BASE+"/packages/"+sha(exe)+".exe","sha256":sha(exe),"size":len(exe)}}
    payload = json.dumps(manifest, ensure_ascii=False, separators=(",",":")).encode("utf8")
    signature = key.sign(payload)
    signed = {"payload":base64.b64encode(payload).decode(),"signature":base64.b64encode(signature).decode()}
    key.public_key().verify(signature,payload)
    output=Path(a.out);output.mkdir(parents=True,exist_ok=True)
    (output/"publication.json").write_text(json.dumps(pub,indent=2)+"\n",encoding="utf8")
    (output/"stable.json").write_text(json.dumps(signed,separators=(",",":"))+"\n",encoding="utf8")
    print("SIGNED CANDIDATE READY locally; not published. Candidate:",a.candidate_run_id,"SHA256:",archive_digest,"updater:",sha(exe))
    print("Commit publication.json first, then stable.json. NEVER commit publisher-key.json.")


if __name__ == "__main__":
    run()
