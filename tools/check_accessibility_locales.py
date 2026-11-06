"""Check that the accessibility controls and messages cover every game language."""
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
LOCALES = ROOT / "crates/omsi-app/locales"
expected = set(re.findall(r"^  ([a-z-]+):", (LOCALES / "passengers-0347.yml").read_text(encoding="utf-8"), re.M))
table = {}
key = None
for line in (LOCALES / "accessibility-223.yml").read_text(encoding="utf-8").splitlines():
    if line.startswith('"'):
        key = json.loads(line[:-1])
        assert key not in table, f"Duplicate text: {key}"
        table[key] = {}
    elif line.startswith("  "):
        lang, text = line.strip().split(":", 1)
        value = json.loads(text.strip())
        assert value.strip() and lang not in table[key], (key, lang)
        table[key][lang] = value
for key, languages in table.items():
    assert set(languages) == expected, (key, expected - set(languages), set(languages) - expected)
    for lang, text in languages.items():
        assert set(re.findall(r"\{[^}]+\}", text)) == set(re.findall(r"\{[^}]+\}", key)), (key, lang)
assert len(table) == 31
schema = (ROOT / "launcher/ui/js/pages/settings-schema.js").read_text(encoding="utf-8")
assert schema.count("key: 'wheelchair_passengers'") == 1
assert schema.count("key: 'wheelchair_spawn_percent'") == 1
print(f"Accessibility: {len(table)} messages in {len(expected)} languages plus English; both launcher controls present")
