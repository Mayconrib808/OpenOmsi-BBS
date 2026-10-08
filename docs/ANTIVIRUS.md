# Auxiliar e análise do Defender — 2.0.3-dev.2

Em 8 de outubro de 2026, o Defender do jogador apontou `Trojan:Win32/Wacatac.B!ml` ao baixar o ZIP da dev1, indicando `app/compat/omsi-plugin-host32.exe` dentro do arquivo. A detecção durante o download pode analisar os executáveis do ZIP antes de sua extração.

O auxiliar é compilado dos quatro arquivos de `source/pluginhost/`, usando apenas a biblioteca padrão do Go. Carrega a DLL `bbs.dll` instalada localmente e usa pipes para conversar com a ponte. Não baixa código, não configura o antivírus e não altera a DLL do fornecedor. A reconstrução independente da dev1 reproduziu o SHA-256 publicado: `1c7f1f25bd18edb2b010ec42de2e34db5f3fc8ed2b1d948003635528b8a11a1a`.

Uma análise do ZIP original e dos seis executáveis no [Windows de CI](https://github.com/Mayconrib808/OpenOmsi-BBS/actions/runs/37812097177) terminou sem detecções. A proteção em tempo real e a nuvem estavam ativas; o mecanismo era `1.1.26080.3`, com assinaturas `1.459.601.0`. O ambiente é Windows Server 2022, diferente do Windows 11 do jogador. Não foi estabelecida a causa exata do alerta, nem obtida uma decisão de falso positivo da Microsoft.

## Mudanças na dev2

O auxiliar recebe recursos padrão VERSIONINFO e o ícone do projeto. O Windows pode ler seu autor, produto, descrição e versão. O executável conserva os símbolos, as informações de depuração e o build ID normal do Go. A compilação usa um módulo local isolado, sem dependências externas, com caminho fixo e `-trimpath`. O código que carrega a DLL, processa mensagens e envia respostas é preservado; o teste exige que a reconstrução com as opções antigas reproduza exatamente o auxiliar da dev1.

Esses recursos são identificação de produto, **não Authenticode**. O projeto continua sem certificado de assinatura de código. Não são usados empacotadores, arquivos protegidos por senha, regras de exclusão ou desativação do Defender.

## Análise antes da publicação

`scripts/scan_release_package.ps1` confere o SHA-256 do ZIP candidato e copia os mesmos bytes para uma pasta fora do workspace do CI. Ativa proteção em tempo real, análise de downloads e proteção em nuvem no runner descartável, atualiza assinaturas e valida a conexão MAPS. Aplica o marcador de zona de Internet ao ZIP candidato; isso não equivale ao download pelo navegador do jogador.

O comando personalizado `MpCmdRun -Scan -ScanType 3 -DisableRemediation` analisa o ZIP e cada um dos seis executáveis, sem ignorar os arquivos por exclusões e sem executar os programas. Os executáveis extraídos são comparados com o manifesto do pacote. Qualquer detecção, erro de análise ou falta de proteção/conexão bloqueia a publicação. O publisher também valida o manifesto completo, a origem dos fontes, os recursos do auxiliar e a conclusão dessa etapa.

O artefato **Defender-package-report** contém SHA-256, resultado por arquivo, mecanismo, assinaturas, estado da proteção e registros de ameaças. O relatório não faz parte do ZIP que ele analisa: adicioná-lo depois mudaria os bytes publicados.

Uma análise limpa nesse runner não garante o resultado de outro antivírus nem de outra máquina. A confirmação do caso relatado exige baixar a dev2 no computador afetado com o Defender ativo. Se o alerta continuar, registre nome da detecção, arquivo afetado e versão das assinaturas para a análise; não é necessário executar um arquivo bloqueado.
