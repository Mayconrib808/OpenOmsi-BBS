"""Check the actual shipped Setup's automatic and manual signed update checks."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("native_ui", ROOT / "build-support/bridge2203/test_p15_host_controls.py")
ui_module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ui_module)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("package", type=Path)
    args = parser.parse_args()
    package = args.package.resolve()
    with tempfile.TemporaryDirectory() as temp:
        env = dict(os.environ)
        for name in ("LOCALAPPDATA", "APPDATA"):
            env[name] = str(Path(temp) / name)
            Path(env[name]).mkdir()
        proc = subprocess.Popen([str(package / "Setup.exe")], cwd=package, env=env)
        ui = ui_module.NativeUI(proc.pid)
        window = None
        try:
            deadline = time.monotonic() + 40
            while time.monotonic() < deadline:
                assert proc.poll() is None, "Setup exited during its update check"
                window = next((h for h in ui.windows() if "BBS" in ui.text(h)), None)
                if window:
                    controls = ui.children(window)
                    texts = [c["text"] for c in controls]
                    assert "Distribuição aguardando configuração." not in texts, "Shipped Setup has no update origin"
                    assert "Não foi possível buscar agora." not in texts, "Shipped Setup could not verify the live signed feed"
                    if "Você está atualizado." in texts:
                        break
                time.sleep(0.2)
            else:
                raise AssertionError("Setup did not complete its live signed update check")
            button = next(c for c in controls if c["cls"].lower() == "button" and c["text"] == "Buscar atualizações")
            ui.send(button["hwnd"], 0xF5)
            # The manual check must return to Current and must never offer the
            # older Hotfix 2 feed as an update to this newly packaged release.
            time.sleep(0.5)
            deadline = time.monotonic() + 30
            while time.monotonic() < deadline:
                texts = [c["text"] for c in ui.children(window)]
                assert not any("Atualizar agora" == text for text in texts), "Older feed offered as an update"
                assert "Distribuição aguardando configuração." not in texts
                assert "Não foi possível buscar agora." not in texts
                if "Você está atualizado." in texts:
                    break
                time.sleep(0.2)
            else:
                raise AssertionError("Manual update check did not complete")
            (package / "setup-update-service-test.json").write_text(json.dumps({
                "status": "passed", "automatic_live_signed_check": True,
                "manual_live_signed_check": True, "old_feed_not_offered": True,
                "tested_executable": "Setup.exe",
            }, indent=2) + "\n")
        finally:
            if window:
                ui.u.PostMessageW(window, 0x10, 0, 0)
            try:
                proc.wait(timeout=8)
            except subprocess.TimeoutExpired:
                proc.terminate()
                proc.wait(timeout=8)


if __name__ == "__main__":
    main()
