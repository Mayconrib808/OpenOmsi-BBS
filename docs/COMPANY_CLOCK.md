# Relógio automático da empresa — 2.0.0-dev.4

A configuração **Mudança de horário** do BCS é relativa ao seu relógio de referência, não ao fuso do computador do jogador. A ponte usa um relógio explícito compartilhado no perfil da empresa. No exemplo observado, o valor configurado foi **-8 horas**.

## Configurar uma vez

1. Extraia o novo pacote em outra pasta e configure o Setup normalmente.
2. Na opção **9 → 1**, no campo da data da sessão, digite **company**. Informe **-8** no campo da mudança de horário do BCS. Cadastre o mapa, o endereço e os pacotes normalmente. Os próximos mapas reutilizam esse relógio.
3. Confira a data/hora calculada contra o relógio da empresa no BCS. O padrão **Europe/Berlin** segue CET/CEST, inclusive horário de verão. Essa escolha corresponde ao exemplo observado e é configurável; a documentação do BCS não confirma claramente o comportamento no inverno. Se a empresa mudar sua diferença no BCS, atualize o perfil compartilhado.
4. Na opção **8**, selecione o perfil da empresa. Deixe **date=auto** em `app/bridge.ini` para usar o calendário da empresa. Uma data manual continua representando uma escolha explícita no calendário do BCS e não é substituída automaticamente.

O perfil usa:

```json
"clock": { "timezone": "Europe/Berlin", "shift_minutes": -480 }
```

Na sessão, use **"date": "company"**. O exemplo completo está em `examples/company-clock.example.json`; seus hashes e links são fictícios e devem ser gerados com os arquivos reais pelo Setup.

Perfis antigos com uma data fixa continuam funcionando. É possível migrar um perfil existente acrescentando `clock` e substituindo a data das sessões desejadas por `company`, preservando o identificador e os pacotes. Não basta mudar o perfil em um ZIP antigo: é necessário atualizar os executáveis da ponte.

## Iniciar o servidor sincronizado

Encerre o servidor que foi iniciado com `start.cmd`. Abra **CompanyHost.exe** no pacote da ponte. Informe o perfil local, a sessão, a pasta do servidor OpenOMSI 0.2.0 e a pasta original do OMSI 2. O auxiliar inicia seu próprio processo de servidor e permanece aberto junto com ele. Após a primeira sincronização confirmada, salva apenas os caminhos e a sessão em `CompanyHost.local.json`, ao lado do executável. Na próxima abertura, Enter reutiliza esses dados; não compartilhe esse arquivo local.

O auxiliar lê seu `server.cfg`, mas gera uma configuração separada para esta execução com mapa, data/hora da empresa, velocidade 1 e somente os ônibus cobertos pelos pacotes da sessão. Não altera o arquivo original, o relógio do Windows ou as configurações da empresa no BCS. Uma credencial temporária de administração fica exclusivamente na configuração local do host e não entra no perfil compartilhado.

Depois do carregamento, o auxiliar confere o servidor local e envia correções de relógio periodicamente pela API local do OpenOMSI. `real_time=0` é obrigatório nesse processo: a opção original `real_time=1` segue o Windows e ignora os ajustes administrativos. A data inicial é calculada automaticamente e o servidor avança o calendário através da meia-noite.

Aguarde a linha **SINCRONIZADO** antes de abrir uma viagem pelo BCS. O endereço HTTPS pode aparecer enquanto o mapa ainda carrega: isso comprova que o gateway abriu, mas a sessão ainda pode não ter mundo ativo. A ponte confere `world` para sessões com relógio da empresa, que usam o servidor dedicado gerenciado pelo CompanyHost.

Use o endereço HTTP local para testar no mesmo computador, ou o endereço HTTPS do túnel mostrado pelo servidor para outros jogadores. Um túnel temporário pode mudar depois de reiniciar: atualize `server_url` no perfil e compartilhe a atualização. A sincronização do relógio não cria um endereço permanente nem hospeda mapas automaticamente.

## O que é sincronizado

- O host mantém o relógio da empresa; jogadores entrando não alteram o mundo para sua própria partida.
- O horário de partida continua sendo lido da viagem do BCS e usado no ajuste do itinerário. Cada jogador pode usar seu próprio ônibus da frota declarada.
- A data automática vem da empresa, sem depender do fuso do jogador.
- Antes de liberar o BCS, a ponte verifica a data e hora reais do mundo que o OpenOMSI confirmou. Se o dia mudar durante o carregamento inicial, a viagem é interrompida com orientação para iniciar novamente, evitando um itinerário preparado para o dia anterior. Viagens já iniciadas continuam acompanhando o calendário compartilhado.

A configuração da diferença é feita uma vez pelo responsável. Não há leitura automática da conta ou das definições privadas da empresa do BCS. A hora UTC depende do relógio do computador do host e dos jogadores estar correto; mantenha a sincronização automática de hora do Windows.

## Recuperação e testes

A API `/status` do OpenOMSI 0.2.0 não fornece a data completa. O auxiliar inicia o servidor com uma data conhecida e acompanha os ajustes. Em uma pausa prolongada ou perda de controle que deixe a data incerta, ele não tenta adivinhar um dia por `HH:MM`; encerra a execução gerenciada e informa a falha. Não reinicia silenciosamente um servidor ocupado.

Os testes automatizados cobrem o exemplo **07/10 00:14 em São Paulo → 06/10 21:14 na empresa**, viradas de dia/mês/ano, transições CET/CEST, independência do fuso do PC, configurações e a API de controle simulada. O teste com OpenOMSI/BCS instalado e o multiplayer com dois jogadores continuam necessários. O suporte continua limitado ao OpenOMSI **0.2.0 / protocolo 6**.

Referências: [manual do BCS](https://busbetrieb-simulator.de/Manual_OMSI2_Busbetrieb_Simulator_en_web.pdf), [servidor OpenOMSI 0.2.0](https://github.com/openOMSI-Project/openOMSI/blob/538ad31b2a2c664cb0726db2547bf6238411eedf/docs/SERVER.md), [relógio nativo](https://github.com/openOMSI-Project/openOMSI/blob/538ad31b2a2c664cb0726db2547bf6238411eedf/crates/omsi-app/src/real_time.rs).

## Atualizar os arquivos de um perfil existente

Se o relatório mostrar **conteúdo alterado**, o arquivo instalado tem bytes diferentes dos registrados no JSON; isso não identifica sozinho uma versão comercial diferente nem quem modificou o arquivo. Scripts e `Holidays.txt` continuam sendo conferidos.

O administrador pode registrar a instalação atual como nova referência sem recadastrar a empresa: com BCS e CompanyHost fechados, use **Setup → 9 → 2**, escolha o JSON local e confira a lista antes de confirmar. O assistente atualiza somente os hashes dos arquivos já declarados e guarda o JSON anterior numa cópia `.backup-<data/hora>`. Preserva nome/identificador, links, sessões, caminhos da frota e relógio. Arquivos ausentes ou extras exigem corrigir a instalação ou atualizar o inventário em separado; não são aceitos automaticamente.

Depois, reinicie o CompanyHost com o mesmo JSON, espere **SINCRONIZADO** e abra o BCS. Compartilhe o JSON atualizado com os jogadores, ou atualize o arquivo publicado no link HTTPS. Se os arquivos mudarem novamente a cada viagem, guarde o novo relatório: atualizar o perfil uma vez não normaliza alterações repetidas ou desconhecidas do BCS.
