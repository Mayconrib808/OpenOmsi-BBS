# 2.0.3-dev.2 — identificação do auxiliar e análise do Defender

- Auxiliar com produto, autor, versão, descrição e ícone nas propriedades do Windows; conserva símbolos e identificação normal da compilação Go.
- Setup reconhece o auxiliar da dev1 para atualização e restaura o arquivo anterior em falhas de ativação.
- Mantém o código/protocolo do auxiliar, a abertura sem CMD e todas as funções da dev1, incluindo servidor automático, mapas, perfil, próxima viagem e clima.
- Testes com os bytes reais do auxiliar anterior, leitura de versão pela API nativa e comunicação com DLL sintética.
- Publicação exige análise do ZIP e dos seis executáveis pelo Defender com nuvem e proteção em tempo real ativas. Um resultado limpo no CI não confirma o fim do bloqueio no computador afetado; veja `docs/ANTIVIRUS.md`.

# 2.0.3-dev.1 — próxima viagem, clima e painel BCS

- Mantém a observação da próxima seleção por até 15 minutos após a conclusão e saída do jogo; confirma a transferência entre chamadas concorrentes e ignora conclusões duplicadas.
- Transfere o clima recente da situação do BCS. O servidor automático usa o clima do primeiro pedido de um novo mundo, sem alterar uma sessão ocupada. Atualize o agente e o serviço de `relay/` para essa transferência.
- Confirmação inicial compatível com UTF-16BE, UTF-16LE e UTF-8, capturada antes das janelas e sem perder respostas rápidas.
- Auxiliar de 32 bits sem console; mantém a comunicação por pipes e as janelas da DLL. O auxiliar da 2.0.1/dev.4 pode ser atualizado pelo Setup.
- Testes de regressão e teste nativo Windows das janelas da facade. Teste real das três correções continua necessário; veja `docs/releases/v2.0.3-dev.1.md`.

# 2.0.2-dev.4 — servidor automático

- Painel HostAgent com mapas instalados, importação do JSON atual, frota em lote e opções independentes por mapa.
- Servidores dedicados iniciados por pedido dos jogadores, reaproveitando pedidos simultâneos e fechando mapas vazios após o tempo configurado.
- Perfil online em URL fixa, com publicação automática dos novos endereços de túnel após verificação do servidor.
- Inicialização opcional ao entrar no Windows e interface PT/EN/DE. Inclui o serviço de diretório e o guia `docs/AUTO_HOST.md`.
- Multiplayer permissivo preservado: arquivos locais e hashes não bloqueiam a entrada. Perfis antigos sem diretório continuam no modo manual.
- A configuração inicial do serviço HTTPS e o teste real com dois jogadores ainda são necessários.

# 2.0.2-dev.2 — histórico: inicialização e conteúdo da empresa

- CompanyHost cria `server.cfg` quando ausente, já com mapa, relógio e frota do perfil. Configurações existentes, incluindo portas e limite de jogadores, são preservadas.
- A conferência ignora `*.osn.owt` e `*.osn_<x>_<y>.dds` na raiz do mapa, inclusive registros de perfis antigos. O `timezone.txt` real continua sendo verificado; cópias chamadas `timezone.txt.backup.txt` e `Holidays.txt.backup.txt` ficam fora do inventário.
- Alterações de sessão em `Holidays.txt`, arquivos `.bus`, scripts `.osc` e listas de variáveis podem usar o original do BBS como referência: o caminho precisa constar em `BBS_Backups.txt` e a cópia `.backup` precisa ter exatamente o SHA-256 cadastrado. Arquivos ausentes, backups diferentes e alterações em modelos, pinturas, horários ou fuso continuam bloqueando.
- Setup 9 → 3 permite revisar inclusões, remoções e hashes do inventário, preservando os dados da empresa e uma cópia do JSON anterior. A revisão do administrador exige os bytes atuais exatos.
- Relatórios idênticos de uma pasta não se repetem por modelo de ônibus; o assistente também evita cadastrar a mesma pasta de frota novamente na sessão.
- Testes cobrem a primeira configuração, situações salvas, fuso, alterações com/sem original comprovado, inventários BBS UTF-16 e atualização do perfil. A viagem real com o Solaris permanece pendente de teste no jogo.

