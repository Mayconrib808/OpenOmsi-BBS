# 2.2.2.2 WinterClothes: visual deformation failure (unreleased safety patch)

## Evidence from player screenshots on 2026-11-01

- At 12°C, an additional dark cloth mesh protrudes far outside a passenger's body and intersects the bus.
- At -20°C with snow, jackets, sleeves and hats do not conform to torsos, heads or moving arms; large detached triangles float beside passengers.
- **The game does not contain a correctly fitted winter coat in these screenshots.** A parsed OBJ, valid material and uploaded vertex buffer cannot certify correct appearance.

## Root cause

`scripts/retarget_winter_models.py` only performs axis/scale conversion and heuristic x-position-only bone weighting. Its JSON explicitly records `fitted_to_stock_omsi_human: false` and `released: false`. `winter.rs` mistakenly loaded those meshes into the live scene without respecting either flag. Original stock OMSI skeleton joints and MakeHuman rest joints do not match. The generated weights and offsets can therefore distort geometry severely. The asset loader tests passed UV, OBJ and texture parsing, **not** visual rigging.

## Immediate safe behavior

`winter.rs` rejects all artist garment meshes lacking **both** visual fit approval and explicit release approval, and logs why. The stock human mesh remains visible and other weather systems remain intact. The ordinary installed CTI clothing selection still works when seasonal CTI variants exist. This is **not** the completed winter garment feature and must not be advertised as such.

For players on the existing 2.2.2.2 distribution, disabling the `weather_clothing` optional feature in the launcher immediately avoids rendering the broken overlay; restart the game.

## Release blockers — do not sign the clothing as fixed yet

1. Fit authored garments against the actual stock OMSI human meshes, measured shoulder, torso, head, wrists and walk/sit bone transforms; rebuild semantic weights from the target rig, rather than horizontal x thresholds.
2. Validate in-game screenshots and videos at 12°C and -20°C with snow, standing, walking, seated, boarding, waving and paying; check front/back/side views and different male/female stock rigs. Require no floating caps, detached gloves, giant triangles, face occlusion or clipping.
3. Validate mesh transformations and instance reuse for multiple NPCs; reject geometry with out-of-range posed bounds or stretched triangles, and verify no GPU mesh data is shared unsafely between NPCs.
4. Repeat Windows gameplay, LAN pair, performance and memory tests. Only then mark specific tested model/rig combinations fitted and released.
5. Publish the result with a **new hotfix changelog** and signed update, preserving the cumulative release notes and rollback. Passing parser/unit tests alone does not permit publication.

## Verification

The branch `fix/winter-visual-mesh-qa-gate` runs the Windows app check and the real artist-bundle parser with a regression asserting unapproved bindings cannot be loaded as visible garments. This safety patch has **not** been promoted to the live update channel.
