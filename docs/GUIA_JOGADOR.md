# OpenOmsi - BBS 2.0.1 — guia do jogador

Escolha a viagem no BBS normalmente. A ponte prepara o OpenOMSI e procura a sessão cadastrada para o mapa e o relógio da empresa.

## Preparar uma vez

Você precisa de **OMSI 2, BCS/BBS e OpenOMSI para Windows** instalados, além do mapa e dos ônibus usados pela empresa. O administrador fornece o **perfil da empresa**, como arquivo JSON ou link HTTPS, e os links dos addons.

1. Feche o BCS, o OpenOMSI e o CompanyHost, se estiver aberto nesse computador.
2. Extraia **todo o ZIP OpenOmsi + BBS 2.0.1** e abra **Setup.exe**.
3. Em **Pasta do OMSI 2**, escolha a pasta que contém o `Omsi.exe` original. Em **Executável do OpenOMSI**, escolha o `openomsi.exe` do seu OpenOMSI.
4. Marque **Ativar multiplayer da empresa**. Em **Perfil da empresa (arquivo JSON ou link HTTPS)**, selecione o arquivo ou cole o link recebido. Digite **Seu nome no multiplayer**.
5. Clique em **Salvar e ativar** e aceite a permissão do Windows. O Setup guarda os caminhos, seu nome e uma cópia do perfil.
6. No BCS, em **Definições → Definições avançadas → OMSI**, deixe **Iniciar o OMSI mais depressa** desmarcado. No OpenOMSI, desative a sincronização com o relógio real.

**Você não precisa editar o JSON, descobrir seu endereço de rede ou abrir um servidor no seu PC para jogar na sessão da empresa.** O administrador prepara o servidor e informa o endereço no perfil.

## Jogar

1. Aguarde o administrador avisar que o servidor está pronto.
2. Abra o **BCS** e inicie sua viagem normalmente.
3. Espere o mapa carregar. Confira **Online: in …** dentro do OpenOMSI para confirmar sua conexão.
4. No ponto final: **F9 → aguarde pelo menos dois segundos → finalize a viagem no BCS com o OpenOMSI aberto**.

Para continuar, clique em **Próxima viagem** no BCS e aguarde a nova sessão. A ponte só inicia a transição depois de a viagem anterior ser concluída e de o BCS registrar outra viagem com identificador próprio. Deixe o OpenOMSI aberto durante essa troca; a ponte encerra a sessão anterior. Esse fluxo ainda precisa de teste real.

Se aparecer uma página de requisitos, siga os links dos addons fornecidos pelo administrador. Se houver erro de horário, versão, servidor ou conteúdo diferente, envie a mensagem ao administrador. O Setup oferece **Coletar logs** para gerar um diagnóstico local; revise os arquivos antes de compartilhar.

Para o chat, pressione **/** ou clique na caixa de mensagens, escreva e pressione **Enter**. **V** mostra ou esconde o chat. A tecla de entrada pode variar conforme os controles do OpenOMSI.

## Atualizar a ponte

Feche os jogos, extraia todo o pacote novo e abra seu **Setup.exe**. Confira os caminhos e o nome que já foram recuperados e clique em **Salvar e ativar**. A cópia do perfil fica nas configurações do usuário; mover ou apagar o ZIP antigo não exige refazer o perfil.

Se o administrador enviar um perfil atualizado em arquivo, selecione esse novo arquivo no Setup. Perfis fornecidos por link HTTPS são consultados pela ponte ao preparar a viagem; o endereço precisa continuar disponível.

Para voltar ao OMSI original, feche os jogos e clique em **Desativar ponte**.

## Teste com duas pessoas

O administrador precisa manter uma sessão pronta para o mapa escolhido.

1. Cada jogador configura seu próprio Setup usando o perfil fornecido pela empresa e um nome diferente.
2. Os dois iniciam uma viagem no BCS no mesmo mapa e na sessão compatível da empresa.
3. Confiram o estado **Online**, troquem uma mensagem no chat e combinem um local de encontro.
4. Verifiquem se os dois conseguem ver o ônibus do colega e acompanhar seu movimento. Depois testem sair do próprio ônibus e embarcar como passageiro, se a versão do OpenOMSI utilizada oferecer essa função.

O ingresso de um jogador, o chat e os passageiros compartilhados já foram observados. **O teste com duas pessoas, a visualização entre clientes e o embarque no ônibus de outro jogador ainda precisam de validação real.**

## Para o administrador

Prepare o perfil e hospede a sessão antes de chamar os jogadores. O **CompanyHost** sincroniza o servidor com o relógio configurado da empresa; espere **SINCRONIZADO** e mantenha sua janela aberta.

O endereço **`http://127.0.0.1:27025` funciona apenas no computador do servidor**. Para os colegas, o CompanyHost detecta e confere o HTTPS do túnel atual e cria um perfil pronto com esse endereço. Espere aparecer **PERFIL PARA OS JOGADORES** e envie o arquivo indicado.

O arquivo normalmente fica em **`%LOCALAPPDATA%\OpenOmsi-BBS\Companies\<empresa>.players.json`**. O perfil original do administrador continua sendo usado para hospedar. Um túnel temporário pode mudar ao reiniciar; o CompanyHost atualiza o arquivo dos jogadores, mas você ainda precisa enviar esse arquivo atualizado ou atualizar o link HTTPS onde o disponibiliza. A ponte não publica o perfil na internet.

Os jogadores recebem o perfil pronto. Eles não precisam criar sessões, escolher fusos horários ou editar o `server.cfg`. Os detalhes do administrador estão em [Multiplayer](MULTIPLAYER.md) e [Relógio da empresa](COMPANY_CLOCK.md).