# 2.0.2-dev.1 — preparação da 2.0.2

- Seleção de português, inglês e alemão na janela do Setup. A interface muda sem apagar os campos; o idioma é salvo junto com a configuração ao usar Salvar e ativar.
- Multiplayer identificado como opcional. Perfil e nome ficam desabilitados quando a opção está desligada, com orientação explícita de que podem ficar vazios.
- Arte do projeto no cabeçalho e ícone incorporado ao Setup.exe, à janela e à barra de tarefas.
- Verificação dos recursos incorporados e teste nativo dos idiomas e controles no Windows, com capturas para revisão visual.
- Revisão da descoberta do site e da interface de semáforos do OpenOMSI 0.2.14. Não foi identificado bloqueio público de rastreamento nas páginas verificadas; o motivo exato da indexação depende do Search Console. A multa de sinal vermelho continua dependendo de uma interface adicional no simulador.

# 2.0.1 — 2026-10-07

- Menu gráfico nativo para configurar, ativar e coletar logs.
- Perfil e nome do jogador persistem nas atualizações; o administrador exporta o perfil com o HTTPS verificado da sessão.
- Compatibilidade baseada em recursos do executável e no protocolo multiplayer, com verificações das versões oficiais 0.2.0 e 0.2.11.
- Correção de horários operacionais: 01:40 e 25:40 selecionam a mesma partida com offset lógico zero. Nenhum timetable instalado é alterado.
- Chamadas duplicadas da mesma viagem são ignoradas; transição para outro Schicht ID preserva a avaliação concluída.
- Tutorial simples e limitações documentadas. Testes reais de 0.2.11, Próxima viagem e dois jogadores continuam necessários; a API atual não permite reproduzir a multa específica por sinal vermelho.

# Changelog

## 2.0.2-dev.4

- Native Host Agent panel with map discovery, JSON import, bulk fleets and independent per-map server settings.
- Demand-driven dedicated servers with duplicate-request reuse and configurable idle shutdown.
- Stable online profile discovery and automatic publication of verified replacement tunnel addresses.
- Optional Windows-login startup, authenticated local agent controls, and PT/EN/DE interface checks.
- Preserves permissive multiplayer and legacy manual profiles; includes the directory service and `docs/AUTO_HOST.md`.

## 2.0.0-dev.5

- Fixed exact trip matching after midnight when BCS displays 00:20 and the map records 24:20. A unique route-compatible overnight departure is required; duplicate civil hours and unknown offsets still fail safely.
- Aligns only the selected overnight record in a temporary timetable ZIP, preserving its ordinal, other departures and the installed map bytes. Company date and world clock are unchanged.
- Added regression coverage for the reported 64-trip Carrão duty, opposite directions, duplicate departures and explicit extended hours.

## 2.0.0-dev.4

- Added administrator-only refresh of existing local company-profile hashes in Setup 9 → 2, preserving company/session/link/clock settings and the declared inventory, with an exact previous-JSON backup.
- Changed asset diagnostics to report changed content rather than claiming a known addon version difference. Player preflight still rejects unapproved script, calendar and other asset changes.
- Company-clock sessions now distinguish a dedicated gateway with no active world from a live clock mismatch; players are told to wait for CompanyHost synchronization. Fixed-date player-hosted rooms remain compatible.
- Added regression checks for the reported calendar/script drift, metadata and backup preservation, cancelled/concurrent updates, undeclared or missing files, and map loading status.

## 2.0.0-dev.3

- Fixed official OpenOMSI 0.2.0 server rejection: its HTTP status reports workspace version 0.1.0. Both the host supervisor and joining bridge now recognize the exact pinned 538ad31 release build with protocol 6.
- Unknown legacy builds, other protocols and OpenOMSI 0.2.9 remain unsupported. Diagnostics now include the actual reported version/protocol.

## 2.0.0-dev.2

