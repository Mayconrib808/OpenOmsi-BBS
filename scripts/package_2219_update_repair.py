"""Create the repaired full ZIP from an authenticated localized package and candidate."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import subprocess
import zipfile

ROOT = Path(__file__).resolve().parents[1]
sha = lambda data: hashlib.sha256(data).hexdigest()
APPROVED_BASES = {
    "ef06c873d840f1267f3b78e9776dd6943cc3c472d5b07e4785e4c9c148863cac",
    "edab7b34e026da034ac8635859adafddea1b86f8e90023f23bbc7e3ca98974cd",
}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--basezip", type=Path, required=True)
    parser.add_argument("--candidate", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    data = args.basezip.read_bytes()
    assert sha(data) in APPROVED_BASES
    with zipfile.ZipFile(io.BytesIO(data)) as z:
        prefixes = {name.split("/")[0] for name in z.namelist()}
        assert len(prefixes) == 1
        prefix = prefixes.pop() + "/"
        files = {name[len(prefix):]: z.read(name) for name in z.namelist() if not name.endswith("/")}
    report = json.loads((args.candidate / "build-report.json").read_text())
    assert report["version"] == "2.2.1.9-hotfix.3"
    assert report["setup_live_update_ui_tested"] and report["complete_host_ui_tested"]
    native = {name: sha(data) for name, data in files.items() if name.startswith(("OpenOmsi Coop ", "app/server"))}
    assert native
    for name, digest in report["components_sha256"].items():
        data = (args.candidate / "runtime" / name).read_bytes()
        assert sha(data) == digest, name
        if name in report["retained_runtime_sha256"]:
            assert sha(files[name]) == digest, name
        files[name] = data
    installer = (args.candidate / "Atualizar-Bridge.exe").read_bytes()
    assert sha(installer) == report["updater_sha256"]
    files["Atualizar-Bridge.exe"] = installer
    # This launcher explicitly targets its own extracted folder, including
    # installations whose Setup configuration has not yet been saved.
    files["Reparar-Esta-Instalacao.cmd"] = b'@echo off\r\n"%~dp0Atualizar-Bridge.exe" --target-dir "%~dp0." --reopen\r\n'
    for name in ("docs/SHA256.txt", "SHA256-CONTEUDO-WEBDISK.txt", "SHA256-CONTEUDO.txt", "docs/BUILD-WEBDISK-LOCALIZED.json", "CHANGELOG-2.2.1.9-WEBDISK.txt", "DESCRICAO-PARA-WEBDISK.txt"):
        files.pop(name, None)
    source = subprocess.check_output(["git", "archive", "--format=zip", "HEAD", "source", "scripts", "LICENSE", "AGENTS.md", "build-support", "README.md", "README.pt-BR.md", "docs/releases"], cwd=ROOT)
    files["SOURCE-CODE/bridge-source.zip"] = source
    files["docs/BUILD-UPDATE-REPAIR.json"] = (json.dumps(report, indent=2) + "\n").encode()
    files["CHANGELOG-2.2.1.9-HOTFIX3.md"] = (ROOT / "docs/releases/2.2.1.9-hotfix.3.md").read_bytes()
    files["LICENCAS-E-CODIGO-FONTE.txt"] = b"Bridge code by Mayconrib808 under MIT License (LICENSE-MIT.txt).\nModified openOMSI sources, licenses and notices remain in OpenOmsi Coop 0.3.41/.\nThe repaired Bridge source and release scripts are in SOURCE-CODE/bridge-source.zip.\nThe Windows-tested candidate is b2c70838b3ab1bfaec49d9affd15e8634a225d61, Actions run 40901236438.\n"
    files["00-LEIA-ME-PTBR.txt"] = """OpenOMSI BCS Bridge 2.2.1.9 Hotfix 3 — 29/10/2026

Corrige "Distribuição aguardando configuração" e inclui a correção
de identificação do motorista no BCS com nome localizado
(rótulos Bus Company Simulator e Simulador de Companhia de Autobuses).
Buscar atualizações volta a consultar o serviço de atualização assinado.
Mantém o jogo cooperativo, sons, HostAgent/CompanyHost e servidores P14/P15.

