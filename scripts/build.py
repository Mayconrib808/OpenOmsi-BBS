#!/usr/bin/env python3
"""Test and assemble a complete source-backed Windows x86 bridge package."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile

import setup_resources
import plugin_host_resources
import build_native_server

ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "source"
REFERENCE_GO = "go1.23.2"
VERSION = re.search(r'const bridgeVersion = "([^"]+)"', (SOURCE / "version.go").read_text()).group(1)
PACKAGE_NAME = f"OpenOmsi.+.BBS.{VERSION}"
TEST_COMMON = (
    "main.go pedepe_native.go log_text.go weather.go startup_ack.go timetable.go diagnostics.go launch_checks.go facade_memory.go launch_session.go session_transition.go openomsi_compatibility.go company_host_share.go setup_gui_model.go setup_gui_text.go "
    "driver.go session.go config.go plugin_host.go registry.go setup_files.go "
    "paths.go version.go company.go company_content.go company_clock.go company_directory.go company_host.go company_gateway.go company_gateway_protocol.go company_gateway_ws.go company_host_tunnel.go host_config.go host_agent.go multiplayer.go setup_ui.go company_setup.go profile_store.go"
).split()
TEST_FILES = (
    "driver_test.go session_test.go setup_test.go diagnostics_test.go launch_session_test.go session_transition_test.go openomsi_compatibility_test.go company_host_share_test.go setup_gui_model_test.go weather_test.go startup_ack_test.go "
    "timetable_test.go timetable_endpoints_test.go regression_test.go pedepe_native_test.go "
    "launch_checks_test.go company_test.go company_content_test.go company_clock_test.go company_directory_test.go host_agent_test.go multiplayer_startup_test.go multiplayer_test.go multiplayer_clock_test.go company_host_test.go company_gateway_test.go company_gateway_native_test.go profile_store_test.go"
).split()
PROGRAMS = {
    "HostAgent.exe": (
        "host_agent_main.go host_agent.go host_config.go host_gui_windows.go host_gui_preview_windows.go company_directory.go weather.go log_text.go company_host.go company_gateway.go company_gateway_protocol.go company_gateway_ws.go company_host_tunnel.go company_host_share.go company_host_process_windows.go company.go company_content.go company_clock.go openomsi_compatibility.go openomsi_probe_windows.go version.go"
    ).split(),
    "CompanyHost.exe": (
        "company_host_main.go company_host.go company_gateway.go company_gateway_protocol.go company_gateway_ws.go company_host_tunnel.go company_host_share.go company_host_process_windows.go company.go company_directory.go weather.go log_text.go company_content.go company_clock.go openomsi_compatibility.go openomsi_probe_windows.go version.go"
    ).split(),
    "Setup.exe": (
         "setup_main.go setup_files.go diagnostics.go setup_windows.go setup_gui_windows.go setup_gui_preview_windows.go setup_gui_model.go setup_gui_text.go openomsi_compatibility.go openomsi_probe_windows.go "
        "config.go plugin_host.go registry.go paths.go version.go company.go company_directory.go weather.go log_text.go company_content.go company_clock.go setup_ui.go company_setup.go profile_store.go"
    ).split(),
    "PeDePeAdapter/openomsi.exe": (
        "pedepe_adapter_main.go pedepe_adapter_windows.go pedepe_native.go driver.go config.go plugin_host.go paths.go version.go company.go company_directory.go company_content.go company_clock.go multiplayer.go profile_store.go weather.go log_text.go openomsi_compatibility.go openomsi_probe_windows.go"
    ).split(),
    "app/OpenOMSI_BCS_Bridge.exe": (
         "main.go log_text.go weather.go startup_ack.go timetable.go diagnostics.go launch_checks.go driver.go session.go launch_session.go session_transition.go openomsi_compatibility.go openomsi_probe_windows.go company_host_process_windows.go "
        "config.go plugin_host.go paths.go version.go process_windows.go company.go company_directory.go company_content.go company_clock.go multiplayer.go profile_store.go"
    ).split(),
    "app/compat/Omsi.exe": (
        "compat.go log_text.go startup_ack.go facade_memory.go driver.go version.go"
    ).split(),
}
HOST = "app/compat/omsi-plugin-host32.exe"
HOST_SOURCES = ["pluginhost/" + name for name in (
    "main.go", "wire.go", "environment.go", "dll_windows.go"
)]


def run(go: str, args: list[str], env: dict[str, str], cwd: Path = SOURCE) -> None:
    print("+ go " + " ".join(args), flush=True)
    subprocess.run([go, *args], cwd=cwd, env=env, check=True)


def pin_release_build_id(path: Path, relative_path: str) -> None:
    """Restore tested Go build metadata, then require the entire tested binary."""
    release = ROOT / "docs/releases"
    metadata = release / f"v{VERSION}-buildids.json"
    if not metadata.is_file():
        return
    identifiers = json.loads(metadata.read_text(encoding="utf-8"))
    if relative_path not in identifiers:
        return
    data = path.read_bytes()
    matches = list(re.finditer(rb'Go build ID: "([^"\n]+)"', data))
    wanted = identifiers[relative_path].encode("ascii")
    if len(matches) != 1 or len(matches[0].group(1)) != len(wanted):
        raise ValueError(f"Unexpected Go build ID layout: {relative_path}")
    start, end = matches[0].span(1)
    normalized = data[:start] + wanted + data[end:]
    approved = dict((name, digest) for digest, name in (
        line.split("  ", 1)
        for line in (release / f"v{VERSION}-binaries.sha256").read_text(encoding="utf-8").splitlines()
        if line
    ))
    if hashlib.sha256(normalized).hexdigest() != approved[relative_path]:
        raise ValueError(f"Compiled code differs from the approved release: {relative_path}")
    path.write_bytes(normalized)
    print(f"Exact live-tested executable restored after Go build-ID normalization: {relative_path}", flush=True)


def assemble(stage: Path, host_hash: str) -> None:
    ignore = shutil.ignore_patterns("__pycache__", "*.pyc", "*.exe", "*.dll", "*.obj", "*.lib", "*.exp")
    for name in ("source", "docs", "scripts", "examples", "relay", "native"):
        shutil.copytree(ROOT / name, stage / name, ignore=ignore)
    for p in ROOT.glob("*.md"):
        shutil.copy2(p, stage / p.name)
    for name in ("LICENSE", "LEIA-ME.txt", "TUTORIAL.html"):
        shutil.copy2(ROOT / name, stage / name)
    shutil.copy2(ROOT / "app/bridge.ini.example", stage / "app/bridge.ini.example")
    (stage / "docs/BUILD-INFO.txt").write_text(
        f"OpenOMSI BCS Bridge v{VERSION} - by Mayconrib808\n"
        f"Toolchain: {REFERENCE_GO}; GOOS=windows; GOARCH=386; GO386=sse2; CGO_ENABLED=0\n"
        "The seven bridge executables are built from the Go source included in this package.\n"
        "PeDePeAdapter/openomsi.exe is a transparent drop-in for PeDePe's official OpenOMSI (Beta) path and only adds company multiplayer when needed.\n"
        "app/server is a modified dedicated openOMSI server built from the pinned upstream source and native/free-player-vehicles.patch.\n"
        "Its native/bbs-server.json counterpart in app/server records compiler, tested commit and file hashes.\n"
        "HostAgent/CompanyHost also adapt official protocol-6 servers, including 0.2.23, without modifying those binaries.\n"
        "The adapter owns per-map UDP/WebSocket transports and tunnels; private player buses use the native stand-in.\n"
        "The plugin host includes Windows version/product/icon resources and retains normal Go symbols/build identity.\n"
        f"Plugin host SHA-256: {host_hash}\n"
        "The historical host with unresolved provenance is not bundled.\n"
        "Integration uses executable capabilities and multiplayer protocol 6. Official 0.2.0 and 0.2.11 metadata are checked on Windows CI.\n"
        "Automated checks do not certify a real vendor-plugin session or trip evaluation.\n"
        "Live PeDePe native trip/evaluation, next-trip, vendor panel and two-player full-game validation are pending.\n"
        "See docs/VALIDATION.md and docs/TEST_ON_WINDOWS.md.\n",
        encoding="utf-8",
    )
    inventory = stage / "docs/PACKAGE_FILES.txt"
    paths = sorted(p.relative_to(stage).as_posix() for p in stage.rglob("*") if p.is_file())
    paths.append("docs/PACKAGE_FILES.txt")
    inventory.write_text("\n".join(sorted(paths)) + "\n", encoding="utf-8")
    manifest = "".join(
        hashlib.sha256((stage / rel).read_bytes()).hexdigest() + "  " + rel + "\n"
        for rel in sorted(paths)
    )
    (stage / "docs/SHA256.txt").write_text(manifest, encoding="utf-8")


def archive_package(directory: Path) -> Path:
    target = directory.with_name(directory.name + ".zip")
    with zipfile.ZipFile(target, "x", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for p in sorted(directory.rglob("*")):
            if not p.is_file():
                continue
            entry = zipfile.ZipInfo(p.relative_to(directory).as_posix(), date_time=(2026, 10, 6, 0, 0, 0))
            entry.compress_type = zipfile.ZIP_DEFLATED
            entry.external_attr = (0o100644 << 16)
            archive.writestr(entry, p.read_bytes(), compresslevel=9)
    digest = hashlib.sha256(target.read_bytes()).hexdigest()
    target.with_name(target.name + ".sha256").write_text(f"{digest}  {target.name}\n", encoding="utf-8")
    print(f"Complete package: {target}\nZIP SHA-256: {digest}")
    return target


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check-only", action="store_true", help="Run complete build/checks in a temporary folder.")
    parser.add_argument("--native-server", type=Path, help="Verified dedicated server directory built by build_native_server.py.")
    parser.add_argument("--output", type=Path, default=ROOT / "dist" / PACKAGE_NAME,
                        help="New full-package directory. A ZIP and checksum are written beside it.")
    args = parser.parse_args()
    if not args.check_only and not args.native_server:
        parser.error("The complete release requires --native-server from the tested native build.")
    if args.native_server:
        build_native_server.verify_package(args.native_server)
    if output := os.environ.get("GITHUB_OUTPUT"):
        with Path(output).open("a", encoding="utf-8") as stream:
            stream.write(f"version={VERSION}\n")
    go = shutil.which("go")
    if go is None:
        parser.error("Install Go 1.23.2 and put its bin directory on PATH.")
    native = os.environ.copy()
    native.update(GO111MODULE="off", CGO_ENABLED="0", GOTOOLCHAIN="local", GOEXPERIMENT="")
    native.pop("GOOS", None)
    native.pop("GOARCH", None)
    version = subprocess.check_output([go, "version"], env=native, text=True).strip()
    if version.split()[2] != REFERENCE_GO:
        parser.error(f"This facade layout uses {REFERENCE_GO}; found {version}.")
    if not args.check_only and any(p.exists() for p in (
        args.output, args.output.with_name(args.output.name + ".zip"),
        args.output.with_name(args.output.name + ".zip.sha256"),
    )):
        parser.error("The output directory/ZIP/checksum already exists; choose a new --output path.")
    platform_file = "process_windows.go" if os.name == "nt" else "process_teststub_test.go"
    host_platform_file = "company_host_process_windows.go" if os.name == "nt" else "company_host_process_other.go"
    probe_platform_file = "openomsi_probe_windows.go" if os.name == "nt" else "openomsi_probe_other.go"
    test_sources = [*TEST_COMMON, platform_file, host_platform_file, probe_platform_file, *TEST_FILES]
    if os.name == "nt":
        test_sources.append("company_host_process_windows_test.go")
    run(go, ["test", "-v", *test_sources], native)
    run(go, ["vet", *test_sources], native)
    run(go, ["test", "-v", "./pluginhost"], native)
    run(go, ["vet", "./pluginhost"], native)
    windows = native.copy()
    windows.update(GOOS="windows", GOARCH="386", GO386="sse2")
    run(go, ["vet", "./pluginhost"], windows)
    with tempfile.TemporaryDirectory(prefix="openomsi-bbs-build-") as temporary:
        stage = Path(temporary) / "package"
        host = stage / HOST
        host.parent.mkdir(parents=True, exist_ok=True)
        # Package builds include the native resource object. A fixed module
        # path and trimpath remove checkout paths without stripping inspection
        # metadata or the normal Go build ID. The helper protocol is unchanged.
        host_source = Path(temporary) / "host-source"
        host_source.mkdir()
        for name in HOST_SOURCES:
            shutil.copy2(SOURCE / name, host_source / Path(name).name)
        (host_source / "go.mod").write_text("module openomsi-bbs/pluginhost\n\ngo 1.23.2\n", encoding="utf-8")
        host_env = windows.copy()
        host_env["GO111MODULE"] = "on"
        host_resources = plugin_host_resources.prepare(host_source, SOURCE / "resources", VERSION)
        run(go, ["build", "-trimpath", "-buildvcs=false", "-ldflags=-H=windowsgui", "-o", str(host), "."], host_env, host_source)
        setup_resources.verify_executable(host, host_resources)
        # Rebuild the dev.1 helper from the same protocol sources to prove its
        # runtime code is unchanged and exercise a real old-to-new Setup upgrade.
        previous_host = Path(temporary) / "dev1-host.exe"
        run(go, ["build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -H=windowsgui -buildid=", "-o", str(previous_host), *HOST_SOURCES], windows)
        if hashlib.sha256(previous_host.read_bytes()).hexdigest() != "1c7f1f25bd18edb2b010ec42de2e34db5f3fc8ed2b1d948003635528b8a11a1a":
            raise ValueError("Helper runtime source differs from the published dev.1; review upgrade verification")
        previous_metadata_host = Path(temporary) / "dev2-host.exe"
        plugin_host_resources.prepare(host_source, SOURCE / "resources", "2.0.3-dev.2")
        run(go, ["build", "-trimpath", "-buildvcs=false", "-ldflags=-H=windowsgui", "-o", str(previous_metadata_host), "."], host_env, host_source)
        if hashlib.sha256(previous_metadata_host.read_bytes()).hexdigest() != "2e6dca9ca9f2732d88e5ad3c3787479549d76de3d60c4b90bc874790ecfbf6fd":
            raise ValueError("Rebuilt dev.2 helper differs from the published bytes")
        host_hash = hashlib.sha256(host.read_bytes()).hexdigest()
        print(f"Source-backed host SHA-256: {host_hash}", flush=True)
        digest_flag = "-X=main.bundledPluginHostSHA256=" + host_hash
        for rel, sources in PROGRAMS.items():
            target = stage / rel
            target.parent.mkdir(parents=True, exist_ok=True)
            flags = "-s -w" + (" -H=windowsgui" if rel != "CompanyHost.exe" else "")
            if "plugin_host.go" in sources:
                flags += " " + digest_flag
            if rel == "Setup.exe":
                # A file-list build ignores .syso resources. Use an isolated
                # package with only Setup's explicitly selected source files.
                setup_source = Path(temporary) / "setup-source"
                setup_source.mkdir()
                for name in sources:
                    shutil.copy2(SOURCE / name, setup_source / name)
                # A fixed module path keeps checkout/temp directory names out
                # of package metadata, making resource-bearing builds repeatable.
                (setup_source / "go.mod").write_text("module openomsi-bbs/setup\n\ngo 1.23.2\n", encoding="utf-8")
                setup_env = windows.copy()
                setup_env["GO111MODULE"] = "on"
                resources = setup_resources.prepare(setup_source, SOURCE / "resources")
                run(go, ["build", "-trimpath", "-buildvcs=false", "-ldflags=" + flags,
                         "-o", str(target), "."], setup_env, setup_source)
                setup_resources.verify_executable(target, resources)
            else:
                run(go, ["build", "-trimpath", "-buildvcs=false", "-ldflags=" + flags, "-o", str(target), *sources], windows)
            pin_release_build_id(target, rel)
        symbols = Path(temporary) / "facade-symbols.exe"
        run(go, ["build", "-trimpath", "-buildvcs=false", "-ldflags=-H=windowsgui", "-o", str(symbols),
                 *PROGRAMS["app/compat/Omsi.exe"]], windows)
        run(go, ["run", "tools/release_tools.go", "layout", str(stage / "app/compat/Omsi.exe"), str(symbols)], native)
        if os.name == "nt":
            preview_env = os.environ.copy()
            preview_env["BRIDGE_SETUP_PREVIEWS"] = str(ROOT / "build/setup-previews")
            subprocess.run([str(stage / "Setup.exe"), "--gui-smoke"], check=True, timeout=30, env=preview_env)
            subprocess.run([str(stage / "HostAgent.exe"), "--gui-smoke"], check=True, timeout=30, env=preview_env)
            subprocess.run([sys.executable, str(ROOT / "scripts/test_openomsi_compat.py"), "--report", str(ROOT / "build/openomsi-compatibility.json")], check=True, timeout=600)
            subprocess.run([sys.executable, str(ROOT / "scripts/test_pluginhost_windows.py"), str(host)], check=True, timeout=180)
            subprocess.run([sys.executable, str(ROOT / "scripts/test_facade_windows.py"), str(stage / "app/compat/Omsi.exe")], check=True, timeout=60)
            # Exercise the 32-bit Job Object layout used by CompanyHost.exe,
            # as well as the runner-native layout in the normal test suite.
            run(go, ["test", "-v", "company_host_process_windows.go", "company_host_process_windows_test.go"], windows)
        else:
            print("Native DLL/message-pump checks run on the Windows CI job, not on this host.")
        print("RELEASE_BINARY_HASHES=" + json.dumps({rel: hashlib.sha256((stage / rel).read_bytes()).hexdigest() for rel in [*PROGRAMS, HOST]}, sort_keys=True), flush=True)
        if args.native_server:
            server_dir = stage / "app/server"
            shutil.copytree(args.native_server, server_dir)
            if os.name == "nt":
                result = subprocess.run([str(server_dir / "openomsi.exe"), "--version"], check=True, capture_output=True, text=True, timeout=20)
                if build_native_server.PIN["version"] not in result.stdout + result.stderr:
                    raise ValueError("Packaged dedicated server version mismatch")
        assemble(stage, host_hash)
        package_env = native.copy()
        package_env["BRIDGE_BUILT_PACKAGE"] = str(stage)
        package_env["BRIDGE_PREVIOUS_HOST"] = str(previous_host)
        package_env["BRIDGE_PREVIOUS_METADATA_HOST"] = str(previous_metadata_host)
        # go test compiles the tested package under command-line-arguments and
        # creates a separate generated main; target the tested package's symbol.
        test_digest_flag = "-X=command-line-arguments.bundledPluginHostSHA256=" + host_hash
        run(go, ["test", "-v", "-ldflags=" + test_digest_flag, "-run=^TestBuiltRuntimePackage$", *test_sources], package_env)
        if not args.check_only:
            args.output.parent.mkdir(parents=True, exist_ok=True)
            shutil.copytree(stage, args.output)
            archive_package(args.output)
    print("Bridge/host tests, vet, all seven Windows builds, facade layout and full-package integrity passed.")
    print("Live BCS panel, UAC, PeDePe native trip evaluation and game-trip evaluation require the separately installed products.")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except subprocess.CalledProcessError as error:
        raise SystemExit(error.returncode)
