# Review of the corrected 2.2.2 Hotfix 2 native source

This developer-source revision fixes the language and optional-setting defects in the original Hotfix 2 export. It is **later than the automatic-update release**, which is retained as an immutable original-source record. The fixes below are in production source, with executable regression tests, rather than proposed changes alone.

## Review history consulted

| Upstream PR | Initial concerns relevant to this delivery | Follow-up actually observed |
| --- | --- | --- |
| [#3427](https://github.com/openOMSI-org/openOMSI/pull/3427) | Complete all 29 languages; default off in both settings writers; no work when disabled; avoid per-frame buffers and repeated bus matching; only stationary waiting passengers; separate tests and preserve attribution | Initial [review](https://github.com/openOMSI-org/openOMSI/pull/3427#pullrequestreview-5711877331), then [approval](https://github.com/openOMSI-org/openOMSI/pull/3427#pullrequestreview-5716284744). PR is merged at this audit. Do not reintroduce its hailing helpers on current main. |
| [#3428](https://github.com/openOMSI-org/openOMSI/pull/3428) | Full-bus reaction must differ from hailing; stack on #3427; cache capacity only for targeted buses; prevent repeated scans and wrist snapping | Initial [review](https://github.com/openOMSI-org/openOMSI/pull/3428#pullrequestreview-5711877600) and [follow-up](https://github.com/openOMSI-org/openOMSI/pull/3428#pullrequestreview-5716293888). The API currently marks that follow-up DISMISSED, and the PR remains open; its positive text is not reported as current approval. |
| [#3429](https://github.com/openOMSI-org/openOMSI/pull/3429) | Complete translations; avoid routine info logging; check usable services before terrain/collision; consume no RNG while disabled; explicit multiplayer authority; keep feature tests independent | Initial [review](https://github.com/openOMSI-org/openOMSI/pull/3429#pullrequestreview-5711877849), then [approval](https://github.com/openOMSI-org/openOMSI/pull/3429#pullrequestreview-5716294498). PR remains open. Approval of that single-player adaptation does not approve this BCS/cooperative implementation. |

The three PRs are adaptations of earlier passenger features, not a review or approval of the new BCS incidents, inspector or traffic-density code. Their advice is applied here as review criteria. This audit consulted the current descriptions and initial/follow-up review bodies directly, rather than relying on an old screenshot.

## Corrections implemented in this revision

| Finding | Correction in the current source | Evidence |
| --- | --- | --- |
| I18N-01 — resolved | Four distinct passenger reaction captions use the native compiled catalog. All new setting labels/help and nearby previously untranslated controls have all 29 non-English translations. Native and launcher tables agree. | Upstream locale validator: 19 new keys, 573 added translations, no existing translations changed, zero errors/warnings. Compiled-catalog tests cover every supported language and alias and reject English placeholders. |
| OPTIONS-01 — resolved for the added features | Seven independent controls in the game and launcher; default off, explicit false preserved, restart required. Existing umbrellas/hailing default off; legacy migration no longer forces effects on. Original animation is the fresh default in both interfaces. | Every one of the 128 setting combinations is saved/reopened through both writers; legacy/partial configurations and real launcher schema are tested. See [OPTIONS.md](OPTIONS.md). |
| Cosmetic RNG — resolved | Disabled door/thanks paths return before work. Enabled exit thanks use deterministic cosmetic selection, separate from task/destination/ticket RNG. Connections are effectively 0% while disabled. | Behavioral simulation tests check wait state, caption identity and unchanged RNG; configuration tests enforce disabled connections even with a saved 100% chance. |
| TRAFFIC-01 — calculation unified | Host and LAN centers call the same `street_target_near` implementation. Extra density is host-authoritative and optional. | One shared source path; bounded-scale unit tests. A real separated two-PC journey is still pending. |
| TRAFFIC-02 — resolved calculation defects | One nearby-lane query. No extra length summation while disabled. Off preserves the original road scale. Enabled extra scale has a zero lower bound, so closed roads are not forced to density 1. | Tests cover disabled baseline, zero roads, sparse cases and bounded long-avenue increase. No FPS claim follows from these unit tests. |
| PERF-01 — duplicate synchronization removed | Inspector changes use the existing people synchronization on the following frame; the extra whole-population `apply_people` call was removed. | Windows build and passenger/inspector regressions. No measured FPS improvement is asserted. |
| Missed-stop caption — resolved | Missed stop, closed-on-rider request, polite door request and exit thanks have distinct caption identities and translations. | Reaction priority and every-language caption distinction tests. |
| Stale AI-stop test — corrected contract | The older unknown-route test expected the last stop ID although the Hotfix 2 code correctly selects the nearest eligible stop. The test now checks geometry in both directions by moving the previously nearest stop farther away. | Full passenger simulation test selection, not just the former narrow filters. |

## Passenger speech scope

[reactions.rs](../crates/omsi-sim/src/people/reactions.rs) and [pax_tick.rs](../crates/omsi-sim/src/people/pax_tick.rs) contain the optional door/missed-stop behavior. [pax_task.rs](../crates/omsi-sim/src/people/pax_task.rs) invokes optional exit thanks only when the rider leaves the bus. [dialog.rs](../crates/omsi-app/src/app_events/frame/people/dialog.rs) selects translated catalog text in the current UI language.

- Door request: “Could you open the door, please?”
- Missed stop: “Driver, you missed my stop!”
- Closed-on-rider request: “Please let me get off!”
- Exit thanks: “Thank you, driver!”

The installed voice pack supplies audio where available. There are no newly recorded or synthesized 29-language voice packs. Captions remain available without a dedicated voice file. Ordinary ticket/payment thanks and bad-driving complaints remain upstream features with their existing settings.

## Remaining findings and limits

| ID | Current evidence | Remaining work |
| --- | --- | --- |
| PERF-02 — partially mitigated | An already-walking inspector now returns before cloning the bus/Pax or collecting cabin points. Patrol-leg point collection/sorting still exists. | Cache topology with correct invalidation and profile live addon cabins. |
| PERF-03 — still open while enabled | Active BCS scenes can rebuild context/rider buffers; remote poses are cloned. Eligible active door reactions still clone a Pax. Disabled features skip these optional paths. | Event-change detection/reusable buffers and allocation profiling. |
| PERF-04 — still open while enabled | `incident_pavement` still scans/sorts full-network candidates; failed dispatch retries every 0.5 seconds. | Spatial index, bounded work and blocked-curb/large-map profiling. |
| SPEECH-01 — still open while enabled | Missed-stop reactions can bypass the bus-wide caption decision; audio has its normal shared cooldown. | Full-bus priority/caption-throttling behavioral coverage. Do not claim full-bus spam fixed by caption translation. |
| SPEECH-02 — still open | Open-exit inference is not proven for every articulated/addon cabin's selected reachable door. | Multi-section/addon fixtures and selected-exit mapping; the one-entry/one-exit regression remains covered. |
| TEST-01 — expanded, not exhaustive | Windows tests now exercise the full people selection, both settings writers, compiled translations and traffic scaling. | Full-bus timers, separated LAN scenes, visual/audio behavior and real-map performance measurements remain pending. |
| DOC-01 — historical document | `docs/beta/bcs-incidents.md` retains an older medical-only beta description. Current dispatch supports police/fire and uses a 0.5-second retry, without that document's five-second dwell parameter. | Use this review and current source as the revision reference; do not copy the old beta description as current implementation. |

The existing capture reader limits each read to 64 KiB, runs on a 250 ms worker, and validates schema/process/freshness. Its worker starts only if an enabled feature needs capture. Scene models start only for enabled scenes with capture available. Inspector-only operation skips scene rider collection and effects. Host response processing and AI greeting candidate collection have their own disabled fast paths.

## Integration into current upstream

Start from current upstream main and inventory already merged helpers before preparing independently buildable PRs. These five file-disjoint patches reconstruct an entire historical-base native workspace; they are review partitions, not five separately compilable contributions. Keep shared settings/catalog changes, attribution, feature test files and host authority explicit. Approval of the single-player port in #3429 does not establish correctness of this cooperative implementation.

No external Bridge, Setup, HostAgent, CompanyHost, P14 service, update feed or proprietary asset was modified for this revision. Existing AGPL and data-file license notices remain in the source tree.