Instalação nova: extraia a pasta completa fora do OMSI 2, abra Setup.exe,
selecione as pastas e clique em Salvar e ativar. No BCS, selecione
OpenOMSI (Beta) e PeDePeAdapter/openomsi.exe deste pacote.

Quem já instalou o ZIP com o erro: feche BCS, jogo, Setup e hosts.
Copie Atualizar-Bridge.exe e Reparar-Esta-Instalacao.cmd para a pasta
da Bridge que já usa (ao lado do Setup.exe) e execute o .cmd.
Ele aponta o atualizador explicitamente para essa instalação.
Depois abra o Setup e use Buscar atualizações normalmente.
As configurações pessoais, empresa e mapas são preservados.

Compilação e verificações automatizadas passaram no Windows, incluindo
a consulta real do Setup ao serviço assinado e os controles dos hosts.
Uma viagem BCS completa em dois PCs continua pendente de validação.
OMSI 2 e BCS comercial não são incluídos. Código e licenças acompanham.
Autor: Mayconrib808 — https://github.com/Mayconrib808/OpenOmsi-BBS
""".encode()
    files["00-README-ENGLISH.txt"] = """OpenOMSI BCS Bridge 2.2.1.9 Hotfix 3 — 29 October 2026

Fixes "Distribution awaiting configuration" in the localized Setup and
includes both Bus Company Simulator / Simulador de Companhia de Autobuses
driver identity support. Automatic and manual signed update checks work.
The modified game, complete hosts, sound and P14/P15 servers are retained.

New installation: extract the complete folder outside OMSI 2 and open Setup.exe.
Existing broken installation: close the games, Setup and hosts; copy
Atualizar-Bridge.exe and Reparar-Esta-Instalacao.cmd beside your existing
Setup.exe, then run the .cmd. Personal configuration and maps are preserved.

Windows compilation, live Setup update checks and actual host controls passed.
A full two-PC BCS trip remains untested. Requires separately licensed OMSI 2
and BCS. Source code, licenses and hashes are included.
""".encode()
    runtime = report["components_sha256"]
    manifest = "# Automatic Bridge update\n" + "".join(f"{digest}  {name}\n" for name, digest in sorted(runtime.items()))
    files["docs/PATCH-PLAYER-SHA256.txt"] = manifest.encode()
    files["docs/PATCH-HOST-SHA256.txt"] = manifest.encode()
    assert "app/bridge.ini" not in files and "app/HostAgent.local.json" not in files
    assert all(sha(files[name]) == digest for name, digest in native.items())
    files["docs/SHA256.txt"] = "".join(f"{sha(data)}  {name}\n" for name, data in sorted(files.items())).encode()
    files["SHA256-CONTEUDO.txt"] = "".join(f"{sha(data)}  {name}\n" for name, data in sorted(files.items())).encode()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    prefix = "OpenOMSI-BCS-Bridge-2.2.1.9-Hotfix3/"
    with zipfile.ZipFile(args.output, "x", zipfile.ZIP_DEFLATED, compresslevel=5) as z:
        for name, data in sorted(files.items()):
            entry = zipfile.ZipInfo(prefix + name, (2026, 10, 29, 0, 0, 0))
            entry.compress_type = zipfile.ZIP_DEFLATED
            entry.external_attr = 0o100644 << 16
            z.writestr(entry, data, compresslevel=5)
    with zipfile.ZipFile(args.output) as z:
        assert len(z.namelist()) == len(files) and z.testzip() is None
        for name, data in files.items():
            assert sha(z.read(prefix + name)) == sha(data), name
    args.output.with_name(args.output.name + ".sha256").write_text(sha(args.output.read_bytes()) + "  " + args.output.name + "\n")
    print("Complete package verified:", args.output.name, args.output.stat().st_size, sha(args.output.read_bytes()))


if __name__ == "__main__":
    main()
