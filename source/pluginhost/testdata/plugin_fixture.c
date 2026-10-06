/* SPDX-License-Identifier: MIT
 * Copyright (c) 2026 Mayconrib808
 * Original synthetic DLL fixture. No vendor code or game data.
 * Built only for Windows tests; its binary is never put in the runtime ZIP.
 */
#define WIN32_LEAN_AND_MEAN
#include <windows.h>

int _fltused = 0;
static DWORD owner_thread;
static int context_ok, message_seen;
static HWND window;
static int equal(const WCHAR *a, const WCHAR *b) {
    while (*a && *a == *b) { ++a; ++b; }
    return *a == *b;
}
static LRESULT CALLBACK procedure(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp) {
    if (msg == WM_APP + 1) { message_seen = 1; return 0; }
    return DefWindowProcW(hwnd, msg, wp, lp);
}
void __stdcall PluginStart(void *owner) {
    static WCHAR cwd[32768], exe[32768], full_exe[32768], env[32];
    DWORD n, i;
    WNDCLASSW wc;
    owner_thread = GetCurrentThreadId();
    GetCurrentDirectoryW(32768, cwd);
    GetModuleFileNameW(0, exe, 32768);
    n = GetLongPathNameW(exe, full_exe, 32768);
    for (i = n; i > 0 && full_exe[i - 1] != L'\\'; --i) {}
    if (i) full_exe[i - 1] = 0;
    context_ok = !owner && n > 0 && n < 32768 && lstrcmpiW(cwd, full_exe) == 0;
    GetEnvironmentVariableW(L"SteamAppId", env, 32);
    context_ok = context_ok && equal(env, L"252530");
    GetEnvironmentVariableW(L"SteamGameId", env, 32);
    context_ok = context_ok && equal(env, L"252530");
    context_ok = context_ok && GetFileAttributesW(L"bbs.start") != INVALID_FILE_ATTRIBUTES;
    wc.lpfnWndProc = procedure;
    wc.style = 0;
    wc.cbClsExtra = 0;
    wc.cbWndExtra = 0;
    wc.hIcon = 0;
    wc.hCursor = 0;
    wc.hbrBackground = 0;
    wc.lpszMenuName = 0;
    wc.hInstance = GetModuleHandleW(0);
    wc.lpszClassName = L"BridgeSyntheticPlugin";
    RegisterClassW(&wc);
    window = CreateWindowExW(0, wc.lpszClassName, L"", 0, 0, 0, 0, 0, HWND_MESSAGE, 0, wc.hInstance, 0);
    context_ok = context_ok && window != 0;
    PostMessageW(window, WM_APP + 1, 0, 0);
}
void __stdcall AccessSystemVariable(WORD index, float *value, BYTE *write) {
    if (index == 0 && context_ok && GetCurrentThreadId() == owner_thread) { *value += 10.0f; *write = 1; }
}
void __stdcall AccessVariable(WORD index, float *value, BYTE *write) {
    if (index == 1 && context_ok && GetCurrentThreadId() == owner_thread) { *value *= 2.0f; *write = 1; }
}
void __stdcall AccessStringVariable(WORD index, WCHAR *value, BYTE *write) {
    static const WCHAR result[] = { L'P', L'r', L'o', L'n', L't', L'o', L' ', 0x20AC, L' ', 0xD83D, 0xDE8C, 0 };
    int i;
    if (index == 0 && context_ok && GetCurrentThreadId() == owner_thread) {
        for (i = 0; i < 12; ++i) value[i] = result[i];
        *write = 1;
    }
}
void __stdcall AccessTrigger(WORD index, BYTE *active) {
    *active = index == 2 && context_ok && message_seen && GetCurrentThreadId() == owner_thread;
}
void __stdcall PluginFinalize(void) {
    int ok = context_ok && message_seen && GetCurrentThreadId() == owner_thread;
    const char *result = ok ? "finalized" : "invalid";
    DWORD written;
    HANDLE file = CreateFileW(L"test-finalized.txt", GENERIC_WRITE, 0, 0, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, 0);
    if (file != INVALID_HANDLE_VALUE) { WriteFile(file, result, ok ? 9 : 7, &written, 0); CloseHandle(file); }
    if (window) DestroyWindow(window);
}
