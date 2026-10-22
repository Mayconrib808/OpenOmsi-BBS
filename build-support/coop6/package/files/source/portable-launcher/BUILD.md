# Portable package launcher

`ABRIR-COOPERATIVO.exe` starts `programa/openomsi.exe` from this package.
It uses Unicode Windows APIs, accepts and forwards the ordinary game arguments,
and requests no administrator privileges. It does not copy, patch or overwrite
an existing openOMSI installation. The game keeps its usual per-user settings.

The launcher sets `OMSI_NO_UPDATE=1`, `OMSI_NO_GUARD=1` and `OPENOMSI_BIN` for its
own process and descendants, and clears `OMSI_LAUNCHER` there. This prevents
official automatic replacement of the experimental fork during this launch and
selects this package's launcher/game. No persistent environment or settings
changes are made by this small program. The game and official launcher still
create their usual settings, logs and caches.

Build on Windows with a MinGW-w64 x86-64 toolchain, from this source directory:

```bat
x86_64-w64-mingw32-windres -I . start-coop.rc -O coff -o start-coop.res.o
x86_64-w64-mingw32-gcc -std=c11 -Os -s -Wall -Wextra -Werror -municode -mwindows -static-libgcc -finput-charset=UTF-8 start-coop.c start-coop.res.o -o ..\..\ABRIR-COOPERATIVO.exe -luser32
```

Linux cross-build uses the same commands with forward slashes in the output
path. The delivered binary uses the local `gcc-posix` executable from the GCC 13
MinGW-w64 toolchain described in `../BUILD.md`; standard GCC 13 works as well.
The resource object is an intermediate file and need not be distributed.

The icon is the unmodified openOMSI icon from the corresponding upstream source
tree (`assets/icons/app/openomsi.ico`). Source license: AGPL-3.0-only, as the
containing project. The program and icon do not imply upstream endorsement.
