# Bridge update distribution

Production: https://openomsi-bbs-updates.maycongamesmaximo.workers.dev

Player downloads use Cloudflare Workers Static Assets. Only the seven Bridge
executables are updated; the native game, server and player configurations are
preserved. No Cloudflare credential or private signing key is shipped to players.

The repository already provides CLOUDFLARE_API_TOKEN and CLOUDFLARE_ACCOUNT_ID
as Actions secrets. The isolated `Deploy update distribution` workflow uses them
to deploy this Worker, without recompiling Rust or modifying the multiplayer
Worker. Browser login is not required.

## Signing-key continuity

The original 2.1 signing key is preserved in the owner's private
`OpenOmsi_2.1.2_Publicador_PRIVADO.zip`, in `publisher-key.json`. The archive
is a private owner backup, never a player download or repository asset.
If a workspace copy is missing, recover this backup before generating any key.
The separate `BBS-OpenOmsi-3x-update-signing-key.json` belongs to the 3.x channel.
Validate the recovered key against `bridgeUpdatePublicKey` before signing.
Cloudflare API credentials grant publication access; they do not replace this
signing key. Read the latest signed manifest and the built Setup sequence before
publishing, because an older backup's `last_sequence` can be stale.

The 2.1.8 automatic update was signed with this original key and uses the same
installer bytes as the validated 2.1.8 manual package. Existing installations
can receive it through Setup without replacing their trust key or reinstalling.

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
