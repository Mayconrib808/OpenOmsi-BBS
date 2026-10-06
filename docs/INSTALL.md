# Installation and use

**Use the complete v1.1.3 runtime ZIP.** All four bridge components, including the new source-backed 32-bit host, are bundled. Setup installs the helper automatically. No separate helper download, Go or Python installation is needed to use the bridge.

Install legitimate OMSI 2 and BCS/BBS separately, along with the required openOMSI version. The imported implementation targets openOMSI 0.2.0 Windows x64 and the BCS/BBS 5.0.0.1 local layout. Updates can affect compatibility.

## Complete package flow

1. Close BCS/BBS, OMSI 2 and openOMSI. When upgrading, deactivate the previous bridge with its own Setup first.
2. Keep the complete bridge folder in a fixed location. Run its `Setup.exe` and choose Portuguese, English or German.
3. Configure the original OMSI 2 folder containing its original `Omsi.exe`, and the separately installed `openomsi.exe` path.
4. Choose Setup option **2** to activate the scoped launch redirect and accept Windows UAC. Activation stages the included helper automatically under a bridge-specific name and updates a recognised historical copy.
5. In BCS/BBS **Settings → Advanced settings → OMSI**, leave **"Start OMSI faster" unchecked**. Portuguese: **"Iniciar o OMSI mais depressa"**; German: **"OMSI schneller starten"**.
6. Disable real-time clock synchronisation in openOMSI. Start the trip normally through BCS/BBS and allow map loading to finish.
7. At the last stop, **press F9**, wait at least **two seconds** for saved data to settle, then finish in BCS/BBS **with openOMSI still open**.

Read [runtime effects](RUNTIME_EFFECTS.md), particularly the write to `Drivers/bbs.odr`. The bridge tries to preserve saved counters; the service's evaluation and full telemetry coverage are not guaranteed.

## Returning to original OMSI

Close the games and use Setup option **3** to deactivate the bridge. The scoped launch redirect is removed/restored by the code; the original `Omsi.exe` was not replaced. This does not reverse an already completed BCS trip.

## Setup options

| Option | Purpose |
| --- | --- |
| 1 | Configure paths and language |
| 2 | Activate or update the bridge |
| 3 | Deactivate and return to original OMSI launches |
| 4 | Inspect activation status |
| 5 | Create a local diagnostic ZIP |
| 6 | Verify a complete package's integrity manifest |
| 7 | Open the offline tutorial included by that full package |
| 0 | Exit Setup |

The source build now supplies the helper, tutorial and full manifest. See [build instructions](BUILD.md); no proprietary dependencies are included.
