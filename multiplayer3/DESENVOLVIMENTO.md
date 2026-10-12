# BBS+OpenOmsi Multiplayer 3.0.2-dev.7

Base recuperada da 3.0.1 e correções de lançamento/dados reais portadas da 2.1.5.
O campo do cliente vazio consulta a última release oficial estável Windows x64 na primeira configuração. O cliente escolhido fica salvo para as próximas viagens; clientes existentes configurados são preservados. O download verifica o SHA-256 oficial e mantém o cache separado por versão.

- Setup PT-BR/EN/DE: empresa ou convite, detecção de OMSI pelo Steam, pastas avançadas opcionais.
- Abrir carrega dados; não ativa nem verifica o BCS. A configuração é um clique explícito, exige BCS fechado e cria backup.
- A edição de config.cfg usa o formato observado, não uma API oficial. Formatos desconhecidos são recusados e oferecem a pasta exata na interface.
- Lua da PeDePe encaminhado byte por byte, motorista com contadores realmente salvos e proteção contra mudança de identidade/arquivo.
- Viagens BCS completas usam autostart e expansão da situação para inicialização de timetable/IBIS; saves manuais e probes não são convertidos.
- Auxiliar próprio da 3.x na raiz OMSI; o auxiliar da 2.x, bbs.start, JAR, DLL e Omsi.exe ficam preservados.
- Convite por empresa, lease por empresa/mapa e host automático em um processo independente. O lançador do BCS espera só pelo próprio jogo; a hospedagem continua para os colegas e encerra após a sala ficar vazia. Portas dinâmicas, relógio e capacidades nativas são verificados.
- Preferência de atualização independente do idioma; busca manual sempre disponível. Ed25519, SHA-256, produto/canal e origem separados da 2.x.
- Atualizador transacional só substitui seus componentes; configuração e documentos não são pré-requisitos nem são substituídos.

## Validação

`go test -race ./...` e `go vet ./...`; workflow próprio executa os testes em Windows e gera os executáveis sem recompilar Rust.
`python scripts/build_preview.py --server CAMINHO_SERVIDOR_3.0.1 --output SAIDA` verifica os hashes do servidor já compilado.
O ZIP completo contém a fonte MIT da integração e a licença do runtime openOMSI.

## Antes da versão pública

1. Implementar Próxima viagem por uma interface permitida e confirmar os requisitos da PeDePe; não portar o patch de JAR da 2.x.
2. Validar BCS real, finalizar viagem, troca de mapa, host/guest e saída do host em pelo menos dois computadores.
3. Validar instalação de atualizações Cloudflare em um computador Windows real. O feed 3.x já está publicado com assinatura; o canal 2.x não é usado nem alterado.
4. Migrar as opções da empresa para temperatura e limite de passageiros por mapa.

O servidor empacotado mantém a proveniência bbs-server.json da 3.0.1 (0.2.20-bbs-free2). Não é apresentado como o cliente 0.2.29 nem como a última versão oficial.

## 3.0.2-dev.7 — instalação independente

ZIP completo utilizável sem Bridge 2.x. Setup.exe instala o auxiliar próprio na configuração explícita do BCS. Cliente oficial baixado quando necessário; arte existente do programa e identificação Setup em inglês embutidas como recursos PE. Conteúdo do cliente não herda OMSI_CONTENT de uma instalação anterior. Próxima viagem e canal de atualização permanecem pendentes conforme indicado acima.

## BCS loading hotfix

Preserva stdout herdado do BCS e encaminha a saída real do jogo tanto para o game.log esperado pelo BCS quanto para multiplayer.log. Nenhuma mensagem de prontidão é fabricada; nenhum JAR é modificado. Padrão para novas salas: Europe/Berlin −8 horas; salas existentes preservam seu relógio, que deve ser corrigido pelo criador quando necessário.

## Controles e atualizações 3.0.2-dev.7

Desativar multiplayer persiste modo solo sem contato com o diretório, mantendo a integração nativa do BCS. Desativar programa exige BCS fechado, salva backup e desmarca o lançamento openOMSI somente se a pasta ativa é nosso adaptador; reativação usa Configurar BCS. Preserva sessões em andamento.

As atualizações desta integração usam exclusivamente o feed Cloudflare 3.x assinado, sem fallback para GitHub. Downloads do cliente oficial openOMSI usam as releases oficiais upstream; são independentes das atualizações da integração.

## 3.0.2-dev.7 — voltar à configuração

A tela salva e a conclusão manual oferecem Ver ou editar configuração. Volta aos campos salvos sem criar outra sala; conclusão quando o BCS está aguardando mantém o fluxo de iniciar a viagem. Erros de Configurar BCS são exibidos em detalhes e registrados em logs/setup.log sem convite ou token.

## Atualização automática de cliente e servidor

Novas configurações selecionam Automático para ambos. O cliente consulta a release oficial mais recente antes de cada viagem pelo BCS, inclusive com caminho já salvo. O cache só é reutilizado após verificação SHA-256. Manual preserva o executável escolhido sem consulta. Sem conexão ao serviço de releases, um cliente gerenciado anterior só é reutilizado se continuar verificado. Caminhos externos existentes continuam manuais.

Servidor Automático consulta a release oficial ao criar uma nova sessão. Sessões em uso não são interrompidas. Se /status não confirmar protocolo 6 e ônibus livres, o processo é encerrado e o servidor incluído compatível é iniciado uma única vez, com motivo no log. Servidor Manual mantém a escolha e erros explícitos.

Ao criar uma empresa, relógio de referência e mudança de horário começam vazios e são obrigatórios no navegador e no servidor. O exemplo mostra “Mudança de horário: -8 Horas”, conforme o print das Definições do BCS; cada empresa deve copiar seu próprio valor, incluindo 0 explicitamente quando apropriado. Entrar por convite usa o relógio do criador.
