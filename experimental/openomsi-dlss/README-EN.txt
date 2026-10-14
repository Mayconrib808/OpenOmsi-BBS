openOMSI 0.2.50-dlss-test.1 — test client, protocol 7

Extract the complete package into a separate folder. Close the game and launcher.
Run Preparar-DLSS.cmd to prepare the four native DLSS DLLs (copied from an existing
0.2.314 installation, or downloaded from NVIDIA Streamline SDK 2.14.1).
In DLSS 5 Swapper, Add Game selects this PACKAGE FOLDER. Use Native DLSS (RenoDX), DX12.
Exit Swapper from its tray icon. Run Preparar-DLSS.cmd again to prepare ReShade.
Open openomsi.exe and choose DirectX 12 and NVIDIA DLSS / DLAA in Graphics.
First try your own map and bus alone; then select this openomsi.exe in Bridge Setup.
The BBS trip continues to supply the map, bus, timetable and server connection.

This port supplies native DLSS/DLAA. Neural Rendering still requires RenoDX/Swapper.
Home -> DLSS 5 Neural Rendering must report active processing and an increasing
Successful NR frames count; an F8 Connected/On label alone is insufficient.

RTX hardware, image/motion, configuration persistence, BBS integration and real
multiplayer still require an in-game Windows test. Build checks do not prove them.
Other players may use an official protocol-7 client; the server needs no DLSS tools.
This test edition does not install official automatic updates over the DLSS build.
See dlss-build.json for build provenance and checksums.
