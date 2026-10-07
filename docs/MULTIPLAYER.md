# Multiplayer da empresa — 2.0.0-dev.1

Esta é a primeira versão de desenvolvimento da integração. A entrada é automática depois de salvar um perfil da empresa no Setup. A sessão precisa estar funcionando em um anfitrião ou servidor openOMSI. A ponte não cria servidores na nuvem e não identifica automaticamente a empresa da conta BBS.

Referência de implementação: openOMSI 0.2.0, protocolo 6, código `538ad31b2a2c664cb0726db2547bf6238411eedf`. Os testes automatizados verificam perfis, arquivos, seleção de sessão e sinais de conexão simulados. A viagem BBS com dois jogadores, os ônibus físicos e o embarque entre jogadores continuam pendentes de validação real. O teste de viagem da 1.1.3 não certifica esta versão.

## Para o jogador

1. Configure as pastas e a ativação pelo Setup, como na 1.1.3.
2. Com os jogos fechados, escolha **8 — Configurar / desativar multiplayer da empresa**.
3. Informe o arquivo JSON ou o link HTTPS fornecido pelo administrador e seu nome no multiplayer. Salve uma vez.
4. Instale o mapa e a frota da sessão nas versões indicadas pelo administrador, na instalação original do OMSI 2.
5. Inicie a viagem normalmente pelo BBS. A ponte procura sessões cadastradas com o mesmo mapa e a mesma data, confere o horário ao vivo e abre o openOMSI com `--lan-join` e `--lan-name`.

Se faltar um ônibus ou seus arquivos forem diferentes, abre uma página local com o nome do pacote, os arquivos afetados e o **link cadastrado pelo administrador**. Nenhuma pesquisa de downloads é feita. O jogador abre a página do fornecedor e instala o pacote; a ponte não extrai ZIPs e não copia os arquivos de colegas.

Todos precisam ter a frota oferecida pela sessão para enxergar os modelos e as pinturas corretos. Dirigir ônibus diferentes é permitido. Arquivos extras dentro das pastas registradas também são apontados: uma pintura adicional pode mudar o índice de pintura enviado pela rede. A ponte preserva os arquivos e informa a diferença; não remove conteúdo.

Para jogar uma viagem sozinho, desative apenas o multiplayer na opção 8. A ponte BBS continua ativada. O multiplayer vem **desativado por padrão**, inclusive ao carregar uma configuração antiga.

## Para o administrador

O perfil deve representar os arquivos usados pela empresa e pelo anfitrião. A opção **9 — Criar perfil da empresa** ajuda a gerar um JSON:

1. Informe um identificador estável e o nome da empresa.
2. Cadastre uma sessão: nome, `maps/<mapa>/global.cfg`, nome do mapa no BBS, data do mundo e endereço HTTP(S) do servidor.
3. Informe a versão e a página de download do mapa.
4. Cadastre os ônibus da frota, versões e links. O assistente registra os arquivos da pasta do ônibus, incluindo scripts, modelos, sons e pinturas.
5. Adicione as pastas de dependências externas necessárias: Sceneryobjects, Splines, Texture, Fonts etc. O assistente não descobre sozinho todas as dependências dos addons.
6. Cadastre outras sessões se necessário. Compartilhe o JSON criado em `Companies`, ou hospede esse mesmo JSON em um endereço HTTPS que entregue o conteúdo do arquivo, e compartilhe o link.

Os jogadores configuram esse arquivo/link uma vez. Um perfil remoto é lido novamente a cada viagem, permitindo atualizar endereços e links sem reconfigurar cada cliente. Um JSON compartilhado como arquivo precisa ser reenviado quando mudar. Não altere `company_id`: trocar a identidade exige configurar novamente a empresa no Setup.

Para mudar links, datas e salas, edite o JSON. Quando mudar os arquivos de mapa, ônibus, dependências ou pinturas, regenere os hashes pela opção 9 usando outro identificador temporário; transfira os pacotes atualizados ao perfil da empresa e preserve seu `company_id`. O assistente recusa sobrescrever um perfil existente. Os hashes conferem os arquivos locais; não hospedam os mods nem certificam que um link externo é confiável.

O perfil organiza o destino das conexões. **Não é uma autenticação de funcionário no BBS nem uma lista de acesso do servidor.** A associação com a empresa é escolhida no Setup; o acesso à sessão depende do anfitrião e de sua rede.

## Endereço e configuração do servidor

`server_url` é a porta **web**, não a porta UDP e não o código de sessão OMSI. Exemplo local: `http://127.0.0.1:27025`. Para outros computadores, use o endereço alcançável por eles. Para internet, um endereço HTTPS do gateway/túnel pode ser usado. `127.0.0.1` só serve para cliente e servidor na mesma máquina.

A ponte lê `GET /status` no mesmo endereço que passa ao openOMSI. O status precisa informar mapa, versão, protocolo, relógio, quantidade de jogadores e frota. Status fora do ar, incompatível ou com a sala cheia impede a entrada. A primeira sala compatível é escolhida na ordem do perfil. O perfil aceita até 16 salas e consulta no máximo quatro ao mesmo tempo.

O servidor deve restringir `vehicles` aos ônibus dos pacotes registrados. Se oferecer um modelo que o perfil não contempla, a ponte avisa ao administrador para incluir o pacote ou limitar a frota. Isso evita que um jogador entre com um ônibus que os demais não têm.

