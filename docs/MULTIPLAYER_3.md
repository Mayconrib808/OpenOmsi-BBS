# BBS+OpenOmsi Multiplayer 3.0

Leve suas viagens do **Bus Company Simulator / Busbetrieb-Simulator (BCS/BBS)** para o **multiplayer do openOMSI**. O BCS escolhe a viagem e recebe os dados da condução; o openOMSI simula o mundo compartilhado. Jogadores no mesmo mapa podem encontrar os ônibus uns dos outros.

A primeira candidata pública é **3.0.3-rc.3**. A numeração continua depois dos testes 3.0.2-dev.13. A candidata precisa de uma viagem completa com dois computadores antes de ser apresentada como lançamento definitivo no WebDisk.

## Requisitos

- Windows 10/11 de 64 bits; OMSI 2 e BCS adquiridos e instalados pelo jogador.
- BCS **5.0.2.0**. O pacote não contém nem modifica o JAR, a DLL ou o plugin original da PeDePe.
- Cliente e servidor openOMSI com **protocolo 7**, introduzido na **0.2.48**. A 0.2.47 não foi publicada oficialmente. O servidor incluído é o oficial **0.2.50**, sem alterações.
- O mapa, suas dependências e o próprio ônibus instalados no OMSI 2 de cada jogador.
- Internet no computador que hospeda a sessão. Esse computador precisa continuar ligado enquanto seus colegas jogam nela.

O download automático do cliente verifica o SHA-256 da release oficial. Em **Opções avançadas → Cliente → Manual**, você pode selecionar seu `openomsi.exe`, inclusive uma instalação 0.2.50 que já usa DLSS/ReShade. O programa não ativa nem instala efeitos gráficos.

## Configurar uma vez

1. Extraia **todo o ZIP** em uma pasta permanente. Não execute o Setup de dentro do ZIP.
2. Feche o BCS e abra **Setup.exe**. Escolha Português, English ou Deutsch.
3. Confira a pasta original do OMSI 2. O Setup tenta encontrá-la pelo Steam.
4. Escolha **Criar sala multiplayer para minha empresa**, ou cole o **convite da empresa** recebido do responsável. Informe seu nome de jogador.
5. Ao criar a sala, copie o fuso de referência e a **Mudança de horário** das definições da empresa no BCS. Esse valor não é necessariamente o fuso do seu computador. Digite `0` quando o BCS indicar zero.
6. Salve. Com o BCS fechado, clique em **Configurar BCS**. O programa faz backup das configurações antes de apontar o lançamento nativo para seu adaptador.
7. Abra o BCS. Confira **OpenOMSI (Beta)** e a pasta **OpenOMSI-BBS/PeDePeAdapter**. Se o formato das configurações não for reconhecido, o Setup oferece a pasta para você copiar e selecionar no próprio BCS.
8. Comece a viagem pelo BCS. Na primeira utilização, aguarde o download do cliente e do túnel, e o carregamento do mapa.

Não selecione a pasta do cliente real dentro do BCS: o BCS precisa iniciar o adaptador. O executável real é escolhido no Setup.

## Host, convite e mapas

O **convite da empresa** é permanente e serve para todos os mapas. O primeiro jogador de cada mapa reserva a sessão e inicia um servidor local independente. Quem entra depois nesse mapa usa a sessão existente.

| Situação | Resultado |
| --- | --- |
| Dois membros usam o convite e escolhem Berlin | Entram na mesma sessão de Berlin. |
| Um escolhe Berlin e outro escolhe Carrão City | São duas sessões, com servidores e túneis separados. Eles não se veem entre mapas diferentes. |
| Alguém tem apenas o link `trycloudflare.com` do servidor de Berlin | Esse link aponta para Berlin. Não encaminha automaticamente uma viagem no Carrão City. Use o convite da empresa no Setup. |
| O motorista que abriu a sessão fecha seu jogo, mas deixa o PC ligado | O servidor continua enquanto houver colegas nele. |
| O host desliga o PC ou perde a internet | Os colegas perdem essa sessão. Uma nova entrada pode eleger outro host; não há migração contínua do mundo em andamento. |
| Todos saem | O servidor encerra depois do período de espera; o convite continua válido. |

