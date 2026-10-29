"""Replace the public full download and expose a repair for the broken Setup."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import zipfile

sha = lambda data: hashlib.sha256(data).hexdigest()


def gh(*args):
    return subprocess.check_output(["gh", *args], text=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--candidate", type=Path, required=True)
    parser.add_argument("--dist", type=Path, required=True)
    args = parser.parse_args()
    repo = os.environ["GITHUB_REPOSITORY"]
    tag = "v2.2.1.9"
    full = args.dist / "OpenOMSI_BCS_Bridge_2.2.1.9.zip"
    assert full.is_file()
    report = json.loads((args.candidate / "build-report.json").read_text())
    assert report["setup_live_update_ui_tested"] and report["complete_host_ui_tested"]
    updater = args.dist / "Atualizar-Bridge.exe"
    updater.write_bytes((args.candidate / "Atualizar-Bridge.exe").read_bytes())
    assert sha(updater.read_bytes()) == report["updater_sha256"]
    updater.with_name(updater.name + ".sha256").write_text(sha(updater.read_bytes()) + "  " + updater.name + "\n")
    repair = args.dist / "Reparar-Atualizacoes-2.2.1.9-Hotfix3.zip"
    with zipfile.ZipFile(full) as z:
        prefix = z.namelist()[0].split("/")[0] + "/"
        launcher = z.read(prefix + "Reparar-Esta-Instalacao.cmd")
        instructions = z.read(prefix + "00-LEIA-ME-PTBR.txt")
    with zipfile.ZipFile(repair, "x", zipfile.ZIP_DEFLATED, compresslevel=5) as z:
        z.writestr("Atualizar-Bridge.exe", updater.read_bytes())
        z.writestr("Reparar-Esta-Instalacao.cmd", launcher)
        z.writestr("LEIA-ME.txt", instructions)
    details = json.loads(gh("api", f"repos/{repo}/releases/tags/{tag}"))
    assert details["id"] == 427273484 and not details["draft"]
    body = details["body"] or ""
    marker = "## Correção urgente — Hotfix 3 (29/10/2026)"
    if body.startswith(marker):
        body = body.split("\n\n---\n\n", 1)[1]
    body = body.replace("The ZIP contains changes through 2.2.1.9 Hotfix 2, plus", "The ZIP contains changes through 2.2.1.9 Hotfix 3, plus")
    repair_url = f"https://github.com/{repo}/releases/download/{tag}/{repair.name}"
    prefix = f"""{marker}

**O ZIP completo abaixo já contém a Hotfix 3.** Corrige a mensagem **Distribuição aguardando configuração** no Setup, restaura a consulta automática/manual ao serviço assinado e inclui a correção dos nomes de motorista do BCS **Bus Company Simulator / Simulador de Companhia de Autobuses**. O serviço **Buscar atualizações** também foi atualizado para esta correção.

**Quem já baixou o pacote com a mensagem:** baixe [{repair.name}]({repair_url}), feche o BCS, o jogo, o Setup e os hosts, e extraia os arquivos na pasta da Bridge que você já usa, ao lado do Setup.exe. Execute **Reparar-Esta-Instalacao.cmd**. Ele aponta o atualizador para essa pasta; depois, Buscar atualizações funciona normalmente. A configuração pessoal é preservada.

O jogo cooperativo Hotfix 2, os sons, HostAgent/CompanyHost completos e os servidores P14/P15 foram mantidos com os mesmos hashes. A referência antiga ao pacote Coop no gerador do atualizador também foi corrigida. As verificações no Windows incluíram as consultas reais do Setup ao serviço assinado e os controles dos hosts. Uma viagem BCS completa em dois PCs continua pendente de validação.

**English:** The complete ZIP now includes Hotfix 3. It repairs the localized Setup's update configuration and includes the localized BCS driver correction. Existing affected users can extract the small repair ZIP beside their current Setup.exe and run Reparar-Esta-Instalacao.cmd with all Bridge/game/host programs closed.

---

"""
    notes = args.dist / "RELEASE-NOTES-HOTFIX3.md"
    notes.write_text(prefix + body)
    uploads = [full, full.with_name(full.name + ".sha256"), updater, updater.with_name(updater.name + ".sha256"), repair]
    subprocess.run(["gh", "release", "upload", tag, *map(str, uploads), "--repo", repo, "--clobber"], check=True)
    subprocess.run(["gh", "release", "edit", tag, "--repo", repo, "--title", "OpenOMSI BCS Bridge 2.2.1.9", "--notes-file", str(notes), "--latest"], check=True)
    updated = json.loads(gh("api", f"repos/{repo}/releases/tags/{tag}"))
    evidence = {"release_url": updated["html_url"], "files": [], "candidate_run": 40901236438}
    for path in uploads:
        data = path.read_bytes()
        asset = next(item for item in updated["assets"] if item["name"] == path.name)
        assert asset["size"] == len(data) and asset["state"] == "uploaded", path.name
        assert asset["digest"] == "sha256:" + sha(data), path.name
        evidence["files"].append({"name": path.name, "size": len(data), "sha256": sha(data), "url": asset["browser_download_url"]})
    with tempfile.TemporaryDirectory() as temp:
        subprocess.run(["gh", "release", "download", tag, "--repo", repo, "--pattern", full.name, "--dir", temp], check=True)
        assert sha((Path(temp) / full.name).read_bytes()) == sha(full.read_bytes())
    (args.dist / "publication-evidence.json").write_text(json.dumps(evidence, indent=2) + "\n")
    print("VERIFIED: complete GitHub ZIP replaced and re-downloaded with matching SHA-256; repair ZIP and manual updater published")


if __name__ == "__main__":
    main()
