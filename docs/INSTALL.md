# Installation and use — 2.0.1

Use the complete **OpenOmsi + BBS 2.0.1** Windows ZIP from [Releases](https://github.com/Mayconrib808/OpenOmsi-BBS/releases). It contains Setup, CompanyHost, the bridge, compatibility facade, 32-bit helper and offline tutorial. Go, Python and a separate helper download are not needed to play.

Install OMSI 2, BCS/BBS and OpenOMSI separately, together with the company's map and buses. The live-tested reference is OpenOMSI 0.2.0 Windows x64 and BCS/BBS 5.0.0.1. Other OpenOMSI versions are checked for required capabilities and plugin interface; multiplayer also requires a compatible server protocol. The official 0.2.11 interfaces were reviewed, but a live trip with that version remains pending.

## One-time setup

1. Close BCS/BBS, OMSI and OpenOMSI. If CompanyHost is running on this computer, close it too.
2. Extract the complete ZIP and open its **Setup.exe**.
3. In **Pasta do OMSI 2**, select the original folder containing the game's original `Omsi.exe`. In **Executável do OpenOMSI**, select your separately installed `openomsi.exe`.
4. For company multiplayer, enable **Ativar multiplayer da empresa**, select the administrator's JSON file or paste its HTTPS link in **Perfil da empresa (arquivo JSON ou link HTTPS)**, and enter **Seu nome no multiplayer**.
5. Click **Salvar e ativar** (save and activate) and accept Windows UAC. The included helper is installed automatically.
6. In BCS/BBS **Settings → Advanced settings → OMSI**, leave **Start OMSI faster** unchecked. Portuguese: **Iniciar o OMSI mais depressa**; German: **OMSI schneller starten**.
7. Disable real-time clock synchronisation in OpenOMSI.

The player does not edit JSON, create sessions or host a server. The administrator supplies the profile and starts the session. Settings and an imported profile copy are saved in the user's settings directory and recovered across package updates.

## Playing and finishing

Once the company server is ready, start your trip normally from BCS/BBS. Allow the map to load and check the in-game **Online: in …** state to confirm multiplayer.

At the last stop: **F9 → wait at least two seconds → finish in BCS/BBS while OpenOMSI remains open**.

To test continuity, choose **Next trip** in BCS and wait. Leave OpenOMSI open: the bridge watches for completion of the current shift and a fresh trip with its own shift ID, then closes the previous session before preparing the next one. This button still requires live BCS validation.

## Updating or restoring original OMSI

Close the games, extract the new complete package, run its Setup, review the recovered paths/profile/name and click **Salvar e ativar**. If the administrator sends an updated local profile, select that file. HTTPS profile sources are consulted when preparing the trip and must remain available.

To return to original OMSI, close the games and click **Desativar ponte**. The original game executable was not replaced. This does not reverse a completed BCS trip.

## Setup buttons

| Button | Purpose |
| --- | --- |
| Salvar e ativar | Save paths/name/profile and activate or update the bridge |
| Desativar ponte | Restore original OMSI launches |
| Conferir estado | Inspect activation and configuration |
| Coletar logs | Create a local diagnostic ZIP |
| Tutorial | Open the offline guide |
| Empresa / servidor | Open CompanyHost for the administrator |
| Perfil (administrador) | Open the administrator's profile wizard |

The administrator wizard and the optional legacy `Setup.exe --cli` interface still use a console. Normal player setup uses the graphical window.

See [the simple Portuguese player guide](GUIA_JOGADOR.md), [validation](VALIDATION.md) and [runtime effects](RUNTIME_EFFECTS.md).
