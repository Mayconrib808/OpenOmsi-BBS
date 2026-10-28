#!/usr/bin/env python3
"""Repackage pinned Hotfix 2 artifacts with BCS localized fixes for GitHub Releases."""
import hashlib
import io
import json
import struct
import zipfile
from pathlib import Path

sha = lambda b: hashlib.sha256(b).hexdigest()
base = Path("candidate")
patch = Path("localized")
out = Path("dist")
out.mkdir(exist_ok=True)
filename = "OpenOMSI_BBS_Bridge_2.2.1.9_Hotfix2_Localized_WebDisk.zip"
folder = "OpenOMSI-BBS-Bridge-2.2.1.9-Hotfix2-Localized/"
installer = (base / "Atualizar-Bridge.exe").read_bytes()
assert sha(installer) == "98ec07db801f028deb0874718aac3251fb6f44893dd6c96a38d4e792289ee338"
spec = json.loads((base / "coop-spec.json").read_text())
assert spec["sha256"] == "bef8e67010ea9c53fe2b80684ae0b8727b7f18e3b9f0e14c31ea5c75ab406574"
end = installer.rfind(b"PK\x05\x06")
assert end > 0
_, _, _, _, _, cs, co, comment = struct.unpack_from("<4s4H2IH", installer, end)
with zipfile.ZipFile(io.BytesIO(installer[end - cs - co:end + 22 + comment])) as z:
    files = {f.filename: z.read(f) for f in z.infolist()}
assert len(files) == 7
parts = []
for part in spec["parts"]:
    data = (base / "coop-assets" / (part["sha256"] + ".bin")).read_bytes()
    assert len(data) == part["size"] and sha(data) == part["sha256"]
    parts.append(data)
packed = b"".join(parts)
assert len(packed) == spec["size"] and sha(packed) == spec["sha256"]
with zipfile.ZipFile(io.BytesIO(packed)) as z:
    assets = {name: z.read(spec["prefix"] + name) for name in spec["files"]}
assert all(sha(b) == spec["files"][name] for name, b in assets.items())
files.update(assets)
files["app/server/openomsi.exe"] = files["app/server-dinamico-p15/openomsi.exe"]
files["app/server/steam_api64.dll"] = files["app/server-dinamico-p15/steam_api64.dll"]
report = json.loads((patch / "build-report.json").read_text())
assert report["bridge_version"] == "2.2.1.9-hotfix.2"
assert report["branch"] == "fix/bcs-localized-driver"
assert report["localized_bcs_driver"] and report["setup_update_unsaved_root"]
expected = {"Setup.exe", "PeDePeAdapter/openomsi.exe", "app/OpenOMSI_BCS_Bridge.exe", "app/compat/Omsi.exe"}
assert set(report["components"]) == expected
for name, item in report["components"].items():
    b = (patch / name).read_bytes()
    assert b.startswith(b"MZ") and len(b) == item["size"] and sha(b) == item["sha256"]
    files[name] = b
source = (patch / "bridge-source.zip").read_bytes()
with zipfile.ZipFile(io.BytesIO(source)) as z:
    assert z.testzip() is None
    assert "Simulador de Companhia de Autobuses" in z.read("source/driver.go").decode("utf8")
    assert "installedBCSJar(root)" in z.read("source/setup_updates_windows.go").decode("utf8")
    mit = z.read("LICENSE")
