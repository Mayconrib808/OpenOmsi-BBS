# Winter clothes: stock-human QA before release

**Status: NOT VISUALLY APPROVED.** The seven authentic artist garments are test-only. The existing `skin_binding.json` flags `fitted_to_stock_omsi_human=false` and `released=false` must stay false until actual in-game QA evidence exists. A successful build, OBJ parser, 21 synthetic rig fits, simulated arm movement, or offline rendered silhouette does **not** establish that clothing looks correct in OpenOMSI.

## Additional numeric QA using licensed OMSI humans

The `actual_installed_stock_humans_fit_without_triangle_explosions` Rust test reads installed .hum files and their referenced .o3d meshes, measures each real passenger rig with the game's `HumanType::load`, fits all seven garments and checks both rest-pose triangles and arm/head joint transforms. It never edits or uploads the licensed OMSI assets. Supply at least one male and one female stock human, plus additional add-on models if possible.

In PowerShell from the repository root (adjust paths to your installation and the extracted, test-only artist bundle):

```powershell
$env:OPENOMSI_WINTER_TEST_ASSETS = 'C:\qa\winter-test-assets'
$env:OPENOMSI_WINTER_STOCK_HUMANS = 'C:\Program Files (x86)\Steam\steamapps\common\OMSI 2\Humans\Other\man02.hum;C:\Program Files (x86)\Steam\steamapps\common\OMSI 2\Humans\Other\woman01.hum'
cargo test --locked -p omsi-app actual_installed_stock_humans_fit_without_triangle_explosions -- --nocapture
```

The example human filenames are placeholders: choose .hum paths that **actually exist** in the installed game. A missing `OPENOMSI_WINTER_STOCK_HUMANS` explicitly **skips** this test and does not count as QA. Record the list of real .hum paths and the `STOCK_HUMAN_NUMERIC_FIT_OK` results. Even passing this test does not authorize release.

## Required visual QA in the actual game

Run a **non-public QA build** with the real game installation and authentic clothing assets, without changing production approval flags or bypassing the live release guard. Capture dated evidence for each of the seven garments on representative stock passengers. Show front, back and side while standing, walking, sitting, looking sideways, reaching with arms, and moving hands. Repeat with short, average and tall passengers; include female and male rigs and a dark/rainy scene. Inspect sleeves, shoulders, hem at waist, neck, hats, gloves, fabric texture, clipping, z-fighting, black shading, and oversized triangles. Record FPS, VRAM, the winter feature audit log, temperature transitions at 15 C and 5 C, and whether existing CTI winter skins are left alone.

Only an explicitly reviewed evidence record linked to the tested executable, garment bundle hashes, actual .hum models and screenshots/videos can support changing the per-garment release flags. Never use a runtime environment variable, a guessed approval bit or a passing geometry test to unlock the shipped renderer.

## Publication conditions

Keep the published `2.2.2.2-hotfix.1` safety version untouched. After visual sign-off, test P15, BCS, Bridge, installer, updater, and the cumulative per-version/hotfix changelog. Publish a new version only through the official Ed25519-v2-signed workflow, verify GitHub success, live Cloudflare `stable.json`, version and SHA-256 checksums, and only then announce availability in **Buscar atualizações**.
