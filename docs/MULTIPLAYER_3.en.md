# BBS+OpenOmsi Multiplayer 3.0

Drive your **Bus Company Simulator / Busbetrieb-Simulator (BCS/BBS)** trips in **openOMSI multiplayer**. BCS selects and evaluates the trip; openOMSI simulates the shared world. Drivers on the same map can encounter each other's buses.

**3.0.3-rc.2 is a release candidate.** Complete a real two-PC trip before treating it as the final WebDisk release.

## Requirements and setup

Windows 10/11 x64, your own installed OMSI 2 and **BCS 5.0.2.0**, and the map, dependencies and bus you drive. Client and server need **protocol 7**, introduced in **openOMSI 0.2.48**. 0.2.47 was not officially released. The package includes the unmodified official **0.2.50 server**.

1. Extract the complete ZIP to a permanent folder. Close BCS and open **Setup.exe**.
2. Choose a language and confirm your original OMSI 2 folder.
3. Create a company room or paste the company's invitation. Enter your player name.
4. When creating a room, copy the reference timezone and **Time shift** from the company's BCS settings. Enter zero explicitly when appropriate.
5. Save and click **Configure BCS**, with BCS closed. Settings are backed up first.
6. In BCS verify **OpenOMSI (Beta)** and **OpenOMSI-BBS/PeDePeAdapter**. If automatic configuration is unavailable, copy the adapter folder from Setup and select it manually in BCS.
7. Start your trip from BCS. Allow time for the first client/tunnel download and map loading.

Select the real client in Setup, not in BCS. Automatic client downloads verify official SHA-256 hashes. Advanced options → Client → Manual lets you keep your own compatible executable, including an existing 0.2.50 DLSS/ReShade installation. Graphics effects are configured separately.

## Hosts and maps

The **company invitation** routes each trip to its own map session. The first driver on a map hosts an independent local server; later drivers on that map join it. Berlin and Carrão City use separate servers and tunnels. Drivers on different maps do not see each other. A raw `trycloudflare.com` link to Berlin points to Berlin only; use the company invitation in Setup to route different maps correctly.

Closing the host's game leaves the dedicated process running while peers remain. The host PC must stay on and connected. Turning it off disconnects the session. A later join can elect another host; an active world's state is not transferred seamlessly. Empty sessions stop after a grace period without changing the invitation.

You drive your own bus. Missing models may be represented by stand-ins on other computers. Install the same bus and repaint to see the exact model. No maps, private buses or paid DLC assets are redistributed.

## Finishing and the next trip

BCS supplies the bus, situation, duty, repaint and weather. The integration prepares the service and restores the selected player's saved fuel. Valid map/HOF/bus scripts are still needed for the destination display.

BCS 5.0.2.0 has a native command/acknowledgment interface for **Next trip**. The original PeDePe plugin is forwarded byte for byte and selects the next duty in the running game. No BCS JAR patch is applied.

Finish using the **native BCS flow**, leaving openOMSI open until BCS confirms the result. The old F9/two-second instructions belong to the legacy 2.x integration. Then select **Next trip** in BCS and check the new duty and display before departing.

## Updates and support

Integration updates use the independent verified **Cloudflare 3.x** feed. Automatic checks, manual checks and their preference are available. Close BCS before installing; company/player preferences are preserved. Client/server updates have separate choices. Recommended uses the included 0.2.50 server; Manual preserves your selection. Protocol 6 is not a silent fallback.

Disable multiplayer to keep solo openOMSI trips. Disable the program with BCS closed to restore original OMSI launching. Existing sessions are not interrupted.

Use **Collect logs** in Setup, review the ZIP and report the map, bus/HOF and trip through [Issues](https://github.com/Mayconrib808/OpenOmsi-BBS/issues/new/choose) or Discord **`.zmaycon.`** (include both dots). No report is uploaded automatically.

The [validation record](VALIDATION_3.md) distinguishes automated checks from pending live gameplay and antivirus reports. Independent community project by Mayconrib808; not an official PeDePe or openOMSI product or endorsement.
