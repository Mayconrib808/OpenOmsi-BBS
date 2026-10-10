# BBS / PeDePe + openOMSI — modo recomendado na 2.1.0

A partir do suporte **OpenOMSI (Beta)** lançado pela PeDePe em 09/10/2026, a bridge 2.1.0 não precisa mais reconstruir a viagem do BBS no modo recomendado. O próprio BBS envia mapa, ônibus, horário, escala e motorista ao openOMSI e recebe a finalização/avaliação.

## Jogador

1. Extraia o pacote completo `OpenOmsi.+.BBS.2.1.0`.
2. Abra `Setup.exe` e informe:
   - a pasta original do OMSI 2;
   - a pasta do **openOMSI real** que você instalou.
3. Se sua empresa usa o multiplayer desta bridge, mantenha a empresa/perfil e o nome do jogador configurados como na 2.0.5-dev.4.
4. No BBS, abra as configurações do openOMSI e escolha **OpenOMSI (Beta)**.
5. Quando o BBS pedir o caminho do openOMSI, NÃO selecione o openOMSI real. Selecione esta pasta do pacote:

   `PeDePeAdapter`

   Ela contém um `openomsi.exe` intermediário.
6. Inicie a viagem normalmente pelo BBS.
7. Ao terminar, use o botão normal de **Finalizar/Encerrar viagem** do BBS. No modo nativo PeDePe, não é necessário usar o antigo procedimento de F9 da bridge.

### O que o adaptador faz

O `PeDePeAdapter\openomsi.exe` recebe a linha de comando criada pelo BBS e a repassa sem alterar mapa, ônibus, pintura, horário, escala, motorista ou opções futuras.

Se o multiplayer da empresa estiver ativo, ele usa o mesmo perfil da 2.0.5-dev.4 para encontrar/acordar o servidor certo e acrescenta somente:

`--lan-join <servidor> --lan-name <jogador>`

Depois ele abre o **openOMSI real** configurado no Setup.

Se uma versão futura do BBS/openOMSI já enviar `--lan-join`, a bridge não adiciona outro servidor e deixa a implementação nativa assumir.

## Host / administrador da empresa

O Host continua usando a estrutura já existente:

- `HostAgent.exe` continua ativo;
- `CompanyHost.exe` continua sendo usado;
- os perfis JSON de empresa continuam válidos;
- as sessões por mapa continuam válidas;
- Cloudflare/túnel continua válido;
- relógio da empresa continua válido;
- servidor oficial selecionável continua válido;
- o servidor de compatibilidade incluído continua disponível.

Não é necessário recriar a empresa apenas por atualizar da 2.0.5-dev.4 para 2.1.0.

## Modo legado

A ponte antiga por `Omsi.exe` continua incluída como fallback enquanto o suporte oficial da PeDePe estiver em beta. Ela não é o modo recomendado para a 2.1.0.

## Diagnóstico

No modo PeDePe nativo, consulte também:

`app\pedepe-native-v2.1.log`

A opção de coletar logs no Setup inclui esse arquivo quando ele existir.