runtime = ["Setup.exe", "HostAgent.exe", "CompanyHost.exe", "PeDePeAdapter/openomsi.exe", "app/OpenOMSI_BCS_Bridge.exe", "app/compat/Omsi.exe", "app/compat/omsi-plugin-host32.exe"]
assert all(name in files for name in runtime)
manifest = "# Automatic Bridge update\n" + "".join(f"{sha(files[name])}  {name}\n" for name in sorted(runtime))
files["docs/PATCH-PLAYER-SHA256.txt"] = manifest.encode()
files["docs/PATCH-HOST-SHA256.txt"] = manifest.encode()
files["docs/SHA256.txt"] = "".join(f"{sha(files[name])}  {name}\n" for name in sorted(files) if name != "docs/SHA256.txt").encode()
assert "app/bridge.ini" not in files and "app/HostAgent.local.json" not in files
files["00-LEIA-ME-PTBR.txt"] = """OpenOMSI + BBS Bridge 2.2.1.9 Hotfix 2 — Localized BCS
Autor: Mayconrib808 — Windows — 28/10/2026

Pacote COMPLETO: Bridge, Setup, adaptador PeDePe, HostAgent,
CompanyHost, OpenOMSI cooperativo, servidores P14/P15, sons,
DLLs, código-fonte e licenças.

Adiciona suporte à identificação do motorista no BCS como
"Simulador de Companhia de Autobuses" ou "Bus Company Simulator",
preservando os dados do motorista. Corrige Atualizar agora quando
a pasta OMSI 2 foi digitada no Setup mas ainda não foi salva.
Mantém as correções da Hotfix 2 para cobrador, troca de motorista,
passageiros, sincronização e outros recursos.

Instalação:
1. Extraia a pasta COMPLETA em local fora da pasta OMSI 2.
2. Feche BCS, OMSI 2, OpenOMSI e hosts.
3. Abra Setup.exe; configure a pasta OMSI 2 e o OpenOMSI.
4. No BCS selecione OpenOMSI (Beta) e aponte para
   PeDePeAdapter/openomsi.exe deste pacote.
5. Salve/ative no Setup e inicie as viagens pelo BCS.

Ao atualizar, faça backup da instalação anterior e mantenha
app/bridge.ini, mapas e configurações da empresa. Não use o
atualizador antigo para instalar este pacote.
OMSI 2 e o BCS comercial não estão incluídos.
Esta compilação Windows passou, mas a viagem BCS de dois
computadores ainda não foi testada; edição experimental.
Códigos da Bridge em SOURCE-CODE/bridge-source.zip.
https://github.com/Mayconrib808/OpenOmsi-BBS
""".encode("utf8")
files["00-README-ENGLISH.txt"] = """OpenOMSI + BBS Bridge 2.2.1.9 Hotfix 2 — BCS Localization Fix
By Mayconrib808 — 28 October 2026 — Windows

Complete package: Setup, PeDePe adapter, Bridge, HostAgent,
CompanyHost, modified OpenOMSI Coop, native multiplayer servers
P14/P15, sounds, third-party notices and source code.

Extra fixes: recognizes both "Bus Company Simulator" and
"Simulador de Companhia de Autobuses" as the saved driver
identity in BCS, without changing driver data. Setup's Update
button now reads the OMSI 2 folder even before Save/Activate.
Includes all previous Hotfix 2 cashier, passenger and driver
handover improvements.

Installation: Extract the entire package OUTSIDE the OMSI 2
directory. Close BCS, OMSI and all hosts. Run Setup.exe, configure
the paths and select OpenOMSI (Beta) in BCS with this package's
PeDePeAdapter/openomsi.exe. Back up any old Bridge installation
and preserve personal app/bridge.ini and company configuration.

OMSI 2 and paid BCS are required separately, not included.
Windows build passed; no full two-PC BCS trip test for this
localized package. Experimental community release.
Source: SOURCE-CODE/bridge-source.zip
https://github.com/Mayconrib808/OpenOmsi-BBS
""".encode("utf8")
files["CHANGELOG-2.2.1.9-WEBDISK.txt"] = """2.2.1.9 Hotfix 2 — BCS localization edition — 28 October 2026
- Fixed localized BCS driver name detection.
- Fixed Setup Update button before configuration is saved.
- Recompiled Setup, PeDePeAdapter, app Bridge and compat launcher.
- Kept original Hotfix 2 HostAgent, CompanyHost, OpenOMSI
  cooperative multiplayer, P14/P15 servers and all other assets.
- Source, licenses and checksums included.
- Live two-PC BCS gameplay validation pending.
""".encode("utf8")
files["DESCRICAO-PARA-WEBDISK.txt"] = """OpenOMSI + BBS Bridge 2.2.1.9 Hotfix 2 — BCS Localization Fix

The OpenOMSI + BBS Bridge integrates Bus Company Simulator
(BCS/BBS) with openOMSI and optional company multiplayer.
This complete Windows package contains the PeDePe-compatible
adapter, Setup, hosts, modified cooperative game and servers.
It fixes the localized BCS driver name and Setup Update button
before the OMSI folder is saved, retaining Hotfix 2 improvements.
OMSI 2 and BCS are required separately. Experimental release.
""".encode("utf8")
files["LICENCAS-E-CODIGO-FONTE.txt"] = """Bridge code by Mayconrib808 under MIT License (LICENSE-MIT.txt).
OpenOMSI derivative sources, licenses and third-party notices are in
OpenOmsi Coop 0.3.41/.
Bridge source in SOURCE-CODE/bridge-source.zip.
The corrected source commit: 5752ae7669991712f55aa76c2acc323ba83c66fc.
""".encode("utf8")
files["LICENSE-MIT.txt"] = mit
files["SOURCE-CODE/bridge-source.zip"] = source
files["docs/BUILD-WEBDISK-LOCALIZED.json"] = (json.dumps({
    "version": "2.2.1.9-hotfix.2", "distribution": "localized BCS/Setup edition",
    "source_commit": "5752ae7669991712f55aa76c2acc323ba83c66fc",
    "base_candidate_run": 40765546864, "fixed_binary_run": 40781049699,
    "fixed_components": report["components"], "two_pc_bcs_gameplay_tested": False
}, indent=2) + "\n").encode("utf8")
files["SHA256-CONTEUDO-WEBDISK.txt"] = "".join(f"{sha(files[name])}  {name}\n" for name in sorted(files)).encode()
with zipfile.ZipFile(out / filename, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=5, allowZip64=True) as z:
    for name, b in sorted(files.items()):
        info = zipfile.ZipInfo(folder + name, date_time=(2026, 10, 28, 19, 0, 0))
        info.compress_type = zipfile.ZIP_DEFLATED
        info.external_attr = (0o100644 << 16)
        z.writestr(info, b, compress_type=zipfile.ZIP_DEFLATED, compresslevel=5)
with zipfile.ZipFile(out / filename) as z:
    assert len(z.namelist()) == len(files) and z.testzip() is None
    for name, b in files.items():
        assert sha(z.read(folder + name)) == sha(b), name
size = (out / filename).stat().st_size
assert 150_000_000 < size < 500_000_000
digest = sha((out / filename).read_bytes())
(out / (filename + ".sha256")).write_text(digest + "  " + filename + "\n")
print("ZIP:", filename, "files:", len(files), "size:", size, "SHA256:", digest)
