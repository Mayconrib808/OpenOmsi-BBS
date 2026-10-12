Development preview for testing BBS + openOMSI multiplayer integration. This is an independent project, not an official PeDePe product.

- Complete standalone package: no Bridge 2.x installation required. Original OMSI 2 and BCS installations are required.
- English Setup.exe name and project artwork; PT-BR, English and German interface.
- Forward real openOMSI output to the BCS-owned game.log to unblock its loading detection.
- Disable multiplayer for solo openOMSI; disable/reactivate the integration with BCS closed, preserving settings and backing up BCS configuration.
- Manual and automatic update checks filter only 3.x releases. The 2.x channel and stable/latest release remain unchanged.

Assets: use the full `-teste.zip` for a first install. `Atualizar-Multiplayer-3.0.2-dev.4.exe` updates an existing 3.x installation. Close BCS and the game first. An updater executable with a GitHub-provided SHA-256 digest is needed for installation from the new update button.

The GitHub update query first exists in dev.4. Older dev.1–dev.3 packages need one manual update; publishing a release does not retrofit their disabled update feed.

Still pending: real BCS host/guest and trip-finalization validation on two PCs, next-trip integration through a permitted interface, dedicated signed update feed publication, and company weather/passenger settings. The bundled server is the verified 0.2.20-bbs-free2 runtime, not an official 0.2.29 server. Existing company clock settings are preserved; Transfort's configuration used Europe/Berlin minus 8 hours.
