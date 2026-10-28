param(
    [string]$CaptureDirectory = (Join-Path $env:LOCALAPPDATA 'OpenOmsi-BBS\EventosBeta'),
    [switch]$PreflightOnly,
    [switch]$NoPause,
    [ValidateRange(1,120)][int]$StartupTimeoutSeconds = 30
)
$ErrorActionPreference = 'Stop'
$version = '0.1.2'
$root = Split-Path $PSScriptRoot -Parent
$agent = Join-Path $PSScriptRoot 'bcs-event-probe.jar'
$java = Join-Path $root 'jre\bin\java.exe'
$launcher = Join-Path $root 'launcher.exe'
$diagnostic = $null
$exitCode = 0

function Record([string]$Message) {
    Write-Host $Message
    if ($script:diagnostic) { Add-Content -LiteralPath $script:diagnostic -Value ((Get-Date -Format o) + ' ' + $Message) -Encoding UTF8 }
}

function Invoke-Beta {
    New-Item -ItemType Directory -Path $CaptureDirectory -Force | Out-Null
    $script:diagnostic = Join-Path $CaptureDirectory ('diagnostico-inicio-' + (Get-Date -Format 'yyyyMMdd-HHmmssfff') + '.txt')
    Record "BCS event capture beta $version"
    Record "Pasta de capturas: $CaptureDirectory"
    if (!(Test-Path -LiteralPath $agent)) { throw 'Extraia o ZIP completo: bcs-event-probe.jar ausente.' }
    if (!(Test-Path -LiteralPath $java)) { throw 'Java do BCS nao encontrado. Extraia Beta-Eventos-BCS dentro de OMSI 2\Busbetrieb-Simulator e envie este diagnostico.' }
    if (!(Test-Path -LiteralPath $launcher)) { throw 'launcher.exe do BCS nao encontrado na pasta acima do beta.' }

    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $admin = ([Security.Principal.WindowsPrincipal]$identity).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    # Configure the scoped agent variable AFTER the elevation required by the official launcher.
    if (!$PreflightOnly -and !$admin) {
        Record 'Solicitando a mesma elevacao exigida pelo launcher oficial; o agente sera configurado depois dela.'
        $arguments = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', ('"' + $PSCommandPath + '"'), '-CaptureDirectory', ('"' + $CaptureDirectory + '"'))
        if ($NoPause) { $arguments += '-NoPause' }
        $arguments += @('-StartupTimeoutSeconds', $StartupTimeoutSeconds)
        $child = Start-Process -FilePath 'powershell.exe' -ArgumentList $arguments -Verb RunAs -Wait -PassThru
        $script:exitCode = $child.ExitCode
        return
    }

    $check = Join-Path $CaptureDirectory ('diagnostico\jre-' + (Get-Date -Format 'yyyyMMdd-HHmmssfff'))
    New-Item -ItemType Directory -Path $check -Force | Out-Null
    $stdout = Join-Path $check 'java-stdout.txt'
    $stderr = Join-Path $check 'java-stderr.txt'
    $probe = Start-Process -FilePath $java -ArgumentList @(('"-javaagent:' + $agent + '=' + $check + '"'), '-version') -Wait -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $ack = @(Get-ChildItem -LiteralPath $check -Filter 'bcs-events-*.json' -File)
    if ($probe.ExitCode -ne 0 -or $ack.Count -eq 0) { throw "O Java do BCS nao confirmou o agente. Nao espere um evento. Envie a pasta diagnostico e este arquivo. Saida Java: $($probe.ExitCode)." }
    Record 'PRE-TESTE OK: o Java instalado carregou o agente e gravou um arquivo. Isso ainda nao confirma a captura no BCS.'
    if ($PreflightOnly) { Record 'Diagnostico concluido sem abrir o BCS ou acessar a conta.'; return }

    $running = @(Get-CimInstance Win32_Process -Filter "Name='java.exe' OR Name='javaw.exe'" | Where-Object { $_.CommandLine -match 'busbetrieb-simulator[^\"]*\.jar' })
    if ($running.Count -gt 0) { throw 'Ha um BCS Java aberto. Feche o BCS e execute Iniciar-Beta.bat novamente; abrir outra janela pode reutilizar a sessao sem agente.' }
    $launchTime = [DateTime]::UtcNow
    $previous = $env:JAVA_TOOL_OPTIONS
    try {
        $env:JAVA_TOOL_OPTIONS = ($previous + ' -javaagent:"' + $agent + '"="' + $CaptureDirectory + '"').Trim()
        $launchInfo = New-Object System.Diagnostics.ProcessStartInfo
        $launchInfo.FileName = $launcher
        $launchInfo.WorkingDirectory = $root
        $launchInfo.UseShellExecute = $false
        $launchInfo.EnvironmentVariables['JAVA_TOOL_OPTIONS'] = $env:JAVA_TOOL_OPTIONS
        [System.Diagnostics.Process]::Start($launchInfo) | Out-Null
        Record 'Launcher oficial iniciado com o agente configurado nesta arvore de processos.'
        $limit = [DateTime]::UtcNow.AddSeconds($StartupTimeoutSeconds)
        while ([DateTime]::UtcNow -lt $limit) {
            $files = @(Get-ChildItem -LiteralPath $CaptureDirectory -Filter 'bcs-events-*.json' -File | Where-Object { $_.LastWriteTimeUtc -ge $launchTime })
            foreach ($file in $files) {
                try { $state = Get-Content -LiteralPath $file.FullName -Raw | ConvertFrom-Json } catch { continue }
                if ($state.status -eq 'compatible') { Record 'CAPTURA ATIVA: BCS reconhecido. Ja ha status sendo gravado, mesmo sem evento.'; return }
                if ($state.status -in @('unsupported_bcs_build', 'capture_error', 'observer_failed')) { throw "O agente iniciou, mas o status e $($state.status). Nao espere um evento; envie os arquivos da pasta de capturas." }
            }
            Start-Sleep -Milliseconds 500
        }
        if ($files.Count -eq 0) { throw 'O launcher abriu, mas nenhum processo Java confirmou o agente na pasta de capturas. Nao espere um evento. Envie os diagnosticos.' }
        Record 'AGENTE CARREGADO, aguardando a viagem/gerenciador de eventos. Isso ainda nao confirma CAPTURA ATIVA.'
        Record 'Depois de iniciar a viagem, use Abrir-Capturas.bat e confira CAPTURA ATIVA antes de esperar um evento.'
    } finally { $env:JAVA_TOOL_OPTIONS = $previous }
}

try { Invoke-Beta } catch {
    $exitCode = 1
    try { Record ('FALHA: ' + $_.Exception.Message) } catch { Write-Host 'Nao foi possivel gravar o diagnostico. Verifique a permissao da pasta de capturas.' }
}
if (!$NoPause) { Read-Host 'Pressione Enter para fechar esta janela' | Out-Null }
exit $exitCode
