# DLSS 5 · 3D-Guided Neural Rendering — integration gate

**State: capability detection only. Neural Rendering is NOT running in openOMSI.**

DLSS 5 is a separate neural rendering stage and must not be represented as a
DLSS Super Resolution quality mode. The NVIDIA Streamline 2.14.16 public SDK
announces a `sl.dlss_nr` plugin in its changelog but does not publish the
DLSS NR integration header, API contract, signed model/runtime distribution
package or a public programming guide for this feature.

The launcher now identifies NVIDIA's currently supported **GeForce RTX 50**
adapters through DXGI vendor ID and the adapter model. These checks are only
preconditions and never claim the feature is running. The existing opt-in
installer installs **DLSS Super Resolution**, not Neural Rendering. ReShade is
unrelated, and neither system is secretly used as a DLSS 5 substitute.

## Work necessary to actually activate DLSS 5

1. Obtain official NVIDIA DLSS Neural Rendering SDK, license, integration
   documentation, supported models and redistribution authorization.
2. Integrate the supported NVIDIA Streamline backend with openOMSI's wgpu
   DirectX 12 or Vulkan command submission *before* presenting any enable
   switch. Do not inject unknown community-modified runtime binaries.
3. Supply correctly synchronized scene color, motion vectors, reset/temporal
   state, and all masks/metadata specified by the official SDK. These must be
   extracted from the real renderer rather than approximated by ReShade.
4. Detect NVIDIA feature support and required driver/runtime with the official
   SDK on the **selected rendering device**, not merely the first DXGI adapter.
5. Evaluate the Neural Rendering feature at the actual frame output stage and
   compare output/performance on a real GeForce RTX 50 system. Restore GPU
   state correctly and provide an automatic fall-back on failures.
6. Only after these steps, expose an independent toggle alongside the
   graphics/upscaling controls, persist the choice and test on the release
   candidate without shipping unlicensed NVIDIA components.

Official information:
- https://www.nvidia.com/en-us/geforce/news/dlss-5-3d-guided-neural-rendering/
- https://github.com/NVIDIA-RTX/Streamline

## Optional experimental route with an external controller

An independent, Apache-2.0 project, [UniversalDLSS5](https://github.com/MotionflowOffical/UniversalDLSS5),
tries to execute NVIDIA Neural Rendering on games without native support. It is not
an NVIDIA product and its injector compatibility is **not verified with openOMSI**.

The desktop launcher's graphics settings can now select a local
`UniversalDLSS5.exe`, or download a fixed SHA-256-verified third-party
controller installer after explicit opt-in. It can then launch the controller
window; the Bridge does not redistribute NVIDIA files, inject graphics hooks,
automatically attach to games, or launch another game.
The player must still obtain the legitimate vendor-signed `nvngx_dlssnr.dll`
and any required Streamline NR plugin from an authorized source, import them
inside UniversalDLSS5, start the game normally, and choose **Attach** inside
the controller.
The controller itself must report **Neural Active** before anyone claims the
feature is working; a successful controller launch is not sufficient.

Avoid combining this experimental injection with the ReShade Vulkan layer until
compatibility is validated. Do not enable it automatically for public online
sessions because manual third-party injection can interfere with game hooks.

The native DLSS 5 implementation remains gated on NVIDIA's official NR SDK.

## Verified controller installer from the launcher (2026-11)

To reduce manual steps, the Windows x64 launcher now exposes a **user-initiated**
installer for independent UniversalDLSS5 **v0.5.1**. After explicit opt-in,
it downloads only the official GitHub release's `UniversalDLSS5-0.5.1-Setup-x64.exe`,
verifies the complete archive with a pinned SHA-256 digest from the release,
then opens the original NSIS installer. The launcher can automatically recognize
common Program Files and LocalAppData installation paths; other paths may be
selected manually. It never silently injects the game or runs this tool at
startup. It never downloads NVIDIA proprietary neural DLLs.

A correctly installed controller is **not** a completed DLSS 5 installation.
The NVIDIA-signed `nvngx_dlssnr.dll` must be obtained from an authorized vendor
source and imported through the independent controller, as described in that
project's instructions. The public Streamline SDK includes Super Resolution,
not a redistributable DLSS 5 model. Windows antivirus may warn about third-party
injection. Do not disable endpoint security, and do not install or attach it
without the player's express action.
