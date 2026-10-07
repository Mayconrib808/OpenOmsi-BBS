# Setup artwork

The header is the same project image as `docs/assets/readme-banner.png`, converted to a 320 × 90, 24-bit BMP for the native Windows STATIC control.

`setup-icon.svg` reuses the bus symbol and colors from `docs/assets/readme-banner.svg`. `setup.ico` contains 16, 24, 32, 48, 64, 128 and 256 pixel PNG images for Explorer and the window/taskbar. Original project artwork remains under the repository MIT license.

`scripts/setup_resources.py` embeds these assets as RT_BITMAP, RT_ICON and RT_GROUP_ICON in Setup.exe using a Windows x86 COFF object generated during the build. The build verifies every linked resource against these source bytes. The native Windows smoke check also loads the actual bitmap and both window icon sizes.

To regenerate the icon, render the SVG with CairoSVG and save an ICO with Pillow at the listed sizes. Header conversion uses Pillow with RGB/LANCZOS. These libraries are only needed when changing artwork, not for normal package builds.
