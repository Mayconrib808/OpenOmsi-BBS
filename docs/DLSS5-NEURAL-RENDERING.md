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

The desktop launcher's graphics settings can now store a locally selected
`UniversalDLSS5.exe` and open its controller window. This is an **explicit manual**
action by the player. The Bridge does not distribute third-party executables,
download them, inject any graphics hook, or automatically launch a second game.
The user must install the program from its project, independently obtain the
legitimate vendor-signed `nvngx_dlssnr.dll` and any required Streamline NR
plugin, start the game normally, and choose **Attach** inside the controller.
The controller itself must report **Neural Active** before anyone claims the
feature is working; a successful controller launch is not sufficient.

Avoid combining this experimental injection with the ReShade Vulkan layer until
compatibility is validated. Do not enable it automatically for public online
sessions because manual third-party injection can interfere with game hooks.

The native DLSS 5 implementation remains gated on NVIDIA's official NR SDK.