O computador que abriu Berlin não precisa hospedar Carrão City. O primeiro membro que entrar em Carrão City hospeda esse mapa com seus próprios arquivos.

Cada motorista dirige seu próprio modelo. Se outro computador não possuir esse modelo, o openOMSI pode representá-lo por um substituto. Para ver o modelo exato, instale o mesmo ônibus e a mesma skin. O pacote não redistribui mapas, ônibus privados ou DLCs.

## Tabela, letreiro, finalização e próxima viagem

O BCS entrega o ônibus, a situação, a linha, o turno, a viagem, a pintura e o clima. A integração prepara o início do serviço e preserva o combustível do ônibus selecionado. A configuração de tabela e letreiro depende também dos arquivos do mapa, HOF e scripts do ônibus.

Na **5.0.2.0**, o plugin nativo da PeDePe recebe o comando de próxima viagem e seleciona o serviço no jogo sem reinício. A 3.0 encaminha esse plugin original, byte por byte. Ela não usa o patch de JAR da linha 2.1.

Finalize pelo **fluxo nativo do BCS**, mantendo o openOMSI aberto até o BCS confirmar o resultado. O procedimento antigo **F9 → aguardar dois segundos** pertence ao modo legado da Bridge 2.x e não é a instrução de finalização da 3.0. O novo BCS lê os contadores e a avaliação pela telemetria do plugin.

Depois de finalizar, escolha **Próxima viagem** no BCS. Confira a nova tabela e o letreiro antes de partir. Se não mudar, colete os logs e informe mapa, ônibus/HOF e viagem; não reinicie a hospedagem dos colegas como tentativa automática de correção.

## Atualizações, desativação e suporte

As atualizações da integração usam o canal **Cloudflare 3.x**, com verificação de origem, identidade, assinatura/registro de publicação e SHA-256. A preferência de atualização automática e a busca manual continuam disponíveis. Feche o BCS antes de instalar. A empresa, o jogador e as opções ficam preservados.

O cliente e o servidor oficiais têm escolhas próprias de atualização. O modo **Manual** mantém a sua escolha. **Recomendado** usa o servidor 0.2.50 do pacote. Protocolo 6 não é usado como fallback silencioso.

**Desativar multiplayer** mantém as viagens solo no openOMSI. **Desativar programa**, com o BCS fechado, volta ao lançamento do OMSI original. Os controles valem para novas viagens e não interrompem sessões já abertas.

Use **Coletar logs** no Setup. O ZIP contém apenas logs do programa e informações de versão; não inclui as chaves da empresa ou o JAR. Revise o arquivo antes de compartilhar. Suporte: [Issues do projeto](https://github.com/Mayconrib808/OpenOmsi-BBS/issues/new/choose) ou Discord **`.zmaycon.`**, incluindo os pontos nas duas pontas.

## O que foi verificado

Foram verificados os argumentos do JAR 5.0.2.0, o plugin original em simulação, os transportes UDP/WebSocket de protocolo 7, a preservação da autenticação e de dados binários, o isolamento de mapas, combustível, prontidão do BCS, preferências e atualização transacional. Veja o [registro de validação](VALIDATION_3.md).

A simulação de um plugin não comprova uma viagem real. A candidata precisa de testes no Windows com dois jogadores, conclusão/avaliação e próxima viagem nos mapas usados pela empresa. Não há promessa de paridade para uma penalidade que o motor não exponha, nem promessa de zero detecções em todos os antivírus.

Projeto comunitário independente de Mayconrib808. Não é um produto oficial, parceria ou aprovação comercial da PeDePe ou do openOMSI.
