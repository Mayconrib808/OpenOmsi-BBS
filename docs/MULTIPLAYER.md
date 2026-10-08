# Multiplayer da empresa — 2.0.3-dev.1

O multiplayer mantém a política permissiva do openOMSI. Hashes, arquivos extras, mudanças do BCS em `Holidays.txt`, scripts, repaints e modelos de outros jogadores não bloqueiam a entrada. A bridge verifica servidor/protocolo, vagas, relógio/data e se o ônibus escolhido é oferecido pela sessão. O openOMSI carrega os recursos locais; instalar o mapa e os ônibus continua necessário para conseguir jogar e visualizar os modelos.

## Servidor automático

**Setup → Empresa / servidor** e **Criar / atualizar empresa** abrem o novo **HostAgent.exe**. Importe o perfil atual ou detecte e cadastre mapas instalados. Cada mapa tem sua própria frota em lote, portas, vagas, tráfego, passageiros, tabela e tempo para fechar vazio. O agente mantém o relógio da empresa e abre um servidor dedicado quando recebe um pedido.

O PC do host precisa estar ligado com o agente ativo. A inicialização ao entrar no Windows é opcional. Fechar o painel deixa o agente em segundo plano; **Parar agente** encerra o agente e os servidores que ele abriu.

A pasta `relay/` contém o serviço do diretório online. Ele precisa ser publicado em uma URL HTTPS fixa e ligado ao aplicativo uma vez. Depois disso, os jogadores configuram o mesmo JSON/link uma única vez. O agente publica os novos endereços de túnel após verificar o servidor e a bridge consulta a URL fixa. A conexão do jogo continua passando pelo multiplayer nativo do openOMSI. Consulte [a configuração completa](AUTO_HOST.md).

## Jogadores

1. Atualize a bridge para 2.0.3 dev1 e abra Setup.
2. Selecione o OMSI 2 e o OpenOMSI. Ative o multiplayer e escolha o novo `.players.json` ou link `/profile` fornecido pelo administrador.
3. Informe seu nome e use **Salvar e ativar** com os jogos fechados.
4. Instale o mapa e os ônibus que pretende usar. Inicie a viagem pelo BCS normalmente.

Com um perfil automático, a bridge solicita o mapa e espera o carregamento. Pedidos simultâneos reutilizam o mesmo servidor. Se o host estiver offline ou o mapa desativado, isso é informado. Sem `directory_url`, o perfil antigo continua usando uma sessão iniciada manualmente.

Para jogar sozinho, desmarque o multiplayer e use **Salvar e ativar**. A bridge BCS continua disponível. O multiplayer vem desativado por padrão.

## Compatibilidade e relógio

As referências são openOMSI 0.2.0/0.2.11 e protocolo 6. O servidor dedicado 0.2.11 é a referência dos testes atuais. Sessões automáticas usam data `company`, um fuso IANA explícito e o ajuste em minutos do BCS. A data/horário do host é confirmada no log do mundo recebido pelo jogador. O endereço só aparece online após carregar o mundo, sincronizar o relógio e verificar o túnel HTTPS.

`server_url` é a base da porta web ou do túnel, não a porta UDP. O perfil admite até 16 sessões. Os mapas do agente têm portas independentes e podem ficar ativos simultaneamente. O `server.cfg` original é preservado; o agente mantém configurações próprias. Mapas vazios podem fechar após o tempo escolhido; mapas com jogadores permanecem ativos.

## Administração manual

`CompanyHost.exe` continua disponível para hospedagem manual com o JSON anterior. O assistente `Setup.exe --cli` mantém a criação de perfis e a auditoria explícita de inventários/hashes. Essas auditorias não passam a bloquear o multiplayer.

A identidade `company_id` é estável. Não troque de empresa mantendo o mesmo diretório; configure um perfil/diretório próprio. O perfil organiza conexões e não verifica a associação da conta BBS a uma empresa. A chave privada do host não pertence ao perfil dos jogadores.

## Validação

As verificações automáticas cobrem perfis, descoberta, pedidos de abertura, rotação de túnel, portas, seleção de frota, inatividade e controles Windows. O teste real de abertura sob demanda, dois jogadores, ônibus físicos, embarque e avaliação de viagem BCS ainda precisa ser concluído com os produtos instalados e o diretório publicado.

## English

Dev.4 adds a native host-agent panel and demand-driven dedicated servers. Configure the per-map fleet/options, deploy the included stable directory once, start the background agent and share the player JSON or `/profile` link once. The agent publishes newly verified tunnel addresses automatically. The host computer must remain on. Game traffic uses native openOMSI multiplayer; local hashes and extra files do not gate joining. Older manual profiles remain supported. Live two-player and BCS evaluation validation is pending.