- Added an explicit company reference timezone and BCS time shift, with portable timezone data and automatic company calendar selection.
- Added CompanyHost.exe to launch an independently installed OpenOMSI 0.2.0 server with the company clock and maintain it through local administrative clock corrections.
- Live sessions validate the shared company clock independently of each player's departure. Fixed-date profiles and manual historical choices remain supported.
- Setup explains/retries invalid company IDs and supports `company` dates with one-time BCS shift configuration.
- Real multiplayer rendering, boarding and BCS evaluation remain pending user testing.

## 2.0.0-dev.1 — development

- Optional company profiles loaded from a local JSON file or administrator HTTPS URL.
- Automatic selection of a compatible registered session when starting a BBS trip.
- Required asset hashes and full-folder inventories, with administrator-provided download links for missing or different bus/map/repaint packages.
- Server map, clock, capacity, version/protocol and declared-fleet checks; joined-world date/time confirmation before BBS readiness.
- Setup options 8 and 9 for player configuration and company-profile creation.
- Multiplayer disabled by default; CI artifacts only while real two-player BBS validation is pending.

This file distinguishes the imported software version from subsequent repository preparation. Earlier full runtime ZIPs are not represented as cleared GitHub releases.

## v1.1.3 — 2026-10-06

- Published the complete runtime ZIP and checksum as an experimental GitHub prerelease, with fixed download links in English and Portuguese. Publication verifies that all four executable hashes match the live-tested package.
- Recorded the completed Carrão City line 2201 / tour 02 live trip: timetable alignment, real bbs.dll startup, six stops, eight tickets, evaluation transfer and normal openOMSI shutdown.

- Replaced the excluded historical helper with an attributed MIT Go Windows x86 host implementing the pinned openOMSI protocol/ABI and documented BCS-specific startup environment.
- Bundled the helper automatically; users need no extra helper download. Added recognised-copy updates with rollback and exact-byte removal while preserving unknown files.
- Added protocol, lifecycle, idle-message-pump and native Windows stdcall mock-DLL tests, plus complete package integrity/staging tests.
- Builds now produce all four executables, the complete ZIP, tutorial in three languages, source, notices, inventory and checksums.
- Updated runtime/log filenames to v1.1.3 and added downloadable CI build artifacts. Original products/assets remain excluded.
- Automated validation is distinct from live BCS panel, game-trip and online evaluation checks; no vendor authorisation is claimed.

## Repository preparation — 2026-10-06

- Imported the v1.1.2 Go implementation without changing its runtime Go files.
- Added English and Brazilian Portuguese project documentation, MIT licensing for original bridge material, upstream notices and individual acknowledgements.
- Documented the `bbs.odr` write, the Registry launch redirect, bridge-owned runtime files and the limits of integration testing.
- Excluded the old prebuilt native helper, all prebuilt executables, the BCS interface screenshot and real diagnostic/game data.
- Replaced driver/session fixtures with freshly authored synthetic examples and updated the corresponding expected test values.
- Removed the test suite's dependency on a bundled native helper: general checks use a synthetic PE header, while exact historical-hash removal is an explicitly optional local test.
- Added repeatable portable tests, vetting, Windows cross-compilation and facade layout checks, plus a Linux/Windows CI workflow.
- No complete installable GitHub release is supplied by this import.

## v1.1.2 — imported package, 2026-10-06

- Added German setup messages and acceptance of `J`/`Ja` confirmations.
- Clarified that BCS/BBS's "Start OMSI faster" setting must remain unchecked.
- Highlighted the reported unresolved Berlin-Spandau loading issue under openOMSI 0.2.0.
- Kept the route matching, timetable-source checks, diagnostics and compatibility logic from the preceding implementation.
- Included the existing Discord support contact in setup messages.

## Earlier implementation context

v1.1.1 prioritised the physical final stop over the displayed terminus during route matching and expanded local timetable diagnostics. v1.1.0 reorganised setup, rejected ambiguous departure/source selection and refined the compatibility startup lifecycle. These summaries describe historical implementation intent; they do not constitute new live validation of those packages or universal map support.