Trecho ilustrativo de `server.cfg` (substitua o mapa e os caminhos dos ônibus pelos seus):

```ini
map = maps/SeuMapa/global.cfg
date = 2026-10-06
time = 09:20
time_speed = 1
real_time = 0
port = 27015
web_port = 27025
max_players = 16
vehicles = Vehicles/OnibusA/A.bus;Vehicles/OnibusB/B.bus
```

O servidor dedicado pertence ao openOMSI, instalado separadamente. Consulte sua [documentação oficial](https://github.com/openOMSI-Project/openOMSI/blob/538ad31b2a2c664cb0726db2547bf6238411eedf/docs/SERVER.md). O ZIP da ponte não inclui mapas, ônibus ou o jogo/servidor openOMSI.

## Data e relógio

Cada sessão declara uma data fixa `YYYY-MM-DD`. Na ponte, a data automática ainda é a data local do Windows, como na 1.1.3; se escolher outro dia no calendário do BBS, configure `date=YYYY-MM-DD` em `app/bridge.ini`.

`clock_tolerance_seconds` aceita 1 a 300 segundos; o assistente usa 180. Uma sessão às 09:20 pode receber uma partida próxima desse horário. Uma viagem às 18h não entra nela. A ponte não muda o relógio de uma sala ocupada, não troca a viagem escolhida no BBS e não escolhe outro mapa para conseguir conectar.

O relógio é conferido de novo antes do lançamento. O status HTTP do openOMSI não expõe a data do mundo. Por isso, depois de abrir o filho openOMSI, a ponte espera o log confirmar a data e o horário realmente recebidos do anfitrião. O sinal de pronto para o BBS só é publicado depois dessa confirmação e do carregamento do veículo. Uma falha de conexão, data diferente ou horário incompatível encerra o lançamento iniciado pela ponte. Não há retorno silencioso ao single-player quando o multiplayer foi solicitado.

Se o openOMSI não confirmar o mundo em dois minutos, o lançamento é encerrado. Após a confirmação inicial, a ponte não monitora continuamente o relógio nem implementa recuperação própria de conexão: o comportamento de queda/reentrada do openOMSI e a avaliação BBS nessa situação precisam ser testados. O tempo de carregamento de mapas grandes também precisa entrar nessa validação.

## Arquivos e downloads

Cada pacote contém `id`, `name`, `version`, `download_url`, `folders` e `files`. `files` lista caminhos relativos à instalação OMSI 2 e SHA-256. `folders` define as pastas completas cujo inventário é conferido. Use o assistente para gerar os hashes; não preencha um hash de exemplo como se fosse real.

O cliente usa a instalação original que foi conferida e o ZIP temporário de horários da ponte. No processo multiplayer, conteúdo adicional do openOMSI é isolado e a transferência automática de mods entre colegas é desativada. Esse ajuste se aplica ao filho iniciado pela ponte; não altera as configurações ou o ambiente do Windows.

Arquivos `.hof` não entram no inventário, pois são preparados pelo BBS a cada viagem. Situações salvas `.osn` também ficam fora: ônibus, pintura e posição salvos diferem entre jogadores. Executáveis, DLLs, logs e perfis de conta também não entram. Somente arquivos de conteúdo do jogo podem ser exigidos. Nenhum arquivo de jogo é enviado pela ponte.

O exemplo [company.example.json](../examples/company.example.json) serve para entender os campos. Seus links e hashes são fictícios e precisam ser substituídos.

## Testes que ainda faltam

- Uma viagem BBS completa com multiplayer ativo, inclusive painel, passageiros, tickets, conforto e avaliação.
- Dois jogadores dirigindo modelos diferentes, com pinturas iguais às vistas pelo colega.
- Sair do ônibus, embarcar no ônibus do colega e retornar ao próprio posto de motorista.
- Viagens diferentes com partidas próximas no mesmo mundo; recuperação após queda/reentrada.
- Mapas grandes, dependências compartilhadas, carregamentos demorados e finalização independente das duas viagens.

Sem outro jogador, é possível testar instalação, Setup, perfil, relatório de requisitos e o fluxo de viagem normal com multiplayer desativado. Com um servidor local, um único cliente também consegue testar a entrada, o relógio e a viagem BBS; isso não comprova a visualização de outro ônibus.

## English overview

This is a development build of optional company multiplayer, targeting openOMSI 0.2.0 / protocol 6. Setup option 9 generates administrator-owned JSON profiles, asset-folder inventories, hashes and curated download links. Setup option 8 binds one local file or HTTPS profile URL and a player name once. Starting a BBS trip selects an already hosted compatible session, validates its status and local content, then adds `--lan-join` / `--lan-name`.

The profile is a routing configuration, not BBS company-membership authentication. Registered sessions have a fixed map and date, a web-gateway base URL, required package IDs and a 1-300 second clock tolerance. The HTTP status omits the date; the launched game's host-world log must confirm it before BBS readiness is published. Failed or mismatched startup is stopped. Runtime reconnect and BBS evaluation behavior remain unverified.

Missing, changed or extra assets produce a local requirements page with the administrator's links. No search, automatic archive installation, peer asset upload or game-file redistribution is performed. Multiplayer is disabled by default and can be disabled separately from the bridge. Two-player rendering, boarding and live BBS trip evaluation have not been validated.
