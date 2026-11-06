# Actual 0.3.47 engine integration

Full three-way engine merge of upstream v0.3.47 (`8c4a0d3fd4439e49d18c2812bbd4d91cdd0af7f6`) over the declared 0.3.46 base `17be22987d7c01c813a1473e7696fc71fb3b1d89`, retaining the released Coop changes from `90a93086762258e41bcad77ad3c9e40a028fd2ca`. This supersedes the selected-passenger backport in Bridge 2.2.2.3.

The actual network protocol is 16, including REALM admission. Client, native server and actual launcher bootstrap must identify 0.3.47; a conflicting build version is rejected. The Windows launcher/game/server and P16 Bridge gateway/host must be tested together.

Conflict review preserves driver transfer, camera/destination synchronization, passenger service intent and handback, separate person rate channels, inspectors/emergencies, umbrella rendering, default-enabled upstream passenger behavior, optional traffic and audio. Winter clothing remains removed. Upstream theme palettes are used by custom Coop panels.

Existing P14/P15 servers and complete host binaries remain untouched. A separate source-built P16 host is integrated by the Bridge package, with actual six-control UI/persistence tests before publication.

Gameplay appearance and two-PC driving are not established by compilation or synthetic tests.

Brazilian Portuguese passenger recordings: eight owner-provided clips in `coop-assets/PassengerVoices/pt-BR`, with SHA-256 provenance and loudness normalization metadata. Requests are cabin-audible, wait two seconds for a closed door, require capacity to board, and a missed alighting emits once even without a previous stop. PTB migration enables the two existing reaction switches once and then honors opt-outs. Other locales retain installed voices.

Local validation before Windows candidate: `cargo check --locked -p omsi-app --tests` passed, including all app test compilation. Library suites: omsi-sim 963 passed / 31 ignored, omsi-net 207 passed, launcher-core 133 passed / 2 ignored / 2 filtered, launcher-api 31 passed, frontend 3 passed. The two process-identity tests were filtered only locally because this executor's virtual process filesystem cannot resolve native process IDs (`ps` reports a self-lookup failure); they are not filtered in Windows CI. Windows build, actual host UI, native P16 gateway and first full-runtime update are required by the candidate workflow.
