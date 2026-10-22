/* SPDX-License-Identifier: AGPL-3.0-only
 * Unofficial openOMSI cooperative package, based on v0.3.38.
 * Starts only the program distributed beside this launcher.
 * Environment changes apply only to this process and its children.
 */
#ifndef UNICODE
#define UNICODE
#endif
#ifndef _UNICODE
#define _UNICODE
#endif
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdlib.h>
#include <wchar.h>

#define PATH_CAP 32768u

static wchar_t package_dir[PATH_CAP];
static wchar_t game_dir[PATH_CAP];
static wchar_t game_exe[PATH_CAP];

static int show_error(const wchar_t *detail, DWORD code) {
    wchar_t system_text[1024] = L"";
    wchar_t message[4096];
    if (code) {
        FormatMessageW(FORMAT_MESSAGE_FROM_SYSTEM | FORMAT_MESSAGE_IGNORE_INSERTS,
                       NULL, code, 0, system_text, 1024, NULL);
    }
    swprintf(message, 4096,
             L"Não foi possível abrir o OpenOMSI Cooperativo.\n\n%ls\n\n"
             L"%ls\nExtraia o ZIP inteiro para uma pasta separada e mantenha "
             L"a pasta programa junto deste executável.",
             detail, system_text);
    MessageBoxW(NULL, message, L"OpenOMSI Cooperativo 0.3.38-coop.2",
                MB_OK | MB_ICONERROR);
    return 1;
}

int WINAPI wWinMain(HINSTANCE instance, HINSTANCE previous,
                    PWSTR arguments, int show) {
    (void)instance;
    (void)previous;
    (void)show;
    DWORD length = GetModuleFileNameW(NULL, package_dir, PATH_CAP);
    if (!length || length >= PATH_CAP) {
        return show_error(L"O caminho desta cópia não pôde ser identificado.",
                          GetLastError());
    }
    wchar_t *separator = wcsrchr(package_dir, L'\\');
    if (!separator) {
        return show_error(L"O caminho desta cópia não pôde ser identificado.", 0);
    }
    *separator = L'\0';
    if (swprintf(game_dir, PATH_CAP, L"%ls\\programa", package_dir) < 0 ||
        swprintf(game_exe, PATH_CAP, L"%ls\\openomsi.exe", game_dir) < 0) {
        return show_error(L"O caminho da pasta é longo demais.", 0);
    }
    DWORD attributes = GetFileAttributesW(game_exe);
    if (attributes == INVALID_FILE_ATTRIBUTES ||
        (attributes & FILE_ATTRIBUTE_DIRECTORY)) {
        return show_error(L"O arquivo programa\\openomsi.exe não foi encontrado.", 0);
    }

    /* The standard updater/guard can replace a derivative with an official build.
     * Limit their opt-out to this launch, without editing user settings or files.
     * A separately configured third-party launcher must not redirect this copy.
     */
    if (!SetEnvironmentVariableW(L"OMSI_NO_UPDATE", L"1") ||
        !SetEnvironmentVariableW(L"OMSI_NO_GUARD", L"1") ||
        !SetEnvironmentVariableW(L"OPENOMSI_BIN", game_exe)) {
        return show_error(L"Não foi possível preparar a abertura desta cópia.",
                          GetLastError());
    }
    if (!SetEnvironmentVariableW(L"OMSI_LAUNCHER", NULL) &&
        GetLastError() != ERROR_ENVVAR_NOT_FOUND) {
        return show_error(L"Não foi possível selecionar o launcher deste pacote.",
                          GetLastError());
    }

    if (!arguments) arguments = L"";
    size_t capacity = wcslen(game_exe) + wcslen(arguments) + 4;
    if (capacity > PATH_CAP) {
        return show_error(L"Os argumentos de abertura são longos demais.", 0);
    }
    wchar_t *command = calloc(capacity, sizeof(*command));
    if (!command) {
        return show_error(L"Memória insuficiente para abrir o jogo.", 0);
    }
    if (swprintf(command, capacity, L"\"%ls\" %ls", game_exe, arguments) < 0) {
        free(command);
        return show_error(L"Não foi possível preparar os argumentos do jogo.", 0);
    }
    STARTUPINFOW startup = {0};
    PROCESS_INFORMATION process = {0};
    startup.cb = sizeof(startup);
    BOOL started = CreateProcessW(game_exe, command, NULL, NULL, FALSE, 0,
                                  NULL, game_dir, &startup, &process);
    DWORD error = started ? 0 : GetLastError();
    free(command);
    if (!started) {
        return show_error(L"O Windows não conseguiu iniciar programa\\openomsi.exe.",
                          error);
    }
    CloseHandle(process.hThread);
    CloseHandle(process.hProcess);
    return 0;
}
