# Bridge update distribution

Production: https://openomsi-bbs-updates.maycongamesmaximo.workers.dev

Player downloads use Cloudflare Workers Static Assets. Only the seven Bridge
executables are updated; the native game, server and player configurations are
preserved. No Cloudflare credential or private signing key is shipped to players.

The repository already provides CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID
as Actions secrets. The isolated `Deploy update distribution` workflow uses them
to deploy this Worker, without recompiling Rust or modifying the multiplayer
Worker. Browser login is not required.

Publication order:
1. Build the tested update components with the production origin and a new
   sequence using `Build update candidate` (Windows GUI smoke test included).
2. Download the candidate EXE and sign its manifest offline with the owner's
   private publisher. The sequence must equal the Setup build sequence.
3. Commit only the signed public `assets/stable.json` and `publication.json`
   containing the approved `candidate_run_id`. Never commit publisher-key.json.
4. The deploy workflow downloads that artifact, validates the manifest signature,
   installer SHA256 and size, then deploys manifest and installer together.
5. The workflow downloads the live manifest and installer to verify publication.

The current asset limit is 25 MiB per file. A larger installer requires switching
storage before publishing. `publication.mjs` enforces this limit.

The Setup checkbox controls checking on startup. Manual checking remains
available. Installation requires the user to choose the update and close the
Bridge/game/BCS/HostAgent processes.
